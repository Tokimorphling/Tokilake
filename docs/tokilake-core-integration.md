---
title: "Integrating tokilake-core into Your Own Gateway"
outline: deep
lastUpdated: true
---

# Integrating tokilake-core into Your Own Gateway

`tokilake-core` is a standalone Go module with **zero dependency on onehub or any database**. You can embed it into any Go HTTP server to add Tokiame worker support — the same tunnel, session management, and multiplexed request forwarding that Tokilake uses, but with your own auth, routing, and persistence layer.

```
your-gateway/
├── main.go                  # Your HTTP server (Gin, net/http, Echo, etc.)
├── auth.go                  # Your auth logic
├── registry.go              # Your worker persistence (DB, Redis, etc.)
└── go.mod
    └── require github.com/Tokimorphling/Tokilake/tokilake-core vX.Y.Z
```

## Module Layout

`tokilake-core` exposes these packages:

| File | Purpose |
|------|---------|
| `interfaces.go` | Interfaces you implement: `Authenticator`, `WorkerRegistry`, `RouteResolver`, `Logger` |
| `gateway.go` | `Gateway` — accepts WebSocket connections, runs the control stream state machine |
| `quic_gateway.go` | Optional QUIC listener for the same gateway |
| `session_manager.go` | `SessionManager` — maps namespaces and channel IDs to live sessions |
| `tunnel.go` | `TunnelSession` / `TunnelStream` abstractions over smux and QUIC |
| `tunnel_stream.go` | `TunnelStreamCodec` — newline-delimited JSON framing for request/response |
| `roundtrip.go` | `DoTunnelRequest` / `DoTunnelRequestByNamespace` — send a request through the tunnel and get an `*http.Response` back |
| `protocol.go` | `ControlCodec` and control message types (auth, register, heartbeat, etc.) |
| `ws_conn.go` | `WebsocketStreamConn` — adapts a `*websocket.Conn` into an `io.ReadWriteCloser` for smux |

## Step 1: Install the Module

```bash
go get github.com/Tokimorphling/Tokilake/tokilake-core@latest
```

In your `go.mod`:

```go
require github.com/Tokimorphling/Tokilake/tokilake-core v0.0.0
```

## Step 2: Implement the Four Interfaces

`tokilake-core` defines four interfaces in `interfaces.go`. You provide your own implementations:

```go
type Authenticator interface {
    AuthenticateTokenKey(ctx context.Context, tokenKey string) (string, *Token, error)
}

type WorkerRegistry interface {
    RegisterWorker(ctx context.Context, session *GatewaySession, register *RegisterMessage) (*RegisterResult, error)
    UpdateHeartbeat(ctx context.Context, session *GatewaySession, heartbeat *HeartbeatMessage) error
    SyncModels(ctx context.Context, session *GatewaySession, modelsSync *ModelsSyncMessage) error
    CleanupWorker(ctx context.Context, session *GatewaySession) error
}

type RouteResolver interface {
    ResolveRoute(ctx context.Context, namespace string) error
}

type Logger interface {
    SysLog(msg string)
    SysError(msg string)
    FatalLog(msg string)
}
```

### Authenticator

Called when a Tokiame client sends its token during WebSocket handshake or in-stream auth. Return the cleaned token key and a `*Token` (which only carries `UserId` — add more fields if you need them).

```go
type MyAuth struct {
    // your dependencies (DB, JWT secret, etc.)
}

func (a *MyAuth) AuthenticateTokenKey(ctx context.Context, tokenKey string) (string, *tokilake.Token, error) {
    // Validate tokenKey against your user/token store.
    user, err := a.db.ValidateToken(tokenKey)
    if err != nil {
        return "", nil, err
    }
    return tokenKey, &tokilake.Token{UserId: user.ID}, nil
}
```

### WorkerRegistry

Called when workers register, heartbeat, sync models, or disconnect. This is where you persist worker state and map workers to your routing layer.

```go
type MyRegistry struct {
    manager *tokilake.SessionManager
    db      *sql.DB
}

func (r *MyRegistry) RegisterWorker(ctx context.Context, session *tokilake.GatewaySession, reg *tokilake.RegisterMessage) (*tokilake.RegisterResult, error) {
    namespace := strings.TrimSpace(reg.Namespace)
    if err := r.manager.ClaimNamespace(session, namespace); err != nil {
        return nil, err
    }

    // Persist to your DB, assign a channel ID, etc.
    channelID, err := r.db.CreateOrUpdateWorker(namespace, reg.Models, session.Token.UserId)
    if err != nil {
        r.manager.Release(session)
        return nil, err
    }

    r.manager.BindChannel(session, channelID, channelID, reg.Group, reg.Models, reg.BackendType, 1)

    return &tokilake.RegisterResult{
        WorkerID:  channelID,
        ChannelID: channelID,
        Namespace: namespace,
        Models:    reg.Models,
        Status:    1,
    }, nil
}

func (r *MyRegistry) UpdateHeartbeat(ctx context.Context, session *tokilake.GatewaySession, hb *tokilake.HeartbeatMessage) error {
    return r.db.UpdateHeartbeat(session.WorkerID, hb.Status, hb.CurrentModels)
}

func (r *MyRegistry) SyncModels(ctx context.Context, session *tokilake.GatewaySession, sync *tokilake.ModelsSyncMessage) error {
    return r.db.UpdateModels(session.ChannelID, sync.Models)
}

func (r *MyRegistry) CleanupWorker(ctx context.Context, session *tokilake.GatewaySession) error {
    r.manager.Release(session)
    return r.db.MarkOffline(session.WorkerID)
}
```

