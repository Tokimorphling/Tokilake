---
title: "Async Video Generation"
layout: doc
outline: deep
lastUpdated: true
---

# Async Video Generation

Tokilake exposes one async video API at `/v1/videos`. Clients call the same Tokilake API for text-to-video and image-to-video. Tokiame adapts the request and response for each local backend.

## Flow

```text
Client
  POST /v1/videos
Tokilake Hub
  Auth, route to a Tokiame channel, create a task snapshot
Tokiame Worker
  Rewrite request for the configured backend_type
Local Backend
  SGLang, vLLM-Omni, or another OpenAI-compatible video backend
```

The create call returns a task object immediately. The client then polls status and downloads the video when the task is complete.

## Backend Configuration

Configure video models in `~/.tokilake/tokiame.json` under `model_targets`.

### SGLang

SGLang text-to-video accepts JSON. For image-to-video, Tokilake supports JSON `reference_url` and multipart `input_reference`; Tokiame keeps the format compatible with SGLang.

```json
{
  "gateway_url": "wss://YOUR_TOKILAKE_HOST/api/tokilake/connect",
  "token": "sk-your-worker-token",
  "namespace": "gpu-video-01",
  "group": "your-private-group",
  "backend_type": "sglang",
  "model_targets": {
    "wan-sglang": {
      "backend_type": "sglang",
      "url": "http://127.0.0.1:30010/v1",
      "mapped_name": "Wan-AI/Wan2.1-T2V"
    }
  }
}
```

### vLLM-Omni

vLLM-Omni video creation expects multipart form requests. Clients can still send JSON to Tokilake; Tokiame converts video create requests to multipart automatically.

```json
{
  "gateway_url": "wss://YOUR_TOKILAKE_HOST/api/tokilake/connect",
  "token": "sk-your-worker-token",
  "namespace": "gpu-video-01",
  "group": "your-private-group",
  "backend_type": "openai",
  "model_targets": {
    "wan-vllm": {
      "backend_type": "vllm_omni",
      "url": "http://127.0.0.1:8000/v1",
      "mapped_name": "Wan-AI/Wan2.2-TI2V-5B-Diffusers"
    }
  }
}
```

Accepted aliases for vLLM-Omni are `vllm_omni`, `vllm-omni`, and `vllm`.

## Text-to-Video

```bash
curl "$TOKILAKE_BASE_URL/v1/videos" \
  -H "Authorization: Bearer $TOKILAKE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "wan-vllm",
    "mode": "text2video",
    "prompt": "A red sports car driving along a coast road at sunset.",
    "size": "832x480",
    "num_frames": 33,
    "fps": 16,
    "num_inference_steps": 40,
    "guidance_scale": 4.0,
    "seed": 42
  }'
```

The response is an async video task:

```json
{
  "id": "video_gen_123",
  "object": "video",
  "created": 1710000000,
  "model": "wan-vllm",
  "mode": "text2video",
  "status": "queued",
  "content_url": "/v1/videos/video_gen_123/content",
  "download_url": "/v1/videos/video_gen_123/content"
}
```

## Image-to-Video

Use exactly one image input: `reference_url`, `image_url`, `image_b64_json`, or multipart `input_reference`.

### Image URL

```bash
curl "$TOKILAKE_BASE_URL/v1/videos" \
  -H "Authorization: Bearer $TOKILAKE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "wan-sglang",
    "mode": "image2video",
    "prompt": "Animate this frame with a slow cinematic camera move.",
    "reference_url": "https://example.com/input.png",
    "size": "1280x720",
    "num_frames": 33,
    "fps": 16
  }'
```

`image_url` is also accepted. For SGLang targets, Tokiame maps `image_url` to `reference_url`.

### Local Image Upload

```bash
curl "$TOKILAKE_BASE_URL/v1/videos" \
  -H "Authorization: Bearer $TOKILAKE_API_KEY" \
  -F "model=wan-vllm" \
  -F "mode=image2video" \
  -F "prompt=Animate this frame with a slow cinematic camera move." \
  -F "size=1280x720" \
  -F "num_frames=33" \
  -F "fps=16" \
  -F "input_reference=@input.png"
```

Multipart is useful when the backend supports file upload or when the image is not publicly reachable.

## Polling

Query a single task:

```bash
curl "$TOKILAKE_BASE_URL/v1/videos/video_gen_123" \
  -H "Authorization: Bearer $TOKILAKE_API_KEY"
```

List recent tasks for the current user:

```bash
curl "$TOKILAKE_BASE_URL/v1/videos?limit=20&status=processing" \
  -H "Authorization: Bearer $TOKILAKE_API_KEY"
```

Tokilake normalizes task status to:

```text
submitted, queued, processing, completed, failed
```

Backend statuses such as `pending`, `running`, `in_progress`, `success`, `succeeded`, `failure`, and `cancelled` are mapped to the normalized states.

## Download

After the task status becomes `completed`, download the generated video:

```bash
curl "$TOKILAKE_BASE_URL/v1/videos/video_gen_123/content" \
  -H "Authorization: Bearer $TOKILAKE_API_KEY" \
  --output result.mp4
```

Download semantics:

- `409 video_not_ready`: the task has not completed.
- `502 video_failed`: the task failed upstream.
- `200`: Tokilake streams the video bytes through Tokiame.

## Python Example

The repository includes a simple helper script at `scripts/async_video.py`. Install `requests` if your Python environment does not already have it:

```bash
python -m pip install requests
```

```bash
python scripts/async_video.py \
  --base-url "$TOKILAKE_BASE_URL" \
  --api-key "$TOKILAKE_API_KEY" \
  --model wan-vllm \
  --out result.mp4
```

Image-to-video with a URL:

```bash
python scripts/async_video.py \
  --base-url "$TOKILAKE_BASE_URL" \
  --api-key "$TOKILAKE_API_KEY" \
  --model wan-sglang \
  --image-url "https://example.com/input.png" \
  --prompt "Animate this image with gentle camera movement." \
  --out i2v-url.mp4
```

Image-to-video with a local file:

```bash
python scripts/async_video.py \
  --base-url "$TOKILAKE_BASE_URL" \
  --api-key "$TOKILAKE_API_KEY" \
  --model wan-vllm \
  --input-reference input.png \
  --prompt "Animate this frame." \
  --out i2v-file.mp4
```

## Backend Contract

A custom backend should expose these endpoints:

- `POST /v1/videos`
- `GET /v1/videos/{id}`
- `GET /v1/videos/{id}/content`

The JSON response should contain a task identifier and status. Tokiame response adapters normalize common fields:

- `task_id` or `video_id` -> `id`
- `created_at`, `createdAt`, or `submit_time` -> `created`
- `task_status` or `state` -> `status`
- `data[0].url`, `video_url`, `url` -> `download_url` / `content_url`
- `fail_reason`, `error_message`, `message` -> `error.message`

If a backend has a different response shape, add a Tokiame response adapter for its `backend_type`.
