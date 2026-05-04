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
//! - Dual-channel priority shaper via `tokio::select!`
//! - Built-in zero-overhead KeepAlive

use crate::{
    frame::{CMD_FIN, CMD_NOP, CMD_PSH, CMD_SYN, CMD_UPD, Frame, HEADER_SIZE},
    stream::Stream,
};
use bytes::Bytes;
use std::{collections::HashMap, sync::Arc, time::Duration};
use tokio::{
    io::{AsyncRead, AsyncReadExt, AsyncWrite, AsyncWriteExt},
    sync::{Mutex, Notify, mpsc},
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
    /// Maximum receive buffer (V2 flow control token bucket).
    pub max_receive_buffer:  usize,
    /// Maximum stream buffer (V2 flow control per stream window).
    pub max_stream_buffer:   usize,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            version:             1,
            keep_alive_disabled: true,
            keep_alive_interval: Duration::from_secs(10),
            keep_alive_timeout:  Duration::from_secs(30),
            max_frame_size:      32768,
            max_receive_buffer:  4 * 1024 * 1024,
            max_stream_buffer:   1024 * 1024,
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
    /// Send UPD frame (window update).
    Upd {
        stream_id: u32,
        consumed:  u32,
        window:    u32,
    },
}

pub(crate) struct StreamShared {
    pub peer_consumed: std::sync::atomic::AtomicU32,
    pub peer_window:   std::sync::atomic::AtomicU32,
    pub window_notify: Notify,
}

pub(crate) struct StreamEntry {
    pub data_tx:       mpsc::Sender<Bytes>,
    pub stream_shared: Arc<StreamShared>,
}

/// Shared state between session and spawned tasks.
pub(crate) struct Shared {
    /// Stream entries, keyed by stream_id.
    pub(crate) streams:           Mutex<HashMap<u32, StreamEntry>>,
    /// Session closed flag.
    pub(crate) is_closed:         std::sync::atomic::AtomicBool,
    /// Token bucket for session-level flow control (V2).
    pub(crate) bucket:            std::sync::atomic::AtomicI32,
    pub(crate) bucket_notify:     Notify,
    /// Last receive time in milliseconds since epoch.
    pub(crate) last_receive_time: std::sync::atomic::AtomicU64,
}

/// A multiplexed session over an underlying transport.
pub struct Session {
    /// Accept incoming streams.
    accept_rx:         mpsc::Receiver<Stream>,
    /// Control frame sender (high priority).
    ctrl_tx:           mpsc::Sender<WriteRequest>,
    /// Data frame sender (low priority).
    data_tx:           mpsc::Sender<WriteRequest>,
    /// Shared state.
    pub(crate) shared: Arc<Shared>,
    /// Config.
    config:            Config,
    /// Next stream ID for open().
    next_stream_id:    u32,
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
        let (ctrl_tx, ctrl_rx) = mpsc::channel(1024);
        let (data_tx, data_rx) = mpsc::channel(1024);

        let now_ms = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_millis() as u64;

        let shared = Arc::new(Shared {
            streams:           Mutex::new(HashMap::new()),
            is_closed:         std::sync::atomic::AtomicBool::new(false),
            bucket:            std::sync::atomic::AtomicI32::new(config.max_receive_buffer as i32),
            bucket_notify:     Notify::new(),
            last_receive_time: std::sync::atomic::AtomicU64::new(now_ms),
        });

        // Spawn recv loop
        let shared_recv = shared.clone();
        let ctrl_tx_recv = ctrl_tx.clone();
        tokio::spawn(recv_loop(
            reader,
            accept_tx,
            ctrl_tx_recv,
            shared_recv,
            config.clone(),
        ));

        // Spawn write loop
        let shared_write = shared.clone();
        tokio::spawn(write_loop(
            writer,
            ctrl_rx,
            data_rx,
            shared_write,
            config.clone(),
        ));

