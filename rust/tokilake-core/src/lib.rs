//! # tokilake-core
//!
//! High-performance tunnel gateway core library.
//!
//! ## Architecture
//!
//! - [`protocol`]: Message types for control and data planes
//! - [`tunnel`]: Tunnel session/stream abstractions
//! - [`session`]: Gateway session management
//! - [`gateway`]: Core gateway logic
//! - [`codec`]: NDJSON message codecs

pub mod codec;
pub mod error;
pub mod gateway;
pub mod protocol;
pub mod roundtrip;
pub mod service;
pub mod session;
pub mod tunnel;

pub use anyhow::{Error as AnyError, Result as AnyResult};
