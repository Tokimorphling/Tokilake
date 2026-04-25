package tokilake

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/xtaci/smux"
)

const (
	TunnelTransportWebSocket = "websocket"
	TunnelTransportQUIC      = "quic"
)

type TunnelSession interface {
	AcceptStream(context.Context) (TunnelStream, error)
	OpenStream(context.Context) (TunnelStream, error)
	Close() error
}

type TunnelStream interface {
	io.ReadWriteCloser
	SetReadDeadline(time.Time) error
}

type smuxTunnelSession struct {
	session *smux.Session
}

type smuxTunnelStream struct {
	stream *smux.Stream
}

type quicTunnelSession struct {
	conn *quic.Conn
}

type quicTunnelStream struct {
	stream *quic.Stream
}

var _ TunnelSession = (*smuxTunnelSession)(nil)
var _ TunnelSession = (*quicTunnelSession)(nil)
var _ TunnelStream = (*smuxTunnelStream)(nil)
var _ TunnelStream = (*quicTunnelStream)(nil)

func NewSMuxTunnelSession(session *smux.Session) TunnelSession {
	return &smuxTunnelSession{session: session}
}


func (s *smuxTunnelSession) AcceptStream(ctx context.Context) (TunnelStream, error) {
	if s == nil || s.session == nil {
		return nil, io.ErrClosedPipe
	}
	stream, err := acceptSMuxStreamWithContext(ctx, s.session)
	if err != nil {
		return nil, err
	}
	return &smuxTunnelStream{stream: stream}, nil
}

func (s *smuxTunnelSession) OpenStream(_ context.Context) (TunnelStream, error) {
	if s == nil || s.session == nil {
		return nil, io.ErrClosedPipe
	}
	stream, err := s.session.OpenStream()
	if err != nil {
		return nil, err
	}
	return &smuxTunnelStream{stream: stream}, nil
}

func (s *smuxTunnelSession) Close() error {
	if s == nil || s.session == nil {
		return nil
	}
	err := s.session.Close()
	if errors.Is(err, io.ErrClosedPipe) {
		return nil
	}
	return err
}

func (s *smuxTunnelStream) Read(p []byte) (int, error) {
	return s.stream.Read(p)
}

func (s *smuxTunnelStream) Write(p []byte) (int, error) {
	return s.stream.Write(p)
}

func (s *smuxTunnelStream) Close() error {
	return s.stream.Close()
}

func (s *smuxTunnelStream) SetReadDeadline(t time.Time) error {
	return s.stream.SetReadDeadline(t)
}

func NewQUICTunnelSession(conn *quic.Conn) TunnelSession {
	return &quicTunnelSession{conn: conn}
}

func (s *quicTunnelSession) AcceptStream(ctx context.Context) (TunnelStream, error) {
	if s == nil || s.conn == nil {
		return nil, io.ErrClosedPipe
	}
	if ctx == nil {
		ctx = context.Background()
	}
	stream, err := s.conn.AcceptStream(ctx)
	if err != nil {
		return nil, err
	}
	return &quicTunnelStream{stream: stream}, nil
}

func (s *quicTunnelSession) OpenStream(ctx context.Context) (TunnelStream, error) {
	if s == nil || s.conn == nil {
		return nil, io.ErrClosedPipe
	}
	if ctx == nil {
		ctx = context.Background()
	}
	stream, err := s.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return &quicTunnelStream{stream: stream}, nil
}

func (s *quicTunnelSession) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.CloseWithError(0, "")
}

func (s *quicTunnelStream) Read(p []byte) (int, error) {
	return s.stream.Read(p)
}

func (s *quicTunnelStream) Write(p []byte) (int, error) {
	return s.stream.Write(p)
}

func (s *quicTunnelStream) Close() error {
	return s.stream.Close()
}

func (s *quicTunnelStream) SetReadDeadline(t time.Time) error {
	return s.stream.SetReadDeadline(t)
}

func acceptSMuxStreamWithContext(ctx context.Context, session *smux.Session) (*smux.Stream, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = session.SetDeadline(deadline)
		defer session.SetDeadline(time.Time{})

		stream, err := session.AcceptStream()
		if err != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return stream, err
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = session.SetDeadline(time.Now())
		case <-done:
		}
	}()
	defer func() {
		close(done)
		_ = session.SetDeadline(time.Time{})
	}()

	stream, err := session.AcceptStream()
	if err != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return stream, err
}
