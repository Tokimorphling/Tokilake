# sglang / vLLM Diffusion Mock

This mock server is a small Python implementation for local testing.

It simulates a shared OpenAI-compatible diffusion subset used by `sglang` and `vLLM`:

- `GET /health`
- `GET /ready`
- `GET /models`
- `GET /v1/models`
- `POST /v1/images/generations`

It also adds a small async video task subset for video workflow testing:

- `POST /v1/videos`
- `GET /v1/videos`
- `GET /v1/videos/{id}`
- `GET /v1/videos/{id}/content`

## Run

```bash
python3 /Users/asuka/codes/Tokilake/mocks/sglang-vllm-diffusion/server.py
```

Optional flags:

```bash
python3 /Users/asuka/codes/Tokilake/mocks/sglang-vllm-diffusion/server.py \
  --host 0.0.0.0 \
  --port 30010 \
  --image-model Qwen/Qwen-Image \
  --video-model THUDM/CogVideoX-5b
```

Environment variables are also supported:

- `MOCK_DIFFUSION_HOST`
- `MOCK_DIFFUSION_PORT`
- `MOCK_DIFFUSION_BASE_URL`
- `MOCK_DIFFUSION_IMAGE_MODEL`
- `MOCK_DIFFUSION_VIDEO_MODEL`
- `MOCK_DIFFUSION_IMAGE_DELAY_MS`
- `MOCK_DIFFUSION_VIDEO_QUEUE_SECONDS`
- `MOCK_DIFFUSION_VIDEO_PROCESS_SECONDS`
- `MOCK_DIFFUSION_BEARER_TOKEN`

## Example: models

```bash
curl http://127.0.0.1:30010/v1/models
```

## Example: image generation

```bash
curl http://127.0.0.1:30010/v1/images/generations \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "Qwen/Qwen-Image",
    "prompt": "a cinematic robot walking through rain",
    "n": 1,
    "response_format": "b64_json"
  }'
```

Use `response_format=url` to make the response point at a downloadable PNG served by the mock.

## Example: async video flow

Create:

```bash
curl http://127.0.0.1:30010/v1/videos \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "THUDM/CogVideoX-5b",
    "prompt": "a drone shot above a neon city",
    "duration": 5,
    "fps": 24,
    "size": "1280x720"
  }'
```

Query:

```bash
curl http://127.0.0.1:30010/v1/videos
curl http://127.0.0.1:30010/v1/videos/<video_id>
curl -OJ http://127.0.0.1:30010/v1/videos/<video_id>/content
```

The mock keeps jobs in memory only. Video status moves from `queued` to `processing` to `completed` based on time.

## Failure injection

Image request failure:

- Header: `X-Mock-Fail: 1`
- Prompt marker: `[mock:fail]`

Video request failure:

- Header: `X-Mock-Video-Fail: 1`
- Prompt marker: `[mock:fail]`

Failures are returned as OpenAI-style JSON errors.
