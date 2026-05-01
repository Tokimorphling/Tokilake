<p align="right">
   <strong>English</strong> | <a href="./README.zh-CN.md">中文</a>
</p>

# Tokilake & Tokiame

> **Control your own GPUs like OpenRouter.**

Tokilake is a decentralized Large Language Model (LLM) API scheduling gateway built on the One-API ecosystem. It completely flips the traditional API gateway model: instead of the gateway strictly acting as a client that actively requests servers with public IPs, it **allows any GPU worker node (Tokiame) located behind NAT/Intranets to actively connect to the central gateway (Hub) via a reverse tunnel (WebSocket or QUIC)**.

> **Tokilake** is built on top of [MartialBE/one-hub](https://github.com/MartialBE/one-hub) and the broader One-API ecosystem that evolved around it.

## 📖 Quick Start

You can visit the [Tokilake Demo](https://tokilake.abrdns.com/) to explore the core features. For total data privacy and distribution control, we highly recommend self-hosting following our **End-to-End Deployment Guide**. Whether you're a first-time user or ready for a full deployment, the guides below are here to help.

- **[📚 中文使用指南](./docs/guide.zh.md)**
- **[📖 User Guide (English)](./docs/guide.en.md)**
- **[🖼️ Image Generation Guide](./docs/ImageGen.md)**
- **[🖼️ 图像生成指南](./docs/ImageGen.zh.md)**
- **[🎨 Image Gen via Chat Completions (OpenAI SDK)](./docs/ImageGenChat.md)**
- **[🎨 通过 Chat Completions 生成图像（OpenAI SDK）](./docs/ImageGenChat.zh.md)**

## 🚀 One-Click Deployment (Recommended)

The fastest way to deploy your own Tokilake Hub with automatic HTTPS and QUIC support:

```bash
# 1. Clone the repository
git clone https://github.com/Tokimorphling/Tokilake.git
cd Tokilake

# 2. Production deploy with host nginx + Docker + Let's Encrypt
sudo ./deploy/bootstrap-nginx-letsencrypt.sh \
  --domain api.example.com \
  --email admin@example.com \
  --sql-dsn 'postgres://user:password@127.0.0.1:5432/tokilake'
```

Access your dashboard at `https://api.example.com`.

To update the image later:

```bash
sudo ./deploy/bootstrap-nginx-letsencrypt.sh \
  --domain api.example.com \
  --update
```

For a local-only smoke test without nginx or certificates:

```bash
docker run -d \
  --name tokilake-local \
  --restart unless-stopped \
  -p 19981:19981 \
  -e TZ=UTC \
  -e PORT=19981 \
  -e GIN_MODE=release \
  -e SERVER_ADDRESS="http://localhost:19981" \
  -e USER_TOKEN_SECRET="$(openssl rand -hex 32)" \
  -e SESSION_SECRET="$(openssl rand -hex 32)" \
  -v tokilake-local-data:/data \
  ghcr.io/tokimorphling/tokilake:latest
```

Then open `http://localhost:19981`.


## 🌟 Core Concept

Traditional API proxies typically act as clients, routing requests to servers with public IP addresses. If your high-performance GPUs (like an RTX 4090) are sitting quietly on a local home network, or scattered across temporary Spot Instances from various cloud providers, unifying them into a stable, accessible API is a major challenge.

**Tokiame** changes the game. Operating as a lightweight daemon, it actively "dials out" to connect to the cloud-based **Tokilake** gateway. Upon a successful connection, Tokilake seamlessly maps the worker internally to a standard `Channel`. This means **you don't need any tricky intranet penetration tools (like FRP or Ngrok). You get to enjoy the gateway's enterprise-grade load balancing, high-concurrency traffic shaping, authentication, and billing systems right out of the box.**

## 🚀 Perfect Use Cases

### 1. Distributed GPU Pooling for Individuals & Studios (NAT Penetration)
Tailor-made for home broadband or campus network environments without public IPs. Just run the Tokiame process locally, and it instantly establishes a tunnel with the cloud gateway. The LLMs you deploy locally using Ollama or vLLM can instantly and securely provide standard OpenAI-compatible API services to the outside world.

### 2. Hybrid Cloud Orchestration
Purchased scattered GPU instances across different compute platforms (e.g., AWS, AliCloud, AutoDL, RunPod)? Skip the complex SD-WAN setups. Simply attach the Tokiame startup script to your new instances, and they automatically register into the load-balancing pool. When instances are destroyed or shut down, the heartbeat mechanism safely takes the node offline, drastically reducing DevOps overhead.

### 3. Enterprise Data Privacy & "Bring Your Own Model" (BYOM)
SaaS providers handle the business logic frontend, while clients provide the compute backend. Clients only need to deploy Tokiame within their highly secure private server rooms, initiating a one-way outbound connection to the SaaS gateway. The client's server room **exposes absolutely zero inbound ports**, yet perfectly completes the business scheduling of private models, satisfying the most stringent security audit requirements.

### 4. Community Compute Sharing & C2C API Trading
Built around native **Private Group** and **Invite Code** mechanisms. User A hooks up their compute node and generates an invite code; User B redeems the code, gains access to User A's private multi-tenant environment, and invokes the compute power. The gateway handles all centralized billing and authentication, making it effortless to build your very own "OpenRouter."

## 🛠 Architecture Design

```mermaid
graph TB
    subgraph Users ["🌐 API Consumers"]
        U1["Apps / SDKs"]
        U2["curl / ChatUI"]
    end

    subgraph Gateway ["☁️ Tokilake Gateway (Hub)"]
        GIN["Gin HTTP Server"]
        RELAY["Relay Router"]
        PROV["Tokiame Provider"]
        SM["Session Manager"]
        DB[("DB / Channel Table")]
        GIN --> RELAY --> PROV
        PROV -->|"Lookup Session"| SM
        SM -->|"R/W Virtual Channel"| DB
    end

    subgraph Tunnel ["🔒 Multiplexed Reverse Tunnel"]
        direction LR
        CTRL["Control Stream<br/>register / heartbeat / models_sync"]
        DATA["Data Streams<br/>TunnelRequest ↔ TunnelResponse"]
    end

    subgraph Workers ["🖥️ Tokiame Edge Nodes (Behind NAT)"]
        W1["Tokiame Client A"]
        W2["Tokiame Client B"]
        B1["Ollama / vLLM<br/>Local GPU"]
        B2["SGLang / ComfyUI<br/>Local GPU"]
        W1 --> B1
        W2 --> B2
    end

    U1 & U2 -->|"Standard OpenAI HTTP API"| GIN
    PROV <-->|"Multiplexed Tunnel"| Tunnel
    Tunnel <-->|"Outbound-Only Connection"| W1 & W2
```

- **`Tokilake` (Gateway/Hub Level)**: The unified ingress for traffic. It receives standard HTTP API requests from end-users and multiplexes them to the corresponding edge nodes.
- **`Tokiame` (Node/Worker Level)**: The lightweight client on the edge. It maintains an ultra-low latency reverse tunnel via either WebSocket (with `xtaci/smux` multiplexing) or QUIC.
- **`tokilake-core`**: The standalone protocol, tunnel, session, and gateway core. It has no onehub database dependency. **[📖 Integrate tokilake-core into your own gateway →](./docs/tokilake-core-integration.md)**
- **`tokilake-onehub`**: The onehub adapter that maps connected workers into channels, providers, and video tasks.

### Transport Options

Tokiame supports two transport protocols:

| Protocol | Description |
|----------|-------------|
| **WebSocket** (default) | Uses `xtaci/smux` for stream multiplexing over WebSocket. Compatible with standard HTTP/HTTPS gateways. |
| **QUIC** | Native QUIC protocol (`quic-go`) with built-in multiplexing and 0-RTT connection establishment. Requires TLS and a dedicated QUIC-enabled gateway endpoint. |

**Transport Mode Selection** (set via `TOKIAME_TRANSPORT_MODE`):
- `auto` (default): Attempts QUIC first, falls back to WebSocket if connection fails
- `quic`: QUIC only
- `websocket`: WebSocket only

QUIC is ideal for scenarios requiring lower latency and better connection resilience, especially on unreliable networks. It also supports server-side connection migration.

### Simplified Workflow
1. The `Tokiame` client initiates a WebSocket or QUIC connection request to `Tokilake` using a standard user API token.
2. Upon successful gateway verification, it automatically creates/binds a virtual `Channel` (`type=100`) in the database and assigns it to a specific Private Group.
3. When a user sends an LLM HTTP request through the gateway, the gateway treats it like any normal channel, transparently streaming it to the edge node for processing via the tunnel.
4. Relies on real-time heartbeat keepalives. If an edge node loses its connection, the gateway automatically disables its virtual Channel, achieving zero-downtime Failover.

## 🆚 How Tokilake Compares

Tokilake occupies a unique position in the open-source LLM infrastructure landscape — it is the only project that combines **API aggregation gateway**, **distributed remote worker registration**, and **tunnel-based NAT traversal** in a single system.

### Architecture Comparison

```
┌─────────────────────────────────────────────────────────────────────┐
│                        one-api / new-api / LiteLLM                  │
│                                                                     │
│   ┌──────────┐    ┌──────────┐    ┌──────────┐                     │
│   │ OpenAI   │    │ Claude   │    │ Gemini   │   Static backends   │
│   │ (public) │    │ (public) │    │ (public) │   Manual config     │
│   └────┬─────┘    └────┬─────┘    └────┬─────┘                     │
│        └───────────────┼───────────────┘                            │
│                        ▼                                            │
│               ┌─────────────────┐                                   │
│               │  API Gateway    │   No worker registration          │
│               │  (aggregation)  │   No NAT traversal                │
│               └─────────────────┘                                   │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                              exo                                    │
│                                                                     │
│   ┌─────────┐  TB5  ┌─────────┐  TB5  ┌─────────┐                 │
│   │ Mac A   │◄─────►│ Mac B   │◄─────►│ Mac C   │  One big model  │
│   │ shard 1 │       │ shard 2 │       │ shard 3 │  split across   │
│   └─────────┘       └─────────┘       └─────────┘  devices         │
│                                                                     │
│   Requires high-bandwidth interconnect (Thunderbolt / InfiniBand)   │
│   P2P auto-discovery, LAN only                                      │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                           Tokilake                                  │
│                                                                     │
│              ┌─────────────────────┐                                │
│              │   Tokilake Gateway  │   Central hub                   │
│              │  (API aggregation)  │   OpenAI-compatible API         │
│              └────────┬────────────┘                                │
│                       │                                             │
│          ┌────────────┼────────────┐                                │
│          │ WS/QUIC    │ WS/QUIC    │ WS/QUIC     NAT traversal     │
│          │ tunnel     │ tunnel     │ tunnel      (outbound only)    │
│          ▼            ▼            ▼                                 │
│   ┌──────────┐ ┌──────────┐ ┌──────────┐                           │
│   │Tokiame A │ │Tokiame B │ │Tokiame C │   Independent workers     │
│   │ Ollama   │ │ vLLM     │ │ SGLang   │   Each runs own models    │
│   │ (home)   │ │ (cloud)  │ │ (edge)   │   Heterogeneous hardware  │
│   └──────────┘ └──────────┘ └──────────┘                           │
└─────────────────────────────────────────────────────────────────────┘
```

### Feature Matrix

| Capability | one-api / new-api | LiteLLM | exo | **Tokilake** |
|---|:---:|:---:|:---:|:---:|
| Multi-provider API aggregation | ✅ | ✅ | ❌ | ✅ |
| Static backend configuration | ✅ | ✅ | ❌ | ✅ |
| Remote worker auto-registration | ❌ | ❌ | ✅ | ✅ |
| Tunnel-based NAT traversal | ❌ | ❌ | ❌ | ✅ |
| Model sharding across devices | ❌ | ❌ | ✅ | ❌ |
| Heterogeneous hardware support | N/A | N/A | Limited | ✅ |
| Works over public internet | ✅ | ✅ | ❌ | ✅ |
| Zero inbound ports on workers | N/A | N/A | ❌ | ✅ |
| Heartbeat & auto-failover | ❌ | ❌ | ✅ | ✅ |

### When to Choose Tokilake

- **You have GPUs scattered across different locations** (home, cloud, edge) and want to unify them behind one API
- **Your workers are behind NAT/firewalls** and you can't or don't want to set up FRP/Ngrok
- **You want different workers running different models** rather than sharding one model across devices
- **You need the full one-api ecosystem** (billing, auth, groups, admin UI) plus distributed compute

## Acknowledgements

- [songquanpeng/one-api](https://github.com/songquanpeng/one-api): the architectural foundation of this project.
- [Calcium-Ion/new-api](https://github.com/Calcium-Ion/new-api): reference for some provider integrations and async task patterns.
- [codedthemes/berry-free-react-admin-template](https://github.com/codedthemes/berry-free-react-admin-template): visual base for the admin frontend.
- [minimal-ui-kit/material-kit-react](https://github.com/minimal-ui-kit/material-kit-react): reference for parts of the UI styling.
- [zeromicro/go-zero](https://github.com/zeromicro/go-zero): reference for rate limiting and related implementations.

## Legacy README

[Legacy README (historical introduction and compatibility notes)](./README.legacy.md)
