# Image Generation via Chat Completions (OpenAI SDK)

<p align="right">
  <strong>English</strong> | <a href="./ImageGenChat.zh.md">中文</a>
</p>

This tutorial demonstrates how to serve a **diffusion-based image generation backend** (e.g. `sglang-diffusion`, `vllm-omni`, or any OpenAI-compatible image model) behind Tokilake, and call it from anywhere using the **standard OpenAI Python SDK** — through the familiar `/v1/chat/completions` endpoint.

Unlike the [Image Generation Guide](./ImageGen.md) which uses the dedicated `/v1/images/generations` endpoint, this approach treats the image model as a regular chat model. The diffusion-specific parameters (`height`, `width`, `num_inference_steps`, `seed`, etc.) are passed via `extra_body`, and Tokilake transparently forwards them to the backend.

> **Prerequisites:** You should have a running Tokilake hub and a registered account with a private group and token. If not, follow the [User Guide](./guide.en.md) first.

---

## Architecture Overview

```
┌──────────────────────┐
│   Your Python App    │
│  (OpenAI SDK)        │
│                      │
│  client.chat.comple- │
│  tions.create(       │
│    model="Z-Image",  │
│    messages=[...],   │
│    extra_body={...}  │
│  )                   │
└─────────┬────────────┘
          │  POST /v1/chat/completions
          ▼
┌──────────────────────┐
│  Tokilake Gateway    │
│                      │
│  • Auth (token)      │
│  • Route to channel  │
│  • Forward extra_body│
└─────────┬────────────┘
          │  (WebSocket/QUIC tunnel)
          ▼
┌──────────────────────┐
│  Tokiame (Worker)    │
│                      │
│  • Reverse tunnel    │
│  • Map model name    │
└─────────┬────────────┘
          │  POST /v1/chat/completions
          ▼
┌──────────────────────┐
│  Image Backend       │
│  (SGLang / vLLM /    │
│   ComfyUI / etc.)    │
│                      │
│  Diffusion model on  │
│  local GPU           │
└──────────────────────┘
```

---

## 1. Start an Image Generation Backend

You need a diffusion model server that exposes an **OpenAI-compatible `/v1/chat/completions`** endpoint. Popular options include:

| Backend | Description |
|---------|-------------|
| **SGLang** (`sglang serve`) | High-performance serving with RadixAttention. Supports many diffusion models. |
| **vLLM-Omni** | vLLM fork with multimodal generation support. |
| **ComfyUI** + OpenAI wrapper | Flexible node-based workflow with an OpenAI-compatible API layer. |

### Example: SGLang

```bash
sglang serve \
  --model-path Wan-AI/Wan2.1-T2V-14B \
  --host 0.0.0.0 \
  --port 8122
```

### Example: Ollama (for multimodal LLMs that can generate images)

```bash
ollama run llava
```

> **Note:** The exact model and parameters depend on your backend. The key requirement is that it accepts a prompt via `messages` and returns image data in the response.

---

## 2. Test the Backend Locally

Before connecting through Tokilake, verify your backend works directly.

```python
from openai import OpenAI
import base64

client = OpenAI(base_url="http://127.0.0.1:8122/v1", api_key="x")

prompt = (
    "Tilt POV shot of a hand holding a surreal popsicle with a transparent "
    "blue exterior, revealing an underwater scene inside: a tiny scuba diver "
    "with tiny fish floating with bubbles, ocean waves crashing, and a green "
    "popsicle stick running through the center. The popsicle is melting slightly, "
    "with a wooden stick at the bottom, hand is holding it by the wooden stick, "
    "soft focus new york street background, premium product photography"
)

print("Generating image...")
response = client.chat.completions.create(
    model="your-model-name",
    messages=[{"role": "user", "content": prompt}],
    extra_body={
        "height": 1024,
        "width": 1024,
        "num_inference_steps": 50,
        "true_cfg_scale": 4.0,
        "seed": 42,
    },
)

# Extract and save the image
message_content = response.choices[0].message.content
if isinstance(message_content, list):
    img_url = message_content[0].get("image_url", {}).get("url", "")
elif isinstance(message_content, str):
    img_url = message_content

if img_url and img_url.startswith("data:image"):
    _, b64_data = img_url.split(",", 1)
    with open("output.png", "wb") as f:
        f.write(base64.b64decode(b64_data))
    print("Saved: output.png")
else:
    print("Response:", message_content[:200])
```

If you see `output.png` within seconds, your backend is ready.

---

## 3. Connect the Backend to Tokilake with Tokiame

### 3.1 Install Tokiame

From the source checkout:

```bash
go install ./cmd/tokiame
```

Or via npm (for published releases):

```bash
npm install -g @tokilake/tokiame
```