### RouteResolver (Optional)

Called to verify a namespace is routable before forwarding. Return `nil` if you don't need this check.

```go
type MyResolver struct{}

func (r *MyResolver) ResolveRoute(ctx context.Context, namespace string) error {
    // Optional: check if the namespace has an active session
    return nil
}
```

### Logger

A simple adapter for your logging system:

```go
type MyLogger struct{}

func (l *MyLogger) SysLog(msg string)  { slog.Info(msg) }
func (l *MyLogger) SysError(msg string) { slog.Error(msg) }
func (l *MyLogger) FatalLog(msg string) { slog.Error(msg); os.Exit(1) }
```

## Step 3: Wire Up the Gateway

```go
package main

import (
    "context"
    "net/http"

    tokilake "github.com/Tokimorphling/Tokilake/tokilake-core"
    "github.com/gorilla/websocket"
)

func main() {
    // 1. Create shared session manager
    manager := tokilake.NewSessionManager()

    // 2. Provide your implementations
    auth := &MyAuth{}
    registry := &MyRegistry{manager: manager, db: myDB}
    resolver := &MyResolver{}
    logger := &MyLogger{}

    // 3. Create the gateway
    gw := tokilake.NewGateway(auth, registry, resolver, logger, manager)

    // 4. WebSocket upgrader
    upgrader := websocket.Upgrader{
        Subprotocols: []string{"tokilake.v1"},
        CheckOrigin:  func(r *http.Request) bool { return true },
    }

    // 5. WebSocket connect handler
    http.HandleFunc("/api/tokilake/connect", func(w http.ResponseWriter, r *http.Request) {
        tokenKey, token, err := gw.AuthenticateConnectRequest(r.Context(), r)
        if err != nil {
            http.Error(w, err.Error(), http.StatusUnauthorized)
            return
        }

        wsConn, err := upgrader.Upgrade(w, r, nil)
        if err != nil {
            return
        }
        defer wsConn.Close()

        if err := gw.HandleGatewayConnection(r.Context(), wsConn, token, tokenKey, r.RemoteAddr); err != nil {
            logger.SysError("session closed: " + err.Error())
        }
    })

    // 6. API endpoint that forwards through the tunnel
    http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
        // Resolve which namespace to route to (your logic)
        namespace := resolveNamespace(r) // e.g. from query param, header, or model mapping

        body, _ := io.ReadAll(r.Body)
        defer r.Body.Close()

        tunnelReq := &tokilake.TunnelRequest{
            RouteKind: tokilake.TunnelRouteKindChatCompletions,
            Method:    r.Method,
            Path:      r.URL.Path,
            Model:     extractModel(body),
            Headers:   map[string]string{"Content-Type": r.Header.Get("Content-Type")},
            Body:      body,
        }

        resp, requestID, err := gw.DoTunnelRequestByNamespace(r.Context(), namespace, tunnelReq)
        if err != nil {
            http.Error(w, fmt.Sprintf(`{"error":"%s","request_id":"%s"}`, err, requestID), http.StatusBadGateway)
            return
        }
        defer resp.Body.Close()

        for k, v := range resp.Header {
            if len(v) > 0 {
                w.Header().Set(k, v[0])
            }
        }
        w.WriteHeader(resp.StatusCode)
        io.Copy(w, resp.Body)
    })

    http.ListenAndServe(":8080", nil)
}
```

## Step 4: Forward Requests Through the Tunnel

The key function is `DoTunnelRequest` or `DoTunnelRequestByNamespace` on the `Gateway`:

```go
// Route by namespace (string)
resp, requestID, err := gw.DoTunnelRequestByNamespace(ctx, "my-gpu-box", tunnelReq)

// Route by channel ID (int) — if you track channel IDs in your own routing table
resp, requestID, err := gw.DoTunnelRequest(ctx, channelID, tunnelReq)
```

Both return a standard `*http.Response` with streaming body. The response is assembled from tunnel frames:

1. First frame: status code + headers
2. Subsequent frames: body chunks
3. Final frame: EOF marker

You can pipe it directly to the client:

```go
defer resp.Body.Close()
w.WriteHeader(resp.StatusCode)
io.Copy(w, resp.Body) // streaming works out of the box
```

### TunnelRequest Fields

