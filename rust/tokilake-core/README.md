# tokilake-core

The high-performance core tunnel and gateway abstraction library for the Tokilake ecosystem.

## Overview

`tokilake-core` provides the fundamental trait definitions, session management, routing logic, and protocol codecs required to build scalable, multiplexed gateways. It sits between the network transport layer and the application edge.

### Core Architecture

- **`tunnel`**: Transport-agnostic tunnel traits (`TunnelSession`, `TunnelStream`) using zero-cost `impl Future` architectures.
- **`session`**: Lock-free, concurrent worker registration and namespace claiming via `DashMap`.
- **`roundtrip`**: Asynchronous HTTP-over-Tunnel request/response forwarding with body chunk pumping.
- **`protocol`**: Cross-platform NDJSON protocol definitions used by control planes.
- **`gateway`**: Extensible HTTP/WebSocket handler logic.

### Supported Transports
- **SMUX**: Backward-compatible with standard `tokilake` workers through `tokilake-smux`.
- **QUIC**: Next-generation, zero-RTT capable high-throughput encrypted transport powered by `quinn`.
- **Memory**: In-memory stream channels for rapid unit testing.

## Integration

`tokilake-core` is meant to be embedded into applications like `tokilake-server`.

```rust
use tokilake_core::session::SessionManager;
use tokilake_core::tunnel::smux::SmuxSession;

// Initialize the global session manager
let session_manager = SessionManager::<SmuxSession>::new();

// Handle incoming multiplexed streams through the unified traits...
```

## License

MIT License.
