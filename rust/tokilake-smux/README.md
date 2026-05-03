# tokilake-smux

A wire-compatible smux protocol implementation in Rust, optimized for high performance and zero overhead.

## Overview

This crate provides a Rust implementation of the smux protocol, originally popularized by [xtaci/smux](https://github.com/xtaci/smux) in Go. It is designed to work seamlessly within the `tokilake` ecosystem and other high-concurrency Rust network applications.

### Key Features

- **Zero-overhead Design**: Generic over transport layers (`AsyncRead + AsyncWrite`) without requiring dynamic dispatch (`Box<dyn>`).
- **No `async_trait`**: Utilizes modern Rust's `impl Future` in traits for maximum performance.
- **Protocol Compatible**: Fully wire-compatible with the original Go implementations (v1).
- **Concurrent Mutliplexing**: Channel-based internal stream multiplexing via `tokio`.

## Usage

```rust
use tokilake_smux::{Session, Config};
use tokio::net::TcpStream;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let stream = TcpStream::connect("127.0.0.1:8080").await?;
    
    // Initialize a client-side multiplexed session
    let mut session = Session::client(stream, Config::default());
    
    // Open a logical stream over the single TCP connection
    let mut stream1 = session.open().await.unwrap();
    let mut stream2 = session.open().await.unwrap();

    // Use streams like standard tokio AsyncRead/AsyncWrite
    // ...

    Ok(())
}
```

## Protocol Details

The protocol uses an 8-byte little-endian header format:
```text
| VERSION (1B) | CMD (1B) | LENGTH (2B) | STREAM_ID (4B) |
```

Commands supported: `SYN(0)`, `FIN(1)`, `PSH(2)`, `NOP(3)`.

## License

MIT License.
