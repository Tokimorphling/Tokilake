//! # tokilake-smux
//!
//! Wire-compatible smux protocol implementation for Rust.
//!
//! Zero-overhead design following monolake patterns:
//! - Generic over transport (`AsyncRead + AsyncWrite`), no `Box<dyn>`
//! - `impl Future` in trait returns, no `async_trait` crate
//! - Channel-based stream multiplexing
//!
//! ## Protocol
//!
//! Header format (8 bytes, little-endian):
//! ```text
//! | VERSION (1B) | CMD (1B) | LENGTH (2B) | STREAM_ID (4B) |
//! ```
//!
//! Commands: SYN(0), FIN(1), PSH(2), NOP(3), UPD(4)
//!
//! ## Usage
//!
//! ```ignore
//! use tokilake_smux::{Session, Config};
//!
//! // Server side
//! let mut session = Session::server(stream, Config::default());
//! let mut stream = session.accept().await?;
//!
//! // Client side
//! let mut session = Session::client(stream, Config::default());
//! let mut stream = session.open().await?;
//! ```

mod frame;
mod session;
mod stream;

pub use frame::{Frame, HEADER_SIZE};
pub use session::{Config, Session};
pub use stream::Stream;