        let next_stream_id = if is_client { 1 } else { 0 };

        Self {
            accept_rx,
            ctrl_tx,
            data_tx,
            shared,
            config,
            next_stream_id,
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

        let (data_tx, data_rx) = mpsc::channel(64);
        let stream_shared = Arc::new(StreamShared {
            peer_consumed: std::sync::atomic::AtomicU32::new(0),
            peer_window:   std::sync::atomic::AtomicU32::new(262144), // initial peer window
            window_notify: Notify::new(),
        });

        {
            let mut streams = self.shared.streams.lock().await;
            streams.insert(stream_id, StreamEntry {
                data_tx,
                stream_shared: stream_shared.clone(),
            });
        }

        tracing::debug!("open: sending SYN for stream {stream_id}");
        let _ = self.ctrl_tx.send(WriteRequest::Syn { stream_id }).await;
        tracing::debug!("open: SYN sent for stream {stream_id}");

        Some(Stream::new(
            stream_id,
            data_rx,
            self.ctrl_tx.clone(),
            self.data_tx.clone(),
            self.shared.clone(),
            stream_shared,
            self.config.clone(),
        ))
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
        // Wake up loops
        self.shared.bucket_notify.notify_one();
    }
}

/// Recv loop: reads frames from remote, dispatches to streams.
async fn recv_loop<R: AsyncRead + Unpin>(
    mut reader: R,
    accept_tx: mpsc::Sender<Stream>,
    ctrl_tx: mpsc::Sender<WriteRequest>,
    shared: Arc<Shared>,
    config: Config,
) {
    let mut hdr = [0u8; HEADER_SIZE];
    let is_v2 = config.version == 2;

    loop {
        if shared.is_closed.load(std::sync::atomic::Ordering::Acquire) {
            break;
        }

        // Wait for tokens in V2
        if is_v2 {
            while shared.bucket.load(std::sync::atomic::Ordering::Acquire) <= 0
                && !shared.is_closed.load(std::sync::atomic::Ordering::Acquire)
            {
                shared.bucket_notify.notified().await;
            }
        }

        if shared.is_closed.load(std::sync::atomic::Ordering::Acquire) {
            break;
        }

        // Read header
        if let Err(e) = reader.read_exact(&mut hdr).await {
            tracing::debug!("recv: header read error: {e}");
            break;
        }

        let now_ms = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_millis() as u64;
        shared
            .last_receive_time
            .store(now_ms, std::sync::atomic::Ordering::Release);

        let header = match Frame::decode_header(&hdr) {
            Some(h) => h,
            None => {
                tracing::warn!("recv: invalid header");
                break;
            }
        };

        if header.version != config.version {
            tracing::warn!(
                "recv: version mismatch: expected {}, got {}",
                config.version,
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
            if is_v2 {
                shared
                    .bucket
                    .fetch_sub(buf.len() as i32, std::sync::atomic::Ordering::Release);
            }
            Bytes::from(buf)
        } else {
            Bytes::new()
        };

        match header.cmd {
            CMD_SYN => {
                tracing::debug!("recv: SYN stream_id={}", header.stream_id);
                let (data_tx, data_rx) = mpsc::channel(64);
                let stream_shared = Arc::new(StreamShared {
                    peer_consumed: std::sync::atomic::AtomicU32::new(0),
                    peer_window:   std::sync::atomic::AtomicU32::new(262144),
                    window_notify: Notify::new(),
                });

                {
                    let mut streams = shared.streams.lock().await;
                    streams.insert(header.stream_id, StreamEntry {
                        data_tx,
                        stream_shared: stream_shared.clone(),
                    });
                }
                let stream = Stream::new(
                    header.stream_id,
                    data_rx,
                    ctrl_tx.clone(),
                    ctrl_tx.clone(),
                    shared.clone(),
                    stream_shared,
                    config.clone(),
                );
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
                if let Some(entry) = streams.get(&header.stream_id) {
                    if entry.data_tx.send(payload).await.is_err() {
                        streams.remove(&header.stream_id);
                    }
                }
            }
            CMD_UPD => {
                if !is_v2 {
                    tracing::warn!("recv: UPD on V1");
                    break;
                }
                if payload.len() == 8 {
                    let consumed =
                        u32::from_le_bytes([payload[0], payload[1], payload[2], payload[3]]);
                    let window =
                        u32::from_le_bytes([payload[4], payload[5], payload[6], payload[7]]);
                    let streams = shared.streams.lock().await;
                    if let Some(entry) = streams.get(&header.stream_id) {
                        entry
                            .stream_shared
                            .peer_consumed
                            .store(consumed, std::sync::atomic::Ordering::Release);
                        entry
                            .stream_shared
                            .peer_window
                            .store(window, std::sync::atomic::Ordering::Release);
                        entry.stream_shared.window_notify.notify_one();
                    }
                } else {
                    tracing::warn!("recv: invalid UPD payload");
                    break;
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

/// Write loop: drains write requests to the remote with priority shaping.
async fn write_loop<W: AsyncWrite + Unpin>(
    mut writer: W,
    mut ctrl_rx: mpsc::Receiver<WriteRequest>,
    mut data_rx: mpsc::Receiver<WriteRequest>,
    shared: Arc<Shared>,
    config: Config,
) {
    tracing::debug!("write: loop started");
    let mut interval = tokio::time::interval(config.keep_alive_interval);
    // Tick immediately so we don't delay first keepalive unnecessarily, but we can skip the first.
    interval.tick().await;

    loop {
        if shared.is_closed.load(std::sync::atomic::Ordering::Acquire) {
            tracing::debug!("write: session closed, exiting");
            break;
        }

        let msg_opt;

        if !config.keep_alive_disabled {
            tokio::select! {
                biased;
                msg = ctrl_rx.recv() => msg_opt = msg,
                msg = data_rx.recv() => msg_opt = msg,
                _ = interval.tick() => {
                    let now_ms = std::time::SystemTime::now()
                        .duration_since(std::time::UNIX_EPOCH)
                        .unwrap()
                        .as_millis() as u64;
                    let last_recv = shared.last_receive_time.load(std::sync::atomic::Ordering::Acquire);
                    if now_ms.saturating_sub(last_recv) > config.keep_alive_timeout.as_millis() as u64 {
                        tracing::warn!("write: keepalive timeout");
                        shared.is_closed.store(true, std::sync::atomic::Ordering::Release);
                        break;
                    }
                    tracing::debug!("write: NOP");
                    if let Err(_) = write_frame(&mut writer, &Frame::nop(config.version)).await {
                        break;
                    }
                    // Continue to next loop without processing msg_opt
                    continue;
                }
            }
        } else {
            tokio::select! {
                biased;
                msg = ctrl_rx.recv() => msg_opt = msg,
                msg = data_rx.recv() => msg_opt = msg,
            }
        }

        match msg_opt {
            Some(msg) => {
                let frame = match msg {
                    WriteRequest::Syn { stream_id } => {
                        tracing::debug!("write: SYN sid={stream_id}");
                        Frame::syn(config.version, stream_id)
                    }
                    WriteRequest::Data { stream_id, data } => {
                        tracing::debug!("write: PSH sid={stream_id} len={}", data.len());
                        Frame::psh(config.version, stream_id, data)
                    }
                    WriteRequest::Fin { stream_id } => {
                        tracing::debug!("write: FIN sid={stream_id}");
                        Frame::fin(config.version, stream_id)
                    }
                    WriteRequest::Upd {
                        stream_id,
                        consumed,
                        window,
                    } => {
                        tracing::debug!("write: UPD sid={stream_id} cons={consumed} win={window}");
                        Frame::upd(config.version, stream_id, consumed, window)
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
