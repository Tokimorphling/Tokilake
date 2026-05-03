use crate::error::TunnelError;
use std::future::Future;

pub mod quic;
pub mod smux;

/// Tunnel session trait - multiplexed stream container.
pub trait TunnelSession: Send + Sync + 'static {
    type Stream: TunnelStream;

    fn accept_stream(
        &mut self,
    ) -> impl Future<Output = Result<Option<Self::Stream>, TunnelError>> + Send + '_;
    fn open_stream(
        &mut self,
    ) -> impl Future<Output = Result<Self::Stream, TunnelError>> + Send + '_;
    fn close(&self) -> impl Future<Output = Result<(), TunnelError>> + Send + '_;
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
    fn flush(&mut self) -> impl Future<Output = Result<(), TunnelError>> + Send + '_;
    fn close(&mut self) -> impl Future<Output = Result<(), TunnelError>> + Send + '_;
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
        let (tx1, rx1) = mpsc::channel(100);
        let (tx2, rx2) = mpsc::channel(100);
        (MemoryStream::new(rx1, tx2), MemoryStream::new(rx2, tx1))
    }

    #[allow(clippy::manual_async_fn)]
    impl TunnelStream for MemoryStream {
        fn read<'a>(
            &'a mut self,
            buf: &'a mut [u8],
        ) -> impl Future<Output = Result<usize, TunnelError>> + Send + 'a {
            async move {
                if self.buffer.is_empty() {
                    self.buffer = self.rx.recv().await.ok_or(TunnelError::StreamClosed)?;
                }
                let n = std::cmp::min(buf.len(), self.buffer.len());
                buf[..n].copy_from_slice(&self.buffer[..n]);
                self.buffer.drain(..n);
                Ok(n)
            }
        }

        fn write<'a>(
            &'a mut self,
            buf: &'a [u8],
        ) -> impl Future<Output = Result<usize, TunnelError>> + Send + 'a {
            async move {
                self.tx
                    .send(buf.to_vec())
                    .await
                    .map_err(|_| TunnelError::StreamClosed)?;
                Ok(buf.len())
            }
        }

        fn flush(&mut self) -> impl Future<Output = Result<(), TunnelError>> + Send + '_ {
            async move { Ok(()) }
        }

        fn close(&mut self) -> impl Future<Output = Result<(), TunnelError>> + Send + '_ {
            async move { Ok(()) }
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
        let _ = a.write(b"ping").await;
        let _ = b.write(b"pong").await;
        let mut buf = vec![0u8; 64];
        let n = b.read(&mut buf).await.unwrap();
        assert_eq!(&buf[..n], b"ping");
        let n = a.read(&mut buf).await.unwrap();
        assert_eq!(&buf[..n], b"pong");
    }
}
