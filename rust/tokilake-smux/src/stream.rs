//! SMUX stream implementation.
//!
//! A stream represents a single logical channel within a multiplexed session.
//! Data arrives via a channel from the session's recv loop.
//! Writes go through a channel to the session's write loop.

use crate::session::{Config, Shared, StreamShared, WriteRequest};
use bytes::{Buf, Bytes};
use std::sync::Arc;
use tokio::sync::mpsc;

/// A multiplexed stream within a session.
///
/// Implements `AsyncRead` for reading data delivered by the session's recv loop.
/// Writes are sent as PSH frames through the session's write loop.
pub struct Stream {
    /// Stream identifier.
    id:             u32,
    /// Receiver for incoming data (from recv loop).
    data_rx:        mpsc::Receiver<Bytes>,
    /// Sender for control requests (high priority).
    ctrl_tx:        mpsc::Sender<WriteRequest>,
    /// Sender for data requests (low priority).
    data_tx:        mpsc::Sender<WriteRequest>,
    /// Session shared state (for global bucket).
    session_shared: Arc<Shared>,
    /// Stream shared state (for window updates).
    stream_shared:  Arc<StreamShared>,
    /// Session config.
    config:         Config,
    /// Partially consumed read buffer.
    read_buf:       Bytes,
    /// Whether FIN has been received (remote closed).
    fin_received:   bool,
    /// Whether FIN has been sent (local closed).
    fin_sent:       bool,

    // V2 flow control counters
    num_read:                u32,
    num_written:             u32,
    incr:                    u32,
    window_update_threshold: u32,
}

impl Stream {
    /// Create a new stream.
    pub(crate) fn new(
        id: u32,
        data_rx: mpsc::Receiver<Bytes>,
        ctrl_tx: mpsc::Sender<WriteRequest>,
        data_tx: mpsc::Sender<WriteRequest>,
        session_shared: Arc<Shared>,
        stream_shared: Arc<StreamShared>,
        config: Config,
    ) -> Self {
        let window_update_threshold = (config.max_stream_buffer / 2) as u32;
        Self {
            id,
            data_rx,
            ctrl_tx,
            data_tx,
            session_shared,
            stream_shared,
            config,
            read_buf: Bytes::new(),
            fin_received: false,
            fin_sent: false,
            num_read: 0,
            num_written: 0,
            incr: 0,
            window_update_threshold,
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
                self.consume_tokens(n);
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

        if data.is_empty() {
            return Ok(0);
        }

        if self.config.version == 2 {
            loop {
                // Check if session is closed
                if self
                    .session_shared
                    .is_closed
                    .load(std::sync::atomic::Ordering::Acquire)
                {
                    return Err(std::io::Error::new(
                        std::io::ErrorKind::BrokenPipe,
                        "session closed",
                    ));
                }

                let peer_consumed = self
                    .stream_shared
                    .peer_consumed
                    .load(std::sync::atomic::Ordering::Acquire);
                let peer_window = self
                    .stream_shared
                    .peer_window
                    .load(std::sync::atomic::Ordering::Acquire);

                let inflight = self.num_written.wrapping_sub(peer_consumed) as i32;
                if inflight < 0 {
                    return Err(std::io::Error::new(
                        std::io::ErrorKind::InvalidData,
                        "peer consumed more than sent",
                    ));
                }

                let win = (peer_window as i32) - inflight;

                if win > 0 {
                    let to_write = std::cmp::min(
                        data.len(),
                        std::cmp::min(crate::frame::MAX_PAYLOAD_SIZE, win as usize),
                    );

                    self.data_tx
                        .send(WriteRequest::Data {
                            stream_id: self.id,
                            data:      Bytes::copy_from_slice(&data[..to_write]),
                        })
                        .await
                        .map_err(|_| {
                            std::io::Error::new(std::io::ErrorKind::BrokenPipe, "session closed")
                        })?;

                    self.num_written = self.num_written.wrapping_add(to_write as u32);
                    return Ok(to_write);
                } else {
                    // Wait for window update
                    tokio::select! {
                        _ = self.stream_shared.window_notify.notified() => continue,
                        _ = self.data_tx.closed() => {
                            return Err(std::io::Error::new(
                                std::io::ErrorKind::BrokenPipe,
                                "session closed",
                            ));
                        }
                    }
                }
            }
        } else {
            // V1: No flow control, just chunk and send
            let to_write = std::cmp::min(data.len(), crate::frame::MAX_PAYLOAD_SIZE);

            self.data_tx
                .send(WriteRequest::Data {
                    stream_id: self.id,
                    data:      Bytes::copy_from_slice(&data[..to_write]),
                })
                .await
                .map_err(|_| {
                    std::io::Error::new(std::io::ErrorKind::BrokenPipe, "session closed")
                })?;

            Ok(to_write)
        }
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
                .ctrl_tx
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

    /// Consume tokens (V2) when data is read by the application.
    fn consume_tokens(&mut self, n: usize) {
        if self.config.version != 2 || n == 0 {
            return;
        }

        // Return tokens to global bucket
        if self
            .session_shared
            .bucket
            .fetch_add(n as i32, std::sync::atomic::Ordering::Release)
            <= 0
        {
            self.session_shared.bucket_notify.notify_one();
        }

        // Update local read counters
        self.num_read = self.num_read.wrapping_add(n as u32);
        self.incr = self.incr.wrapping_add(n as u32);

        // Send window update if needed
        if self.incr >= self.window_update_threshold || self.num_read == n as u32 {
            let req = WriteRequest::Upd {
                stream_id: self.id,
                consumed:  self.num_read,
                window:    self.config.max_stream_buffer as u32,
            };
            self.incr = 0;

            if let Err(_) = self.ctrl_tx.try_send(req) {
                // If the channel is full, spawn a task to ensure the window update is delivered
                let tx = self.ctrl_tx.clone();
                let stream_id = self.id;
                let consumed = self.num_read;
                let window = self.config.max_stream_buffer as u32;
                tokio::spawn(async move {
                    let _ = tx
                        .send(WriteRequest::Upd {
                            stream_id,
                            consumed,
                            window,
                        })
                        .await;
                });
            }
        }
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
            self.consume_tokens(n);
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
                self.consume_tokens(n);
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
