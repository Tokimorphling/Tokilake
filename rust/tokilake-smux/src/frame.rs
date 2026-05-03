//! SMUX frame types and codec.
//!
//! Wire format (8 bytes header):
//! ```text
//! | VERSION (1B) | CMD (1B) | LENGTH (2B LE) | STREAM_ID (4B LE) |
//! ```
//!
//! Commands:
//! - `SYN` (0): Open a new stream
//! - `FIN` (1): Close a stream (EOF)
//! - `PSH` (2): Data push
//! - `NOP` (3): No operation (keepalive)
//! - `UPD` (4): Window update (v2 only)

use bytes::Bytes;
use std::fmt;

/// Protocol version 1.
pub const VERSION_1: u8 = 1;

/// Protocol version 2 (adds flow control).
pub const VERSION_2: u8 = 2;

/// Stream open command.
pub const CMD_SYN: u8 = 0;
/// Stream close command (EOF).
pub const CMD_FIN: u8 = 1;
/// Data push command.
pub const CMD_PSH: u8 = 2;
/// No operation (keepalive).
pub const CMD_NOP: u8 = 3;
/// Window update (v2 only).
pub const CMD_UPD: u8 = 4;

/// Header size in bytes: version(1) + cmd(1) + length(2) + stream_id(4).
pub const HEADER_SIZE: usize = 8;

/// Maximum payload size (u16::MAX).
pub const MAX_PAYLOAD_SIZE: usize = 65535;

/// A decoded smux frame.
#[derive(Debug, Clone)]
pub struct Frame {
    /// Protocol version.
    pub version:   u8,
    /// Command type.
    pub cmd:       u8,
    /// Stream identifier.
    pub stream_id: u32,
    /// Payload data.
    pub data:      Bytes,
}

impl Frame {
    /// Create a new frame with no payload.
    pub fn new(version: u8, cmd: u8, stream_id: u32) -> Self {
        Self {
            version,
            cmd,
            stream_id,
            data: Bytes::new(),
        }
    }

    /// Create a SYN frame (stream open).
    pub fn syn(version: u8, stream_id: u32) -> Self {
        Self::new(version, CMD_SYN, stream_id)
    }

    /// Create a FIN frame (stream close).
    pub fn fin(version: u8, stream_id: u32) -> Self {
        Self::new(version, CMD_FIN, stream_id)
    }

    /// Create a PSH frame (data push).
    pub fn psh(version: u8, stream_id: u32, data: Bytes) -> Self {
        Self {
            version,
            cmd: CMD_PSH,
            stream_id,
            data,
        }
    }

    /// Create a NOP frame (keepalive).
    pub fn nop(version: u8) -> Self {
        Self::new(version, CMD_NOP, 0)
    }

    /// Returns true if this is a SYN frame.
    pub fn is_syn(&self) -> bool {
        self.cmd == CMD_SYN
    }

    /// Returns true if this is a FIN frame.
    pub fn is_fin(&self) -> bool {
        self.cmd == CMD_FIN
    }

    /// Returns true if this is a PSH (data) frame.
    pub fn is_psh(&self) -> bool {
        self.cmd == CMD_PSH
    }

    /// Returns true if this is a NOP (keepalive) frame.
    pub fn is_nop(&self) -> bool {
        self.cmd == CMD_NOP
    }

    /// Encode the frame header into a buffer.
    ///
    /// The buffer must have at least [`HEADER_SIZE`] bytes of capacity.
    pub fn encode_header(&self, buf: &mut [u8]) {
        debug_assert!(buf.len() >= HEADER_SIZE);
        buf[0] = self.version;
        buf[1] = self.cmd;
        buf[2..4].copy_from_slice(&(self.data.len() as u16).to_le_bytes());
        buf[4..8].copy_from_slice(&self.stream_id.to_le_bytes());
    }

    /// Decode a frame header from raw bytes.
    ///
    /// Returns `None` if the buffer is too short.
    pub fn decode_header(buf: &[u8]) -> Option<FrameHeader> {
        if buf.len() < HEADER_SIZE {
            return None;
        }
        Some(FrameHeader {
            version:   buf[0],
            cmd:       buf[1],
            length:    u16::from_le_bytes([buf[2], buf[3]]),
            stream_id: u32::from_le_bytes([buf[4], buf[5], buf[6], buf[7]]),
        })
    }
}

impl fmt::Display for Frame {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "Frame(version={}, cmd={}, sid={}, len={})",
            self.version,
            cmd_name(self.cmd),
            self.stream_id,
            self.data.len()
        )
    }
}

/// A decoded frame header (without payload).
#[derive(Debug, Clone, Copy)]
pub struct FrameHeader {
    pub version:   u8,
    pub cmd:       u8,
    pub length:    u16,
    pub stream_id: u32,
}

impl FrameHeader {
    /// Returns the payload length.
    pub fn payload_len(&self) -> usize {
        self.length as usize
    }

    /// Returns true if this frame has a payload.
    pub fn has_payload(&self) -> bool {
        self.length > 0
    }
}

impl fmt::Display for FrameHeader {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "FrameHeader(version={}, cmd={}, sid={}, len={})",
            self.version,
            cmd_name(self.cmd),
            self.stream_id,
            self.length
        )
    }
}

/// Return a human-readable command name.
fn cmd_name(cmd: u8) -> &'static str {
    match cmd {
        CMD_SYN => "SYN",
        CMD_FIN => "FIN",
        CMD_PSH => "PSH",
        CMD_NOP => "NOP",
        CMD_UPD => "UPD",
        _ => "UNKNOWN",
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_frame_encode_decode_roundtrip() {
        let frame = Frame::psh(1, 42, Bytes::from("hello"));
        let mut buf = [0u8; HEADER_SIZE];
        frame.encode_header(&mut buf);

        let header = Frame::decode_header(&buf).unwrap();
        assert_eq!(header.version, 1);
        assert_eq!(header.cmd, CMD_PSH);
        assert_eq!(header.length, 5);
        assert_eq!(header.stream_id, 42);
    }

    #[test]
    fn test_frame_syn() {
        let frame = Frame::syn(1, 1);
        assert!(frame.is_syn());
        assert!(!frame.is_fin());
        assert_eq!(frame.data.len(), 0);
    }

    #[test]
    fn test_frame_fin() {
        let frame = Frame::fin(1, 2);
        assert!(frame.is_fin());
        assert_eq!(frame.stream_id, 2);
    }

    #[test]
    fn test_decode_header_too_short() {
        assert!(Frame::decode_header(&[0, 1, 2]).is_none());
    }
}