```go
type TunnelRequest struct {
    RequestID string            // auto-generated if empty
    RouteKind string            // e.g. "chat_completions", "embeddings", "images_generations"
    Method    string            // HTTP method
    Path      string            // target path on the worker
    Model     string            // model name (used for routing on worker side)
    Headers   map[string]string // forwarded headers
    IsStream  bool              // whether this is a streaming request
    Body      []byte            // request body
}
```

### Route Kinds

```go
TunnelRouteKindChatCompletions    = "chat_completions"
TunnelRouteKindCompletions        = "completions"
TunnelRouteKindResponses          = "responses"
TunnelRouteKindEmbeddings         = "embeddings"
TunnelRouteKindRerank             = "rerank"
TunnelRouteKindAudioSpeech        = "audio_speech"
TunnelRouteKindAudioTranscription = "audio_transcription"
TunnelRouteKindAudioTranslation   = "audio_translation"
TunnelRouteKindImagesGenerations  = "images_generations"
TunnelRouteKindImagesEdits        = "images_edits"
TunnelRouteKindImagesVariations   = "images_variations"
TunnelRouteKindVideosCreate       = "videos_create"
TunnelRouteKindVideosGet          = "videos_get"
TunnelRouteKindVideosContent      = "videos_content"
```

## Step 5 (Optional): Enable QUIC Transport

If you want QUIC in addition to WebSocket:

```go
closeFn, err := gw.StartQUICGateway(ctx, tokilake.QUICGatewayConfig{
    Enable:   true,
    Port:     "4433",
    CertFile: "/path/to/cert.pem",
    KeyFile:  "/path/to/key.pem",
})
if err != nil {
    log.Fatal(err)
}
defer closeFn()
```

The QUIC listener reuses the same `Gateway` and `SessionManager`. Tokiame clients with `TOKIAME_TRANSPORT_MODE=auto` will automatically try QUIC first and fall back to WebSocket.

## Step 6 (Optional): Cancel In-Flight Requests

If a client disconnects, you can cancel the tunnel request:

```go
ctx, cancel := context.WithCancel(r.Context())
defer cancel()

// The gateway tracks requests internally.
// When the context is cancelled, it sends a cancel_request control message
// to the Tokiame worker and closes the tunnel stream.
resp, requestID, err := gw.DoTunnelRequestByNamespace(ctx, namespace, tunnelReq)
```

You can also cancel by request ID from elsewhere:

```go
session, ok := manager.GetSessionByNamespace("my-worker")
if ok {
    gw.SendCancelRequest(session, requestID, "client_timeout")
}
```

## Minimal Working Example

A complete standalone gateway in ~80 lines is available at [`cmd/tokilake/main.go`](https://github.com/Tokimorphling/Tokilake/blob/main/cmd/tokilake/main.go). It uses in-memory storage and a static token — perfect as a starting point for your own implementation.

```bash
# Run the standalone gateway
go run ./cmd/tokilake -addr :8080 -token sk-test

# In another terminal, connect a Tokiame worker
export TOKIAME_GATEWAY_URL="ws://127.0.0.1:8080/api/tokilake/connect"
export TOKIAME_TOKEN="sk-test"
export TOKIAME_NAMESPACE="local-gpu"
export TOKIAME_MODEL_TARGETS='{"llama3":{"url":"http://127.0.0.1:11434/v1"}}'
go run ./cmd/tokiame

# In a third terminal, send a request
curl http://127.0.0.1:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"llama3","messages":[{"role":"user","content":"hello"}]}'
```

## Architecture Diagram

```
Your Go Application
│
├── main.go
│   └── http.ListenAndServe()
│
├── auth.go
│   └── implements tokilake.Authenticator
│
├── registry.go
│   └── implements tokilake.WorkerRegistry
│       ├── RegisterWorker()    → persist to your DB
│       ├── UpdateHeartbeat()   → update last seen
│       ├── SyncModels()        → update model list
│       └── CleanupWorker()     → mark offline
│
└── go.mod
    └── require tokilake-core

tokilake-core (dependency)
│
├── Gateway
│   ├── HandleGatewayConnection()   ← WebSocket handler calls this
│   ├── DoTunnelRequest()           ← your API handler calls this
│   └── serveControlStream()        ← internal state machine
│
├── SessionManager
│   ├── ClaimNamespace()
│   ├── BindChannel()
│   ├── GetSessionByNamespace()
│   └── Release()
│
└── TunnelSession (smux / QUIC)
    ├── AcceptStream()
    └── OpenStream()
```

## Summary

| What you do | What tokilake-core does |
|---|---|
| Authenticate tokens | Runs auth state machine on control stream |
| Persist worker/channel state | Manages in-memory session maps |
| Decide which namespace to route to | Opens tunnel streams and multiplexes them |
| Forward the response to your client | Frames requests/responses as tunnel JSON |
| Handle billing, logging, rate limits | Handles heartbeat timeout and auto-cleanup |
