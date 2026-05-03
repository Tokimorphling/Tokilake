//! SMUX stream implementation.
//!
//! A stream represents a single logical channel within a multiplexed session.
//! Data arrives via a channel from the session's recv loop.
//! Writes go through a channel to the session's write loop.

use crate::session::WriteRequest;
use bytes::{Buf, Bytes};
use tokio::sync::mpsc;

/// A multiplexed stream within a session.
///
/// Implements `AsyncRead` for reading data delivered by the session's recv loop.
/// Writes are sent as PSH frames through the session's write loop.
pub struct Stream {
    /// Stream identifier.
    id:           u32,
    /// Receiver for incoming data (from recv loop).
    data_rx:      mpsc::Receiver<Bytes>,
    /// Sender for write requests (to write loop).
    write_tx:     mpsc::Sender<WriteRequest>,
    /// Partially consumed read buffer.
    read_buf:     Bytes,
    /// Whether FIN has been received (remote closed).
    fin_received: bool,
    /// Whether FIN has been sent (local closed).
    fin_sent:     bool,
}

impl Stream {
    /// Create a new stream.
    pub(crate) fn new(
        id: u32,
        data_rx: mpsc::Receiver<Bytes>,
        write_tx: mpsc::Sender<WriteRequest>,
    ) -> Self {
        Self {
            id,
            data_rx,
            write_tx,
            read_buf: Bytes::new(),
            fin_received: false,
            fin_sent: false,
        }
    }

    /// Returns the stream identifier.
    pub fn id(&self) -> u32 {
        self.id
    }

    /// Read data from the stream.
    ///
    /// Returns `Ok(0)` on EOF (FIN received and buffer drained).
    pub async fn read(&mut self, buf: &mut [u8]) -> Result<usize, std::io::Error> {
        loop {
            // Try to serve from buffer
            if !self.read_buf.is_empty() {
                let n = std::cmp::min(buf.len(), self.read_buf.len());
                buf[..n].copy_from_slice(&self.read_buf[..n]);
                self.read_buf.advance(n);
                return Ok(n);
            }

            // EOF?
            if self.fin_received {
                return Ok(0);
            }

            // Wait for more data
            match self.data_rx.recv().await {
                Some(data) => {
                    self.read_buf = data;
                }
                None => {
                    // Channel closed = session dropped = EOF
                    self.fin_received = true;
                    return Ok(0);
                }
            }
        }
    }

    /// Write data to the stream (sends PSH frame).
    pub async fn write(&mut self, data: &[u8]) -> Result<usize, std::io::Error> {
        if self.fin_sent {
            return Err(std::io::Error::new(
                std::io::ErrorKind::BrokenPipe,
                "stream write closed",
            ));
        }

        self.write_tx
            .send(WriteRequest::Data {
                stream_id: self.id,
                data:      Bytes::copy_from_slice(data),
            })
            .await
            .map_err(|_| std::io::Error::new(std::io::ErrorKind::BrokenPipe, "session closed"))?;

        Ok(data.len())
    }

    /// Write all data to the stream.
    pub async fn write_all(&mut self, data: &[u8]) -> Result<(), std::io::Error> {
        let mut written = 0;
        while written < data.len() {
            written += self.write(&data[written..]).await?;
        }
        Ok(())
    }

    /// Close the stream (sends FIN frame).
    pub async fn close(&mut self) -> Result<(), std::io::Error> {
        if !self.fin_sent {
            self.fin_sent = true;
            let _ = self
                .write_tx
                .send(WriteRequest::Fin { stream_id: self.id })
                .await;
        }
        Ok(())
    }

    /// Check if FIN has been received from the remote.
    pub fn is_fin_received(&self) -> bool {
        self.fin_received
    }

    /// Check if there's data in the read buffer.
    pub fn has_buffered_data(&self) -> bool {
        !self.read_buf.is_empty()
    }
}

/// Implement `AsyncRead` so `Stream` can be used with tokio I/O utilities.
impl tokio::io::AsyncRead for Stream {
    fn poll_read(
        mut self: std::pin::Pin<&mut Self>,
        cx: &mut std::task::Context<'_>,
        buf: &mut tokio::io::ReadBuf<'_>,
    ) -> std::task::Poll<std::io::Result<()>> {
        // Serve from buffer
        if !self.read_buf.is_empty() {
            let n = std::cmp::min(buf.remaining(), self.read_buf.len());
            buf.put_slice(&self.read_buf[..n]);
            self.read_buf.advance(n);
            return std::task::Poll::Ready(Ok(()));
        }

        if self.fin_received {
            return std::task::Poll::Ready(Ok(()));
        }

        // Poll the channel
        match self.data_rx.poll_recv(cx) {
            std::task::Poll::Ready(Some(data)) => {
                let n = std::cmp::min(buf.remaining(), data.len());
                buf.put_slice(&data[..n]);
                if n < data.len() {
                    self.read_buf = data.slice(n..);
                }
                std::task::Poll::Ready(Ok(()))
            }
            std::task::Poll::Ready(None) => {
                self.fin_received = true;
                std::task::Poll::Ready(Ok(()))
            }
            std::task::Poll::Pending => std::task::Poll::Pending,
        }
    }
}
