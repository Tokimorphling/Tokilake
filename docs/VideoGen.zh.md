---
title: "异步视频生成"
layout: doc
outline: deep
lastUpdated: true
---

# 异步视频生成

Tokilake 对外提供统一的异步视频接口 `/v1/videos`。客户端用同一套 API 调用文生视频和图生视频，Tokiame 会根据本地模型的 `backend_type` 适配不同后端。

## 调用链路

```text
Client
  POST /v1/videos
Tokilake Hub
  鉴权、路由到 Tokiame 渠道、创建任务快照
Tokiame Worker
  根据 backend_type 重写请求
Local Backend
  SGLang、vLLM-Omni 或其他 OpenAI-compatible 视频后端
```

创建接口会立即返回任务对象。客户端随后轮询状态，任务完成后再下载视频内容。

## 后端配置

在 `~/.tokilake/tokiame.json` 的 `model_targets` 中配置视频模型。

### SGLang

SGLang 文生视频使用 JSON。图生视频可使用 JSON `reference_url` 或 multipart `input_reference`，Tokiame 会保持兼容格式。

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

vLLM-Omni 的视频创建接口使用 multipart form。客户端仍然可以向 Tokilake 发送 JSON，Tokiame 会在转发时自动转换成 multipart。

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

vLLM-Omni 支持的别名包括 `vllm_omni`、`vllm-omni`、`vllm`。

## 文生视频

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

返回值是异步任务对象：

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

## 图生视频

图片输入必须且只能使用一种：`reference_url`、`image_url`、`image_b64_json` 或 multipart `input_reference`。

### 图片 URL

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

也可以传 `image_url`。对于 SGLang 目标，Tokiame 会把 `image_url` 映射为 `reference_url`。

### 上传本地图片

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

当后端支持文件上传，或图片无法公开访问时，建议使用 multipart 上传。

## 轮询状态

查询单个任务：

```bash
curl "$TOKILAKE_BASE_URL/v1/videos/video_gen_123" \
  -H "Authorization: Bearer $TOKILAKE_API_KEY"
```

列出当前用户最近的视频任务：

```bash
curl "$TOKILAKE_BASE_URL/v1/videos?limit=20&status=processing" \
  -H "Authorization: Bearer $TOKILAKE_API_KEY"
```

Tokilake 会把任务状态统一为：

```text
submitted, queued, processing, completed, failed
```

后端返回的 `pending`、`running`、`in_progress`、`success`、`succeeded`、`failure`、`cancelled` 等状态会被映射成统一状态。

## 下载结果

任务状态变成 `completed` 后，下载生成视频：

```bash
curl "$TOKILAKE_BASE_URL/v1/videos/video_gen_123/content" \
  -H "Authorization: Bearer $TOKILAKE_API_KEY" \
  --output result.mp4
```

下载接口语义：

- `409 video_not_ready`: 任务尚未完成。
- `502 video_failed`: 上游任务失败。
- `200`: Tokilake 通过 Tokiame 流式返回视频二进制内容。

## Python 示例

仓库内提供了一个简单辅助脚本：`scripts/async_video.py`。如果当前 Python 环境没有 `requests`，先安装依赖：

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

使用图片 URL 做图生视频：

```bash
python scripts/async_video.py \
  --base-url "$TOKILAKE_BASE_URL" \
  --api-key "$TOKILAKE_API_KEY" \
  --model wan-sglang \
  --image-url "https://example.com/input.png" \
  --prompt "Animate this image with gentle camera movement." \
  --out i2v-url.mp4
```

上传本地图片做图生视频：

```bash
python scripts/async_video.py \
  --base-url "$TOKILAKE_BASE_URL" \
  --api-key "$TOKILAKE_API_KEY" \
  --model wan-vllm \
  --input-reference input.png \
  --prompt "Animate this frame." \
  --out i2v-file.mp4
```

## 后端契约

自定义后端应提供以下接口：

- `POST /v1/videos`
- `GET /v1/videos/{id}`
- `GET /v1/videos/{id}/content`

JSON 响应至少应包含任务 ID 和状态。Tokiame response adapter 会归一化常见字段：

- `task_id` 或 `video_id` -> `id`
- `created_at`、`createdAt` 或 `submit_time` -> `created`
- `task_status` 或 `state` -> `status`
- `data[0].url`、`video_url`、`url` -> `download_url` / `content_url`
- `fail_reason`、`error_message`、`message` -> `error.message`

如果某个后端响应结构完全不同，可以为它的 `backend_type` 添加新的 Tokiame response adapter。