### 3.2 Configure Tokiame

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
    "Z-Image-Turbo": {
      "mapped_name": "your-backend-model-name",
      "url": "http://127.0.0.1:8122/v1",
      "api_keys": ["x"],
      "price": {}
    }
  }
}
```

**Key fields:**

| Field | Description |
|-------|-------------|
| `model_targets` key (`Z-Image-Turbo`) | The model name you will use in your API calls through Tokilake. |
| `mapped_name` | The actual model name on your backend (forwarded as `model` in the request). |
| `url` | The local backend's base URL. |
| `api_keys` | API keys for the backend (use `["x"]` if no auth is needed). |

### 3.3 Start Tokiame

```bash
tokiame
```

Upon successful connection, you will see:

```
worker connected group=... models=[Z-Image-Turbo] backend_type=openai
```

### 3.4 Enable `AllowExtraBody` on the Channel

This is **critical** for passing diffusion parameters (`height`, `width`, `num_inference_steps`, etc.) through to the backend.

1. Go to the Tokilake dashboard → **Channel** page.
2. Find the channel that Tokiame auto-created (type `100`).
3. Edit the channel and **enable `Allow Extra Body`** (or `allow_extra_body`).
4. Save.

Without this, Tokilake will strip the extra parameters and the backend will use its defaults.

---

## 4. Call from Anywhere Using the OpenAI Python SDK

Now you can generate images from any machine with internet access, using the standard OpenAI SDK.

```python
from openai import OpenAI
import base64

# Configuration
API_KEY = "sk-YOUR_TOKILAKE_TOKEN"
BASE_URL = "https://YOUR_TOKILAKE_HOST/v1"

PROMPT = (
    "Tilt POV shot of a hand holding a surreal popsicle with a transparent "
    "blue exterior, revealing an underwater scene inside: a tiny scuba diver "
    "with tiny fish floating with bubbles, ocean waves crashing, and a green "
    "popsicle stick running through the center. The popsicle is melting slightly, "
    "with a wooden stick at the bottom, hand is holding it by the wooden stick, "
    "soft focus new york street background, premium product photography"
)

client = OpenAI(base_url=BASE_URL, api_key=API_KEY)

print("Generating image...")
response = client.chat.completions.create(
    model="Z-Image-Turbo",
    messages=[{"role": "user", "content": PROMPT}],
    extra_body={
        "height": 1024,
        "width": 1024,
        "num_inference_steps": 50,
        "true_cfg_scale": 4.0,
        "seed": 42,
    },
)


def extract_and_save_image(response, filename="output.png"):
    try:
        message_content = response.choices[0].message.content

        img_url = None
        if isinstance(message_content, list) and len(message_content) > 0:
            item = message_content[0]
            if isinstance(item, dict):
                img_url = item.get("image_url", {}).get("url")
            else:
                img_url = getattr(getattr(item, "image_url", None), "url", None)
        elif isinstance(message_content, str):
            img_url = message_content

        if not img_url:
            print("Error: Could not find image URL in response.")
            return

        if img_url.startswith("data:image"):
            _, b64_data = img_url.split(",", 1)
            with open(filename, "wb") as f:
                f.write(base64.b64decode(b64_data))
            print(f"Success: Image saved to {filename}")
        else:
            print(f"Image URL found (not base64): {img_url}")

    except Exception as e:
        print(f"Error during extraction: {e}")


extract_and_save_image(response)
```

You should see `output.png` saved locally — the same image you'd get from calling the backend directly.

Below is an example output generated by **Z-Image-Turbo** running on **Google Colab (A100 GPU)**, called through Tokilake using the code above:

![Example output — Z-Image-Turbo on Colab](./ImageGenChat-output.png)

---

## Common `extra_body` Parameters

The exact parameters depend on your backend and model. Here are the most common ones:

| Parameter | Type | Description |
|-----------|------|-------------|
| `height` | int | Output image height in pixels (e.g. `1024`) |
| `width` | int | Output image width in pixels (e.g. `1024`) |
| `num_inference_steps` | int | Number of denoising steps. Higher = better quality, slower. |
| `true_cfg_scale` | float | Classifier-free guidance scale. Controls prompt adherence. |
| `guidance_scale` | float | Alternative name for CFG scale (some backends use this). |
| `seed` | int | Random seed for reproducibility. |
| `negative_prompt` | str | Things to avoid in the generated image. |

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `extra_body` params are ignored | `AllowExtraBody` not enabled on channel | Enable it in Channel settings (Step 3.4) |
| `worker connected` but model not found | Model name mismatch | Ensure `model_targets` key matches the `model` you pass in API calls |
| Image returns as URL instead of base64 | Backend returns URL format | Handle both `data:image` base64 and plain URL responses |
| Connection timeout | Backend not reachable from Tokiame | Check that the `url` in `model_targets` is accessible from where Tokiame runs |
| Token auth fails | Token not bound to the correct group | Verify token is linked to the group that the Tokiame worker joined |

---

## See Also

- [User Guide (English)](./guide.en.md) — Full setup walkthrough
- [中文使用指南](./guide.zh.md) — Chinese setup guide
- [Image Generation Guide](./ImageGen.md) — Using the dedicated `/v1/images/generations` endpoint
- [图像生成指南](./ImageGen.zh.md) — 使用专用 `/v1/images/generations` 端点
