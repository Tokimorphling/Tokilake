package tokilake

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"

	"github.com/quic-go/quic-go"
)

type QUICGatewayConfig struct {
	Enable   bool
	Port     string
	CertFile string
	KeyFile  string
}

func (g *Gateway) StartQUICGateway(ctx context.Context, config QUICGatewayConfig) (func() error, error) {
	if !config.Enable {
		return nil, nil
	}

	if config.Port == "" {
		return nil, errors.New("quic port is required")
	}

	listener, err := g.newQUICGatewayListener(":"+config.Port, config.CertFile, config.KeyFile)
	if err != nil {
		return nil, err
	}

	go g.serveQUICGatewayListener(ctx, listener)
	g.Logger.SysLog(fmt.Sprintf("tokilake quic gateway listening addr=%s", listener.Addr().String()))

	return listener.Close, nil
}

func (g *Gateway) newQUICGatewayListener(addr string, certFile string, keyFile string) (*quic.Listener, error) {
	tlsConfig, err := g.loadQUICServerTLSConfig(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	listener, err := quic.ListenAddr(addr, tlsConfig, &quic.Config{
		KeepAlivePeriod: heartbeatTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("listen quic gateway: %w", err)
	}
	return listener, nil
}

func (g *Gateway) loadQUICServerTLSConfig(certFile string, keyFile string) (*tls.Config, error) {
	if certFile == "" {
		return nil, errors.New("quic cert_file is required when quic is enabled")
	}
	if keyFile == "" {
		return nil, errors.New("quic key_file is required when quic is enabled")
	}

	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load quic certificate: %w", err)
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{"tokilake.v1"},
		Certificates: []tls.Certificate{certificate},
	}, nil
}

func (g *Gateway) serveQUICGatewayListener(ctx context.Context, listener *quic.Listener) {
	if listener == nil {
		return
	}

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept(ctx)
		if err != nil {
			switch {
			case ctx.Err() != nil:
				return
			case errors.Is(err, context.Canceled):
				return
			case errors.Is(err, quic.ErrServerClosed):
				return
			default:
				g.Logger.SysError(fmt.Sprintf("tokilake quic accept failed: err=%v", err))
				continue
			}
		}

		go g.serveQUICGatewayConn(ctx, conn)
	}
}

func (g *Gateway) serveQUICGatewayConn(ctx context.Context, conn *quic.Conn) {
	if conn == nil {
		return
	}

	session := g.Manager.NewGatewaySession(nil, "", conn.RemoteAddr().String(), TunnelTransportQUIC, NewQUICTunnelSession(conn))
	defer func() {
		if cleanupErr := g.Registry.CleanupWorker(ctx, session); cleanupErr != nil {
			g.Logger.SysError(fmt.Sprintf("tokilake cleanup failed: session_id=%d transport=%s err=%v", session.ID, session.Transport, cleanupErr))
		}
		g.Manager.Release(session)
		session.Close()
	}()

	if err := g.serveGatewaySession(ctx, session); err != nil {
		g.Logger.SysLog(fmt.Sprintf("tokilake gateway session closed: transport=%s remote=%s err=%v", session.Transport, session.RemoteAddr, err))
	}
}
