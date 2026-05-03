//! SMUX session implementation.
//!
//! Wire-compatible with Go smux (github.com/xtaci/smux).
//!
//! ## Design (monolake-style zero-overhead)
//!
//! - Generic over transport via split reader/writer — no `Box<dyn>`
//! - `impl Future` returns — no `async_trait`
//! - `Arc<Mutex<>>` for shared stream registry between tasks
//! - Channel-based I/O for concurrent read/write

use crate::{
    frame::{Frame, CMD_FIN, CMD_NOP, CMD_PSH, CMD_SYN, HEADER_SIZE},
    stream::Stream,
};
use bytes::Bytes;
use std::{collections::HashMap, sync::Arc, time::Duration};
use tokio::{
    io::{AsyncRead, AsyncReadExt, AsyncWrite, AsyncWriteExt},
    sync::{mpsc, Mutex},
};

/// Default accept backlog.
const DEFAULT_ACCEPT_BACKLOG: usize = 1024;

/// SMUX session configuration.
#[derive(Debug, Clone)]
pub struct Config {
    /// Protocol version (1 or 2).
    pub version:             u8,
    /// Disable keepalive.
    pub keep_alive_disabled: bool,
    /// Keepalive interval.
    pub keep_alive_interval: Duration,
    /// Keepalive timeout.
    pub keep_alive_timeout:  Duration,
    /// Maximum frame payload size.
    pub max_frame_size:      usize,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            version:             1,
            keep_alive_disabled: true,
            keep_alive_interval: Duration::from_secs(10),
            keep_alive_timeout:  Duration::from_secs(30),
            max_frame_size:      32768,
        }
    }
}

/// Write request from a stream to the write loop.
pub(crate) enum WriteRequest {
    /// Send SYN frame (stream open).
    Syn { stream_id: u32 },
    /// Send data frame.
    Data { stream_id: u32, data: Bytes },
    /// Send FIN frame (stream close).
    Fin { stream_id: u32 },
}

/// Shared state between session and spawned tasks.
struct Shared {
    /// Stream data senders, keyed by stream_id.
    streams:   Mutex<HashMap<u32, mpsc::Sender<Bytes>>>,
    /// Session closed flag.
    is_closed: std::sync::atomic::AtomicBool,
}

/// A multiplexed session over an underlying transport.
pub struct Session {
    /// Accept incoming streams.
    accept_rx:      mpsc::Receiver<Stream>,
    /// Write request sender (shared with streams).
    write_tx:       mpsc::Sender<WriteRequest>,
    /// Shared state.
    shared:         Arc<Shared>,
    /// Next stream ID for open().
    next_stream_id: u32,
    /// Config version.
    version:        u8,
}

impl Session {
    /// Create a server-side session.
    pub fn server<IO>(transport: IO, config: Config) -> Self
    where
        IO: AsyncRead + AsyncWrite + Send + Unpin + 'static,
    {
        let (r, w) = tokio::io::split(transport);
        Self::new(r, w, config, false)
    }

    /// Create a client-side session.
    pub fn client<IO>(transport: IO, config: Config) -> Self
    where
        IO: AsyncRead + AsyncWrite + Send + Unpin + 'static,
    {
        let (r, w) = tokio::io::split(transport);
        Self::new(r, w, config, true)
    }

    /// Create a session from separate reader/writer.
    pub fn from_split<R, W>(reader: R, writer: W, config: Config, is_client: bool) -> Self
    where
        R: AsyncRead + Send + Unpin + 'static,
        W: AsyncWrite + Send + Unpin + 'static,
    {
        Self::new(reader, writer, config, is_client)
    }

    fn new<R, W>(reader: R, writer: W, config: Config, is_client: bool) -> Self
    where
        R: AsyncRead + Send + Unpin + 'static,
        W: AsyncWrite + Send + Unpin + 'static,
    {
        let (accept_tx, accept_rx) = mpsc::channel(DEFAULT_ACCEPT_BACKLOG);
        let (write_tx, write_rx) = mpsc::channel(1024);

        let shared = Arc::new(Shared {
            streams:   Mutex::new(HashMap::new()),
            is_closed: std::sync::atomic::AtomicBool::new(false),
        });

        let version = config.version;

        // Spawn recv loop
        let shared_recv = shared.clone();
        let write_tx_recv = write_tx.clone();
        tokio::spawn(recv_loop(
            reader,
            accept_tx,
            write_tx_recv,
            shared_recv,
            version,
        ));

        // Spawn write loop
        let shared_write = shared.clone();
        tokio::spawn(write_loop(writer, write_rx, shared_write));

        let next_stream_id = if is_client { 1 } else { 0 };

        Self {
            accept_rx,
            write_tx,
            shared,
            next_stream_id,
            version,
        }
    }

    /// Accept an incoming stream from the remote peer.
    pub async fn accept(&mut self) -> Option<Stream> {
        self.accept_rx.recv().await
    }

    /// Open a new stream to the remote peer.
    pub async fn open(&mut self) -> Option<Stream> {
        if self.is_closed() {
            return None;
        }

        if self.next_stream_id >= u32::MAX - 2 {
            return None;
        }

        let stream_id = self.next_stream_id;
        self.next_stream_id += 2;

        // Register stream data channel
        let (data_tx, data_rx) = mpsc::channel(64);
        {
            let mut streams = self.shared.streams.lock().await;
            streams.insert(stream_id, data_tx);
        }

        // Send SYN
        tracing::debug!("open: sending SYN for stream {stream_id}");
        let _ = self.write_tx.send(WriteRequest::Syn { stream_id }).await;
        tracing::debug!("open: SYN sent for stream {stream_id}");

        Some(Stream::new(stream_id, data_rx, self.write_tx.clone()))
    }

