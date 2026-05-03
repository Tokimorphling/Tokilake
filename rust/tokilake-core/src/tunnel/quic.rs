use super::{TunnelError, TunnelSession, TunnelStream};
use std::future::Future;

/// A QUIC tunnel session wrapping `quinn::Connection`.
pub struct QuicSession {
    conn: quinn::Connection,
}

impl QuicSession {
    pub fn new(conn: quinn::Connection) -> Self {
        Self { conn }
    }

    /// Access the underlying `quinn::Connection` for direct stream operations.
    pub fn connection(&self) -> &quinn::Connection {
        &self.conn
    }
}

/// A QUIC stream wrapping `quinn::SendStream` and `quinn::RecvStream`.
pub struct QuicStream {
    send: quinn::SendStream,
    recv: quinn::RecvStream,
}

impl QuicStream {
    pub fn new(send: quinn::SendStream, recv: quinn::RecvStream) -> Self {
        Self { send, recv }
    }
}

impl TunnelSession for QuicSession {
    type Stream = QuicStream;

    async fn accept_stream(&mut self) -> Result<Option<Self::Stream>, TunnelError> {
        match self.conn.accept_bi().await {
            Ok((send, recv)) => Ok(Some(QuicStream::new(send, recv))),
            Err(e) => {
                tracing::warn!("QUIC accept error: {}", e);
                Ok(None)
            }
        }
    }

    async fn open_stream(&mut self) -> Result<Self::Stream, TunnelError> {
        match self.conn.open_bi().await {
            Ok((send, recv)) => Ok(QuicStream::new(send, recv)),
            Err(e) => {
                tracing::warn!("QUIC open error: {}", e);
                Err(TunnelError::StreamClosed)
            }
        }
    }

    async fn close(&self) -> Result<(), TunnelError> {
        self.conn.close(0u32.into(), b"session closed");
        Ok(())
    }

    fn is_alive(&self) -> bool {
        self.conn.close_reason().is_none()
    }
}

#[allow(clippy::manual_async_fn)]
impl TunnelStream for QuicStream {
    fn read<'a>(
        &'a mut self,
        buf: &'a mut [u8],
    ) -> impl Future<Output = Result<usize, TunnelError>> + Send + 'a {
        use crate::error::MapTunnelErr;
        async move {
            self.recv
                .read(buf)
                .await
                .map(|opt| opt.unwrap_or(0))
                .map_stream_closed()
        }
    }

    fn write<'a>(
        &'a mut self,
        buf: &'a [u8],
    ) -> impl Future<Output = Result<usize, TunnelError>> + Send + 'a {
        use crate::error::MapTunnelErr;
        async move {
            self.send
                .write_all(buf)
                .await
                .map(|_| buf.len())
                .map_stream_closed()
        }
    }

    fn flush(&mut self) -> impl Future<Output = Result<(), TunnelError>> + Send + '_ {
        // QUIC streams are not buffered at the application level;
        // quinn handles flushing internally. No-op here.
        async move { Ok(()) }
    }

    fn close(&mut self) -> impl Future<Output = Result<(), TunnelError>> + Send + '_ {
        use crate::error::MapTunnelErr;
        async move { self.send.finish().map_stream_closed() }
    }
}
