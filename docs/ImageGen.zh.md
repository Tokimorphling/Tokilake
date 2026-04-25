<p align="right">
  <a href="./ImageGen.md">English</a> | <strong>中文</strong>
</p>

# 使用 Tokilake 进行图像生成

本文档演示如何启动一个 **OpenAI 兼容**的图像生成后端（例如 `sglang-diffusion` 或 `vllm-omni`），并通过 **Tokiame** 将其接入 Tokilake 网关，从而在 Tokilake 侧以标准的 `POST /v1/images/generations` 形式对外提供服务。

如果你还没有完成 Tokilake/Tokiame 的基础部署与账号/分组配置，建议先阅读：

- [中文使用指南](./guide.zh.md)
- [User Guide (English)](./guide.en.md)

## 1) 启动图像后端（以 SGLang 为例）

```bash
sglang serve \
  --model-path Qwen/Qwen-Image \
  --host 0.0.0.0 \
  --port 8122
```

SGLang 还有很多用于加速推理的参数与环境变量，这里仅保留最小可用示例。

## 2) 本地测试后端

向后端的 `POST /v1/images/generations` 发送请求，并将返回的图片保存到本地。

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
    # 如果你的后端不需要鉴权，可以删除这一行。
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

如果几秒内能在当前目录看到 `generated.png`，说明后端工作正常。

## 3) 通过 Tokiame 将后端接入 Tokilake

从当前源码目录安装 `tokiame`：

```bash
go install ./cmd/tokiame
```

如果你使用的是已发布版本，也可以通过 npm 安装器安装：

```bash
npm i -g @tokilake/tokiame
```

编辑配置文件 `~/.tokilake/tokiame.json`：

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

说明：

- `model_targets` 下的 key（这里是 `Qwen-Image`）是 **你在 Tokilake 侧调用时使用的模型名**。
- `mapped_name` 是后端真实模型名（会作为 `model` 字段透传给后端）。

启动 Tokiame：

```bash
tokiame
```

连接成功后，你会看到类似日志：

```text
worker connected group=... models=[Qwen-Image] backend_type=openai
```

## 4) 通过 Tokilake 从任意位置调用

随后，你就可以通过 Tokilake 的 `POST /v1/images/generations` 接口（携带你生成的 Token）进行调用：

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