    /// Check if the session is closed.
    pub fn is_closed(&self) -> bool {
        self.shared
            .is_closed
            .load(std::sync::atomic::Ordering::Acquire)
    }

    /// Close the session.
    pub fn close(&self) {
        self.shared
            .is_closed
            .store(true, std::sync::atomic::Ordering::Release);
    }
}

/// Recv loop: reads frames from remote, dispatches to streams.
async fn recv_loop<R: AsyncRead + Unpin>(
    mut reader: R,
    accept_tx: mpsc::Sender<Stream>,
    write_tx: mpsc::Sender<WriteRequest>,
    shared: Arc<Shared>,
    version: u8,
) {
    let mut hdr = [0u8; HEADER_SIZE];

    loop {
        if shared.is_closed.load(std::sync::atomic::Ordering::Acquire) {
            break;
        }

        // Read header
        if let Err(e) = reader.read_exact(&mut hdr).await {
            tracing::debug!("recv: header read error: {e}");
            break;
        }

        let header = match Frame::decode_header(&hdr) {
            Some(h) => h,
            None => {
                tracing::warn!("recv: invalid header");
                break;
            }
        };

        if header.version != version {
            tracing::warn!(
                "recv: version mismatch: expected {version}, got {}",
                header.version
            );
            break;
        }

        // Read payload
        let payload = if header.has_payload() {
            let mut buf = vec![0u8; header.payload_len()];
            if let Err(e) = reader.read_exact(&mut buf).await {
                tracing::debug!("recv: payload read error: {e}");
                break;
            }
            Bytes::from(buf)
        } else {
            Bytes::new()
        };

        match header.cmd {
            CMD_SYN => {
                tracing::debug!("recv: SYN stream_id={}", header.stream_id);
                let (data_tx, data_rx) = mpsc::channel(64);
                {
                    let mut streams = shared.streams.lock().await;
                    streams.insert(header.stream_id, data_tx);
                }
                let stream = Stream::new(header.stream_id, data_rx, write_tx.clone());
                if accept_tx.send(stream).await.is_err() {
                    tracing::debug!("recv: accept channel closed");
                    break;
                }
            }
            CMD_FIN => {
                tracing::debug!("recv: FIN stream_id={}", header.stream_id);
                let mut streams = shared.streams.lock().await;
                streams.remove(&header.stream_id);
            }
            CMD_PSH => {
                tracing::debug!(
                    "recv: PSH stream_id={} len={}",
                    header.stream_id,
                    payload.len()
                );
                let mut streams = shared.streams.lock().await;
                if let Some(tx) = streams.get(&header.stream_id) {
                    if tx.send(payload).await.is_err() {
                        streams.remove(&header.stream_id);
                    }
                }
            }
            CMD_NOP => {
                tracing::debug!("recv: NOP");
            }
            _ => {
                tracing::warn!("recv: unknown cmd {}", header.cmd);
            }
        }
    }

    // Cleanup
    tracing::debug!("recv: loop ended");
    let mut streams = shared.streams.lock().await;
    streams.clear();
    shared
        .is_closed
        .store(true, std::sync::atomic::Ordering::Release);
}

/// Write loop: drains write requests to the remote.
async fn write_loop<W: AsyncWrite + Unpin>(
    mut writer: W,
    mut rx: mpsc::Receiver<WriteRequest>,
    shared: Arc<Shared>,
) {
    tracing::debug!("write: loop started");
    loop {
        if shared.is_closed.load(std::sync::atomic::Ordering::Acquire) {
            tracing::debug!("write: session closed, exiting");
            break;
        }

        match rx.recv().await {
            Some(msg) => {
                let frame = match msg {
                    WriteRequest::Syn { stream_id } => {
                        tracing::debug!("write: SYN stream_id={stream_id}");
                        Frame::syn(1, stream_id)
                    }
                    WriteRequest::Data { stream_id, data } => {
                        tracing::debug!("write: PSH stream_id={stream_id} len={}", data.len());
                        Frame::psh(1, stream_id, data)
                    }
                    WriteRequest::Fin { stream_id } => {
                        tracing::debug!("write: FIN stream_id={stream_id}");
                        Frame::fin(1, stream_id)
                    }
                };
                if let Err(e) = write_frame(&mut writer, &frame).await {
                    tracing::debug!("write: error: {e}");
                    shared
                        .is_closed
                        .store(true, std::sync::atomic::Ordering::Release);
                    break;
                }
            }
            None => {
                tracing::debug!("write: channel closed, exiting");
                break;
            }
        }
    }
}

/// Write a single frame to the transport.
async fn write_frame<W: AsyncWrite + Unpin>(
    writer: &mut W,
    frame: &Frame,
) -> Result<(), std::io::Error> {
    let mut hdr = [0u8; HEADER_SIZE];
    frame.encode_header(&mut hdr);
    writer.write_all(&hdr).await?;
    if !frame.data.is_empty() {
        writer.write_all(&frame.data).await?;
    }
    writer.flush().await
}
