package tokilake

import (
	"io"
	"sync"

	"github.com/gorilla/websocket"
)

type websocketStreamConn struct {
	conn          *websocket.Conn
	currentReader io.Reader
	writeMu       sync.Mutex
}

func NewWebsocketStreamConn(conn *websocket.Conn) *websocketStreamConn {
	return &websocketStreamConn{conn: conn}
}

func (c *websocketStreamConn) Read(p []byte) (int, error) {
	for {
		if c.currentReader == nil {
			messageType, reader, err := c.conn.NextReader()
			if err != nil {
				return 0, err
			}
			if messageType != websocket.BinaryMessage {
				continue
			}
			c.currentReader = reader
		}

		n, err := c.currentReader.Read(p)
		if err == io.EOF {
			c.currentReader = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (c *websocketStreamConn) Write(p []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	writer, err := c.conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return 0, err
	}

	n, writeErr := writer.Write(p)
	closeErr := writer.Close()
	if writeErr != nil {
		return n, writeErr
	}
	if closeErr != nil {
		return n, closeErr
	}
	return n, nil
}

func (c *websocketStreamConn) Close() error {
	return c.conn.Close()
}
