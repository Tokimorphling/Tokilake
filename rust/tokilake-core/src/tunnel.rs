use crate::error::TunnelError;
use std::future::Future;

/// Tunnel session trait - multiplexed stream container.
///
/// Uses `impl Future` returns instead of `async_trait` for zero overhead.
pub trait TunnelSession: Send + Sync + 'static {
    type Stream: TunnelStream;
    type AcceptFuture<'a>: Future<Output = Result<Option<Self::Stream>, TunnelError>> + Send
    where
        Self: 'a;
    type OpenFuture<'a>: Future<Output = Result<Self::Stream, TunnelError>> + Send
    where
        Self: 'a;

    fn accept_stream(&self) -> Self::AcceptFuture<'_>;
    fn open_stream(&self) -> Self::OpenFuture<'_>;
    fn close(&self) -> impl Future<Output = Result<(), TunnelError>> + Send;
    fn is_alive(&self) -> bool;
}

/// Tunnel stream trait - bidirectional byte stream.
pub trait TunnelStream: Send + Sync + 'static {
    fn read<'a>(
        &'a mut self,
        buf: &'a mut [u8],
    ) -> impl Future<Output = Result<usize, TunnelError>> + Send + 'a;
    fn write<'a>(
        &'a mut self,
        buf: &'a [u8],
    ) -> impl Future<Output = Result<usize, TunnelError>> + Send + 'a;
    fn flush(&mut self) -> impl Future<Output = Result<(), TunnelError>> + Send;
    fn close(&mut self) -> impl Future<Output = Result<(), TunnelError>> + Send;
}

/// In-memory stream for testing.
#[cfg(test)]
pub mod memory {
    use super::*;
    use tokio::sync::mpsc;

    pub struct MemoryStream {
        rx:     mpsc::Receiver<Vec<u8>>,
        tx:     mpsc::Sender<Vec<u8>>,
        buffer: Vec<u8>,
    }

    impl MemoryStream {
        pub fn new(rx: mpsc::Receiver<Vec<u8>>, tx: mpsc::Sender<Vec<u8>>) -> Self {
            Self {
                rx,
                tx,
                buffer: Vec::new(),
            }
        }
    }

    pub fn create_stream_pair() -> (MemoryStream, MemoryStream) {
        let (tx1, rx1) = mpsc::channel(32);
        let (tx2, rx2) = mpsc::channel(32);
        (MemoryStream::new(rx1, tx2), MemoryStream::new(rx2, tx1))
    }

    impl TunnelStream for MemoryStream {
        async fn read<'a>(&'a mut self, buf: &'a mut [u8]) -> Result<usize, TunnelError> {
            if self.buffer.is_empty() {
                match self.rx.recv().await {
                    Some(data) => self.buffer = data,
                    None => return Ok(0),
                }
            }
            let n = std::cmp::min(buf.len(), self.buffer.len());
            buf[..n].copy_from_slice(&self.buffer[..n]);
            self.buffer.drain(..n);
            Ok(n)
        }

        async fn write<'a>(&'a mut self, buf: &'a [u8]) -> Result<usize, TunnelError> {
            self.tx
                .send(buf.to_vec())
                .await
                .map_err(|_| TunnelError::StreamClosed)?;
            Ok(buf.len())
        }

        async fn flush(&mut self) -> Result<(), TunnelError> {
            Ok(())
        }

        async fn close(&mut self) -> Result<(), TunnelError> {
            Ok(())
        }
    }
}

#[cfg(test)]
mod tests {
    use super::{memory::create_stream_pair, *};

    #[tokio::test]
    async fn test_memory_stream_roundtrip() {
        let (mut a, mut b) = create_stream_pair();
        let data = b"hello, tunnel!";
        a.write(data).await.unwrap();
        let mut buf = vec![0u8; 64];
        let n = b.read(&mut buf).await.unwrap();
        assert_eq!(&buf[..n], data);
    }

    #[tokio::test]
    async fn test_memory_stream_bidirectional() {
        let (mut a, mut b) = create_stream_pair();
        a.write(b"ping").await.unwrap();
        b.write(b"pong").await.unwrap();
        let mut buf = vec![0u8; 64];
        let n = b.read(&mut buf).await.unwrap();
        assert_eq!(&buf[..n], b"ping");
        let n = a.read(&mut buf).await.unwrap();
        assert_eq!(&buf[..n], b"pong");
    }
}
