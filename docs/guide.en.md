# Tokilake User Guide

Tokilake is a decentralized LLM API gateway that allows GPU worker nodes (Tokiame) residing behind NAT/Intranet to connect to a central hub.

## 1. Deploying Tokilake (Hub)

If you only want to run as a client (Tokiame), you can skip this section.

### Run with Docker

It is recommended to use Docker to run Tokilake:

```bash
docker pull ghcr.io/tokimorphling/tokilake:latest

docker run -d \
  --name "tokilake-test-1" \
  --restart always \
  -p 3001:3000 \
  -e TZ="UTC" \
  -v "$(pwd)/deploy/config.local-test.yaml:/data/config.yaml:ro" \
  -v "data:/data" \
  "ghcr.io/tokimorphling/tokilake:latest"
```

> [!TIP]
> Ensure that the `$(pwd)/deploy/config.local-test.yaml` configuration file has been modified according to your needs.

## 2. Registration and Login

1. Access the Tokilake dashboard (default is `http://localhost:3001` if running locally).
2. In the dashboard, you will see **Private Groups**, which is the core mechanism of Tokilake.
3. Click the **Create Group** button to create a private group.
4. In the **Actions/Manage Group** page, you can manage invitations and group members.
5. If you have GPUs and are serving models, you can click **Generate invite code** to share access.
6. Create a token in the **Token** page and bind it to the private group you created. **Keep this token safe.**

## 3. Deploying Tokiame (Worker)

You can deploy Tokiame in environments behind NAT, residential networks, or even Google Colab, as long as you have GPU resources available.

### 3.1 Start Inference Service

Using `llama.cpp` as an example:

```bash
export LLAMA_CACHE="unsloth/Qwen3.5-9B-GGUF"
./llama-server \
    -hf unsloth/Qwen3.5-9B-GGUF:UD-Q4_K_XL \
    --ctx-size 16384 \
    --temp 1.0 \
    --top-p 0.95 \
    --top-k 20 \
    --min-p 0.00 \
    --alias "unsloth/Qwen3.5-9B-GGUF" \
    --port 8001 \
    --chat-template-kwargs '{"enable_thinking":true}'
```

### 3.2 Install and Configure Tokiame

Install `tokiame` (if available via NPM):
```bash
npm install -g @tokilake/tokiame
```

Modify the configuration file `~/.tokilake/tokiame.json`:

```json
{
    "gateway_url": "wss://YOUR_TOKILAKE_IP/api/tokilake/connect",
    "token": "YOUR_TOKEN",
    "namespace": "gpu-01",
    "node_name": "node-1",
    "group": "YOUR_GROUP_NAME",
    "backend_type": "openai",
    "heartbeat_interval_seconds": 15,
    "reconnect_delay_seconds": 5,
    "model_targets": {
        "unsloth_Qwen3.5-9B-GGUF_Qwen3.5-9B-UD-Q4_K_XL": {
            "mapped_name": "unsloth/Qwen3.5-9B-GGUF",
            "url": "http://127.0.0.1:8001/v1",
            "api_keys": ["x"],
            "price": {}
        }
    }
}
```

### 3.3 Start Tokiame

Run the `tokiame` client. Upon successful connection, you will see:
`worker connected group=... models=[...]`

![alt text](image.png)

## 4. Sharing GPUs with Friends

1. Have your friend register for a Tokilake account.
2. On the **Private Groups** page, they can use **Redeem Invite Code**.
3. Once joined, they will be able to see and use your models.
4. They should create an **API Key** in the **Token** page and bind it to the group.
5. They can test the model directly in the **Actions/Chat** section of the dashboard.
![alt text](image-1.png)
**Enjoy!**
