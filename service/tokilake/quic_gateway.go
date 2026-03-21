package tokilake

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"

	"one-api/common/logger"

	"github.com/quic-go/quic-go"
	"github.com/spf13/viper"
)

func StartConfiguredQUICGateway(ctx context.Context) (func() error, error) {
	if !viper.GetBool("quic.enable") {
		return nil, nil
	}

	port := strings.TrimSpace(viper.GetString("quic.port"))
	if port == "" {
		port = strings.TrimSpace(viper.GetString("port"))
	}
	if port == "" {
		return nil, errors.New("quic.port or port is required")
	}

	certFile := strings.TrimSpace(viper.GetString("quic.cert_file"))
	keyFile := strings.TrimSpace(viper.GetString("quic.key_file"))
	listener, err := newQUICGatewayListener(":"+port, certFile, keyFile)
	if err != nil {
		return nil, err
	}

	go serveQUICGatewayListener(ctx, listener)
	logger.SysLog(fmt.Sprintf("tokilake quic gateway listening addr=%s", listener.Addr().String()))

	return listener.Close, nil
}

func newQUICGatewayListener(addr string, certFile string, keyFile string) (*quic.Listener, error) {
	tlsConfig, err := loadQUICServerTLSConfig(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	listener, err := quic.ListenAddr(addr, tlsConfig, &quic.Config{
		KeepAlivePeriod: defaultHeartbeatInterval,
	})
	if err != nil {
		return nil, fmt.Errorf("listen quic gateway: %w", err)
	}
	return listener, nil
}

func loadQUICServerTLSConfig(certFile string, keyFile string) (*tls.Config, error) {
	if certFile == "" {
		return nil, errors.New("quic.cert_file is required when quic.enable=true")
	}
	if keyFile == "" {
		return nil, errors.New("quic.key_file is required when quic.enable=true")
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

func serveQUICGatewayListener(ctx context.Context, listener *quic.Listener) {
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
				logger.SysError(fmt.Sprintf("tokilake quic accept failed: err=%v", err))
				continue
			}
		}

		go serveQUICGatewayConn(ctx, conn)
	}
}

func serveQUICGatewayConn(ctx context.Context, conn *quic.Conn) {
	if conn == nil {
		return
	}

	manager := GetSessionManager()
	session := manager.NewGatewaySession(nil, "", conn.RemoteAddr().String(), TunnelTransportQUIC, newQUICTunnelSession(conn))
	defer func() {
		if cleanupErr := cleanupGatewaySession(session); cleanupErr != nil {
			logger.SysError(fmt.Sprintf("tokilake cleanup failed: session_id=%d transport=%s err=%v", session.ID, session.Transport, cleanupErr))
		}
	}()

	if err := serveGatewaySession(ctx, manager, session); err != nil {
		logger.SysLog(fmt.Sprintf("tokilake gateway session closed: transport=%s remote=%s err=%v", session.Transport, session.RemoteAddr, err))
	}
}
