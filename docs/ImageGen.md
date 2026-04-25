<p align="right">
  <strong>English</strong> | <a href="./ImageGen.zh.md">中文</a>
</p>

# Image Generation with Tokilake

This guide shows how to serve an OpenAI-compatible image generation backend (e.g. `sglang-diffusion` or `vllm-omni`) and connect it to the Tokilake gateway through **Tokiame**.

If you haven’t set up Tokilake/Tokiame yet, start with:

- [User Guide (English)](./guide.en.md)
- [中文使用指南](./guide.zh.md)

## 1) Start an image backend (SGLang example)

```bash
sglang serve \
  --model-path Qwen/Qwen-Image \
  --host 0.0.0.0 \
  --port 8122
```

There are many flags and environment variables for performance tuning; here we only keep the essentials.

## 2) Test the backend locally

Send a request to `POST /v1/images/generations` and save the returned image.

```python
import base64
import os

import requests

BACKEND_BASE_URL = "http://127.0.0.1:8122"
url = f"{BACKEND_BASE_URL}/v1/images/generations"

payload = {
    "model": "Qwen/Qwen-Image",
    "prompt": "A happy 20-year-old Asian girl.",
    "size": "1024x1024",
    "seed": 10240,
    "num_inference_steps": 30,
    "guidance_scale": 4.0,
    "negative_prompt": "blurry, low quality, distorted, bad anatomy",
    "output_format": "png",
    "n": 1,
    "response_format": "b64_json",
}

headers = {
    # Remove this header if your backend does not require authentication.
    "Authorization": "Bearer x",
}

resp = requests.post(url, json=payload, headers=headers, timeout=300)
resp.raise_for_status()
data = resp.json()

image_b64 = data["data"][0]["b64_json"]
image_bytes = base64.b64decode(image_b64)
output_path = os.path.join(os.path.dirname(__file__), "generated.png")
with open(output_path, "wb") as output_file:
    output_file.write(image_bytes)

print("Saved:", output_path)
```

If `generated.png` shows up within a few seconds, your backend is working.

## 3) Connect the backend to Tokilake with Tokiame

Install `tokiame` from this source checkout:

```bash
go install ./cmd/tokiame
```

If you are using a published release, the npm installer is also available:

```bash
npm i -g @tokilake/tokiame
```

Edit `~/.tokilake/tokiame.json`:

```json
{
  "gateway_url": "wss://YOUR_TOKILAKE_HOST/api/tokilake/connect",
  "token": "YOUR_TOKILAKE_TOKEN",
  "namespace": "gpu-01",
  "node_name": "node-1",
  "group": "YOUR_GROUP_NAME",
  "backend_type": "openai",
  "heartbeat_interval_seconds": 15,
  "reconnect_delay_seconds": 5,
  "model_targets": {
    "Qwen-Image": {
      "mapped_name": "Qwen/Qwen-Image",
      "url": "http://127.0.0.1:8122/v1",
      "api_keys": ["x"],
      "price": {}
    }
  }
}
```

Notes:

- The key under `model_targets` (here: `Qwen-Image`) is the **model name you will call through Tokilake**.
- `mapped_name` is the backend model name (forwarded to the backend as `model`).

Start Tokiame:

```bash
tokiame
```

Once connected, you should see a log like:

```text
worker connected group=... models=[Qwen-Image] backend_type=openai
```

## 4) Call Tokilake from anywhere

Now you can call Tokilake’s `POST /v1/images/generations` endpoint with your token:

```python
import base64
import os

import requests

TOKILAKE_BASE_URL = "https://YOUR_TOKILAKE_HOST"
TOKILAKE_TOKEN = "YOUR_TOKILAKE_TOKEN"

url = f"{TOKILAKE_BASE_URL.rstrip('/')}/v1/images/generations"
payload = {
    "model": "Qwen-Image",
    "prompt": "A happy 20-year-old Asian girl.",
    "size": "1024x1024",
    "seed": 10240,
    "num_inference_steps": 30,
    "guidance_scale": 4.0,
    "negative_prompt": "blurry, low quality, distorted, bad anatomy",
    "output_format": "png",
    "n": 1,
    "response_format": "b64_json",
}
headers = {"Authorization": f"Bearer {TOKILAKE_TOKEN}"}

resp = requests.post(url, json=payload, headers=headers, timeout=300)
resp.raise_for_status()
data = resp.json()

image_b64 = data["data"][0]["b64_json"]
image_bytes = base64.b64decode(image_b64)
output_path = os.path.join(os.path.dirname(__file__), "generated.png")
with open(output_path, "wb") as output_file:
    output_file.write(image_bytes)

print("Saved:", output_path)
```
