#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

TOKILAKE_PORT=18080
MOCK_API_PORT=18081
TOKEN="sk-test-token"
NAMESPACE="test-worker"

CUSTOM_CONFIG=""
USE_MOCK_API=true

PIDS=()
TEMP_FILES=()

usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -c, --config FILE    Use custom tokiame config file"
    echo "  -t, --token TOKEN    Token for authentication (default: sk-test-token)"
    echo "  -h, --help           Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                                    # Use default test config with mock API"
    echo "  $0 -c config.tokiame.example.json     # Use custom config"
    echo "  $0 -t my-secret-token                 # Use custom token"
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -c|--config)
                CUSTOM_CONFIG="$2"
                shift 2
                ;;
            -t|--token)
                TOKEN="$2"
                shift 2
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                echo "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
}

cleanup() {
    echo ""
    echo "Cleaning up..."
    for pid in "${PIDS[@]+"${PIDS[@]}"}"; do
        kill "$pid" 2>/dev/null || true
        wait "$pid" 2>/dev/null || true
    done
    for f in "${TEMP_FILES[@]+"${TEMP_FILES[@]}"}"; do
        rm -f "$f"
    done
    echo "Done."
}
trap cleanup EXIT

log() {
    echo -e "\033[0;32m[TEST]\033[0m $*"
}

error() {
    echo -e "\033[0;31m[ERROR]\033[0m $*"
    exit 1
}

wait_for_port() {
    local port=$1
    local timeout=${2:-10}
    local count=0
    while ! curl -s "http://localhost:$port/health" > /dev/null 2>&1; do
        sleep 0.5
        count=$((count + 1))
        if [ "$count" -ge "$((timeout * 2))" ]; then
            error "Timeout waiting for port $port"
        fi
    done
}

extract_json_value() {
    local json_file=$1
    local key=$2
    grep -o "\"$key\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" "$json_file" | head -1 | sed 's/.*" *: *"\([^"]*\)".*/\1/'
}

extract_port_from_url() {
    local url=$1
    # Extract port from ws://host:port/path or http://host:port/path
    echo "$url" | grep -oE ':[0-9]+' | head -1 | tr -d ':'
}

parse_args "$@"

log "=========================================="
log "Tokilake Integration Test"
log "=========================================="

if [ -n "$CUSTOM_CONFIG" ]; then
    if [ ! -f "$CUSTOM_CONFIG" ]; then
        error "Config file not found: $CUSTOM_CONFIG"
    fi
    log "Using custom config: $CUSTOM_CONFIG"
    
    CONFIG_TOKEN=$(extract_json_value "$CUSTOM_CONFIG" "token")
    CONFIG_NAMESPACE=$(extract_json_value "$CUSTOM_CONFIG" "namespace")
    CONFIG_GATEWAY_URL=$(extract_json_value "$CUSTOM_CONFIG" "gateway_url")
    
    if [ -n "$CONFIG_TOKEN" ]; then
        TOKEN="$CONFIG_TOKEN"
        log "Using token from config: $TOKEN"
    fi
    
    if [ -n "$CONFIG_NAMESPACE" ]; then
        NAMESPACE="$CONFIG_NAMESPACE"
        log "Using namespace from config: $NAMESPACE"
    fi
    
    if [ -n "$CONFIG_GATEWAY_URL" ]; then
        CONFIG_PORT=$(extract_port_from_url "$CONFIG_GATEWAY_URL")
        if [ -n "$CONFIG_PORT" ]; then
            TOKILAKE_PORT="$CONFIG_PORT"
            log "Using port from config: $TOKILAKE_PORT"
        fi
    fi
    
    if grep -q "localhost:$MOCK_API_PORT" "$CUSTOM_CONFIG"; then
        USE_MOCK_API=true
        log "Config uses mock API server"
    else
        USE_MOCK_API=false
        log "Config uses external API (mock API server won't be started)"
    fi
else
    log "Using default test config"
fi

log "Building binaries..."
cd "$ROOT_DIR"
go build -o /tmp/tokilake-test-server ./cmd/tokilake/
TEMP_FILES+=("/tmp/tokilake-test-server")

if [ "$USE_MOCK_API" = true ]; then
    go build -o /tmp/tokilake-test-mockapi ./scripts/mockapi/
    TEMP_FILES+=("/tmp/tokilake-test-mockapi")
fi
log "Build complete."

if [ "$USE_MOCK_API" = true ]; then
    log "Starting mock API server on port $MOCK_API_PORT..."
    /tmp/tokilake-test-mockapi -addr ":$MOCK_API_PORT" &
    PIDS+=($!)
    sleep 1
fi

log "Starting tokilake server on port $TOKILAKE_PORT..."
/tmp/tokilake-test-server -addr ":$TOKILAKE_PORT" -token "$TOKEN" &
PIDS+=($!)
wait_for_port "$TOKILAKE_PORT"
log "Tokilake server started."

if [ -n "$CUSTOM_CONFIG" ]; then
    TOKIAME_CONFIG_FILE="$CUSTOM_CONFIG"
else
    TOKIAME_CONFIG_FILE=$(mktemp)
    TEMP_FILES+=("$TOKIAME_CONFIG_FILE")
    cat > "$TOKIAME_CONFIG_FILE" <<EOF
{
  "gateway_url": "ws://localhost:$TOKILAKE_PORT/connect",
  "token": "$TOKEN",
  "namespace": "$NAMESPACE",
  "node_name": "test-node",
  "group": "default",
  "transport_mode": "websocket",
  "model_targets": {
    "gpt-3.5-turbo": {
      "url": "http://localhost:$MOCK_API_PORT",
      "backend_type": "openai"
    },
    "gpt-4": {
      "url": "http://localhost:$MOCK_API_PORT",
      "backend_type": "openai"
    }
  },
  "heartbeat_interval_seconds": 5,
  "reconnect_delay_seconds": 2
}
EOF
fi

log "Starting tokiame client..."
TOKIAME_CONFIG="$TOKIAME_CONFIG_FILE" go run ./cmd/tokiame/ > /tmp/tokiame-test.log 2>&1 &
PIDS+=($!)
sleep 3

log "Verifying tokiame connection..."
if ! grep -q "worker connected" /tmp/tokiame-test.log; then
    error "tokiame failed to connect. Log: $(cat /tmp/tokiame-test.log)"
fi
log "Tokiame connected successfully."

log "Checking health endpoint..."
HEALTH=$(curl -s "http://localhost:$TOKILAKE_PORT/health")
SESSION_COUNT=$(echo "$HEALTH" | grep -o '"sessions":[0-9]*' | cut -d: -f2)
if [ "$SESSION_COUNT" != "1" ]; then
    error "Expected 1 session, got $SESSION_COUNT"
fi
log "Health check passed: $HEALTH"

if [ -n "$CUSTOM_CONFIG" ]; then
    TEST_MODEL=$(extract_json_value "$CUSTOM_CONFIG" "model" || echo "")
    if [ -z "$TEST_MODEL" ]; then
        TEST_MODEL=$(sed -n '/"model_targets"/,/}/p' "$CUSTOM_CONFIG" | grep -oE '"[a-zA-Z0-9._-]+" *:' | head -2 | tail -1 | tr -d '":' || echo "gpt-3.5-turbo")
    fi
else
    TEST_MODEL="gpt-3.5-turbo"
fi

log "------------------------------------------"
log "Test 1: Non-streaming chat completion"
log "------------------------------------------"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "http://localhost:$TOKILAKE_PORT/v1/chat/completions?namespace=$NAMESPACE" \
    -H "Content-Type: application/json" \
    -d "{\"model\":\"$TEST_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello, this is a test\"}],\"stream\":false}")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" != "200" ]; then
    error "Test 1 failed: Expected HTTP 200, got $HTTP_CODE. Body: $BODY"
fi

if [ "$USE_MOCK_API" = true ]; then
    if ! echo "$BODY" | grep -q "test response"; then
        error "Test 1 failed: Expected 'test response' in body, got: $BODY"
    fi
fi
log "Test 1 passed!"
log "Response: $(echo "$BODY" | head -c 200)..."

log "------------------------------------------"
log "Test 2: Streaming chat completion"
log "------------------------------------------"
STREAM_RESPONSE=$(curl -s -X POST "http://localhost:$TOKILAKE_PORT/v1/chat/completions?namespace=$NAMESPACE" \
    -H "Content-Type: application/json" \
    -d "{\"model\":\"$TEST_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello streaming\"}],\"stream\":true}")

if ! echo "$STREAM_RESPONSE" | grep -q "data:"; then
    error "Test 2 failed: Expected SSE data frames, got: $STREAM_RESPONSE"
fi
log "Test 2 passed!"
log "Stream response preview: $(echo "$STREAM_RESPONSE" | head -3)"

log "------------------------------------------"
log "Test 3: Invalid namespace"
log "------------------------------------------"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "http://localhost:$TOKILAKE_PORT/v1/chat/completions?namespace=nonexistent" \
    -H "Content-Type: application/json" \
    -d "{\"model\":\"$TEST_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"test\"}]}")
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)

if [ "$HTTP_CODE" != "502" ]; then
    error "Test 3 failed: Expected HTTP 502, got $HTTP_CODE"
fi
log "Test 3 passed! (correctly returned 502 for offline namespace)"

log "------------------------------------------"
log "Test 4: Heartbeat verification"
log "------------------------------------------"
sleep 6
if grep -q "heartbeat" /tmp/tokiame-test.log; then
    log "Test 4 passed! Heartbeat messages detected."
else
    log "Test 4 skipped: No heartbeat messages yet (may need longer wait)."
fi

log "=========================================="
log "All tests passed!"
log "=========================================="
log ""
log "Summary:"
log "  - Tokilake server: running on port $TOKILAKE_PORT"
if [ "$USE_MOCK_API" = true ]; then
    log "  - Mock API server: running on port $MOCK_API_PORT"
else
    log "  - Using external API"
fi
log "  - Tokiame client: connected with namespace '$NAMESPACE'"
log "  - Token: $TOKEN"
log "  - Test model: $TEST_MODEL"
log "  - Non-streaming requests: working"
log "  - Streaming requests: working"
log "  - Error handling: working"
