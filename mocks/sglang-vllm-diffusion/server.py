#!/usr/bin/env python3
from __future__ import annotations

import argparse
import base64
import json
import os
import threading
import time
import uuid
from dataclasses import dataclass
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any
from urllib.parse import urlparse


PNG_1X1_BASE64 = (
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO2ZQ6cAAAAASUVORK5CYII="
)

MOCK_MP4_BYTES = (
    b"\x00\x00\x00\x18ftypisom\x00\x00\x02\x00isomiso2mp41"
    b"\x00\x00\x00\x08free"
    b"\x00\x00\x00\x08mdat"
)


@dataclass
class ServerConfig:
    host: str
    port: int
    base_url: str
    image_model: str
    video_model: str
    image_delay_ms: int
    video_queue_seconds: float
    video_process_seconds: float
    bearer_token: str | None


@dataclass
class ImageAsset:
    asset_id: str
    created_at: int
    content_type: str
    payload: bytes


@dataclass
class VideoJob:
    video_id: str
    created_at: int
    prompt: str
    model: str
    size: str
    seed: int | None
    duration_seconds: int
    fps: int
    should_fail: bool

    def status(self, config: ServerConfig, now: float | None = None) -> str:
        now = now if now is not None else time.time()
        elapsed = max(0.0, now - float(self.created_at))
        if elapsed < config.video_queue_seconds:
            return "queued"
        if elapsed < config.video_queue_seconds + config.video_process_seconds:
            return "processing"
        if self.should_fail:
            return "failed"
        return "completed"


class MockState:
    def __init__(self) -> None:
        self.lock = threading.Lock()
        self.image_assets: dict[str, ImageAsset] = {}
        self.video_jobs: dict[str, VideoJob] = {}

    def save_image(self, asset: ImageAsset) -> None:
        with self.lock:
            self.image_assets[asset.asset_id] = asset

    def get_image(self, asset_id: str) -> ImageAsset | None:
        with self.lock:
            return self.image_assets.get(asset_id)

    def save_video(self, job: VideoJob) -> None:
        with self.lock:
            self.video_jobs[job.video_id] = job

    def get_video(self, video_id: str) -> VideoJob | None:
        with self.lock:
            return self.video_jobs.get(video_id)

    def list_videos(self) -> list[VideoJob]:
        with self.lock:
            jobs = list(self.video_jobs.values())
        jobs.sort(key=lambda item: item.created_at, reverse=True)
        return jobs


class DiffusionMockHTTPServer(ThreadingHTTPServer):
    def __init__(self, server_address: tuple[str, int], config: ServerConfig) -> None:
        super().__init__(server_address, DiffusionMockHandler)
        self.config = config
        self.state = MockState()


class DiffusionMockHandler(BaseHTTPRequestHandler):
    server: DiffusionMockHTTPServer
    protocol_version = "HTTP/1.1"

    def do_GET(self) -> None:
        parsed = urlparse(self.path)
        path = parsed.path

        if not self._authorize():
            return

        if path in ("/health", "/ready"):
            self._json_response(HTTPStatus.OK, {"status": "ok"})
            return

        if path in ("/models", "/v1/models"):
            self._json_response(HTTPStatus.OK, self._models_payload())
            return

        if path == "/v1/videos":
            data = [self._video_payload(job) for job in self.server.state.list_videos()]
            self._json_response(HTTPStatus.OK, {"object": "list", "data": data})
            return

        if path.startswith("/v1/videos/") and path.endswith("/content"):
            video_id = path[len("/v1/videos/") : -len("/content")].strip("/")
            self._serve_video_content(video_id)
            return

        if path.startswith("/v1/videos/"):
            video_id = path[len("/v1/videos/") :].strip("/")
            self._serve_video(video_id)
            return

        if path.startswith("/mock/assets/images/"):
            asset_id = path[len("/mock/assets/images/") :].strip("/")
            self._serve_image_asset(asset_id)
            return

        self._openai_error(
            HTTPStatus.NOT_FOUND,
            "not_found_error",
            f"unsupported path: {path}",
            code="path_not_supported",
        )

    def do_POST(self) -> None:
        parsed = urlparse(self.path)
        path = parsed.path

        if not self._authorize():
            return

        if path == "/v1/images/generations":
            self._handle_image_generation()
            return

        if path == "/v1/videos":
            self._handle_video_create()
            return

        self._openai_error(
            HTTPStatus.NOT_FOUND,
            "not_found_error",
            f"unsupported path: {path}",
            code="path_not_supported",
        )

    def log_message(self, format: str, *args: Any) -> None:
        timestamp = time.strftime("%Y-%m-%d %H:%M:%S", time.localtime())
        print(f"[{timestamp}] {self.address_string()} {format % args}")

    def _handle_image_generation(self) -> None:
        body, err = self._read_json_body()
        if err is not None:
            self._openai_error(HTTPStatus.BAD_REQUEST, "invalid_request_error", err, code="invalid_json")
            return

        prompt = self._string_field(body, "prompt")
        if not prompt:
            self._openai_error(
                HTTPStatus.BAD_REQUEST,
                "invalid_request_error",
                "prompt is required",
                code="prompt_required",
                param="prompt",
            )
            return

        model = self._string_field(body, "model") or self.server.config.image_model
        n, parse_err = self._int_field(body, "n", default=1)
        if parse_err is not None:
            self._openai_error(
                HTTPStatus.BAD_REQUEST,
                "invalid_request_error",
                parse_err,
                code="invalid_n",
                param="n",
            )
            return
        if n < 1 or n > 8:
            self._openai_error(
                HTTPStatus.BAD_REQUEST,
                "invalid_request_error",
                "n must be between 1 and 8",
                code="invalid_n",
                param="n",
            )
            return

        response_format = self._string_field(body, "response_format") or "b64_json"
        if response_format not in ("b64_json", "url"):
            self._openai_error(
                HTTPStatus.BAD_REQUEST,
                "invalid_request_error",
                "response_format must be b64_json or url",
                code="invalid_response_format",
                param="response_format",
            )
            return

        if self._should_fail(prompt, "X-Mock-Fail"):
            self._openai_error(
                HTTPStatus.INTERNAL_SERVER_ERROR,
                "server_error",
                "mock image generation failed",
                code="mock_generation_failed",
            )
            return

        self._sleep_ms(self.server.config.image_delay_ms)

        now = int(time.time())
        results: list[dict[str, Any]] = []
        for _ in range(n):
            if response_format == "url":
                asset_id = f"img_{uuid.uuid4().hex}"
                asset = ImageAsset(
                    asset_id=asset_id,
                    created_at=now,
                    content_type="image/png",
                    payload=base64.b64decode(PNG_1X1_BASE64),
                )
                self.server.state.save_image(asset)
                results.append(
                    {
                        "url": f"{self._base_url()}/mock/assets/images/{asset.asset_id}",
                        "revised_prompt": self._revised_prompt(prompt, model),
                    }
                )
            else:
                results.append(
                    {
                        "b64_json": PNG_1X1_BASE64,
                        "revised_prompt": self._revised_prompt(prompt, model),
                    }
                )

        self._json_response(
            HTTPStatus.OK,
            {
                "created": now,
                "data": results,
                "model": model,
            },
        )

    def _handle_video_create(self) -> None:
        body, err = self._read_json_body()
        if err is not None:
            self._openai_error(HTTPStatus.BAD_REQUEST, "invalid_request_error", err, code="invalid_json")
            return

        prompt = self._string_field(body, "prompt")
        if not prompt:
            self._openai_error(
                HTTPStatus.BAD_REQUEST,
                "invalid_request_error",
                "prompt is required",
                code="prompt_required",
                param="prompt",
            )
            return

        n, parse_err = self._int_field(body, "n", default=1)
        if parse_err is not None:
            self._openai_error(
                HTTPStatus.BAD_REQUEST,
                "invalid_request_error",
                parse_err,
                code="unsupported_n",
                param="n",
            )
            return
        if n != 1:
            self._openai_error(
                HTTPStatus.BAD_REQUEST,
                "invalid_request_error",
                "mock server only supports n=1 for videos",
                code="unsupported_n",
                param="n",
            )
            return

        duration_seconds, parse_err = self._int_field(body, "duration", default=5)
        if parse_err is not None:
            self._openai_error(
                HTTPStatus.BAD_REQUEST,
                "invalid_request_error",
                parse_err,
                code="invalid_duration",
                param="duration",
            )
            return

        fps, parse_err = self._int_field(body, "fps", default=24)
        if parse_err is not None:
            self._openai_error(
                HTTPStatus.BAD_REQUEST,
                "invalid_request_error",
                parse_err,
                code="invalid_fps",
                param="fps",
            )
            return
        size = self._string_field(body, "size") or "1280x720"
        model = self._string_field(body, "model") or self.server.config.video_model
        seed = body.get("seed")
        if seed is not None:
            try:
                seed = int(seed)
            except (TypeError, ValueError):
                self._openai_error(
                    HTTPStatus.BAD_REQUEST,
                    "invalid_request_error",
                    "seed must be an integer",
                    code="invalid_seed",
                    param="seed",
                )
                return

        video_id = f"vid_{uuid.uuid4().hex}"
        job = VideoJob(
            video_id=video_id,
            created_at=int(time.time()),
            prompt=prompt,
            model=model,
            size=size,
            seed=seed,
            duration_seconds=duration_seconds,
            fps=fps,
            should_fail=self._should_fail(prompt, "X-Mock-Video-Fail"),
        )
        self.server.state.save_video(job)
        self._json_response(HTTPStatus.OK, self._video_payload(job))

    def _serve_image_asset(self, asset_id: str) -> None:
        asset = self.server.state.get_image(asset_id)
        if asset is None:
            self._openai_error(
                HTTPStatus.NOT_FOUND,
                "not_found_error",
                f"image asset not found: {asset_id}",
                code="image_not_found",
            )
            return
        self._bytes_response(HTTPStatus.OK, asset.payload, asset.content_type)

    def _serve_video(self, video_id: str) -> None:
        job = self.server.state.get_video(video_id)
        if job is None:
            self._openai_error(
                HTTPStatus.NOT_FOUND,
                "not_found_error",
                f"video not found: {video_id}",
                code="video_not_found",
            )
            return
        self._json_response(HTTPStatus.OK, self._video_payload(job))

    def _serve_video_content(self, video_id: str) -> None:
        job = self.server.state.get_video(video_id)
        if job is None:
            self._openai_error(
                HTTPStatus.NOT_FOUND,
                "not_found_error",
                f"video not found: {video_id}",
                code="video_not_found",
            )
            return

        status = job.status(self.server.config)
        if status == "failed":
            self._openai_error(
                HTTPStatus.BAD_GATEWAY,
                "server_error",
                "mock video generation failed",
                code="mock_video_failed",
            )
            return
        if status != "completed":
            self._openai_error(
                HTTPStatus.CONFLICT,
                "invalid_request_error",
                f"video is not ready yet, current status: {status}",
                code="video_not_ready",
            )
            return

        self._bytes_response(HTTPStatus.OK, MOCK_MP4_BYTES, "video/mp4")

    def _models_payload(self) -> dict[str, Any]:
        created = int(time.time())
        return {
            "object": "list",
            "data": [
                {
                    "id": self.server.config.image_model,
                    "object": "model",
                    "created": created,
                    "owned_by": "mock-vllm",
                    "modalities": ["image"],
                },
                {
                    "id": self.server.config.video_model,
                    "object": "model",
                    "created": created,
                    "owned_by": "mock-sglang",
                    "modalities": ["video"],
                },
            ],
        }

    def _video_payload(self, job: VideoJob) -> dict[str, Any]:
        status = job.status(self.server.config)
        payload: dict[str, Any] = {
            "id": job.video_id,
            "object": "video",
            "created": job.created_at,
            "created_at": job.created_at,
            "updated_at": int(time.time()),
            "model": job.model,
            "prompt": job.prompt,
            "size": job.size,
            "fps": job.fps,
            "duration": job.duration_seconds,
            "status": status,
            "content_url": f"{self._base_url()}/v1/videos/{job.video_id}/content",
            "download_url": f"{self._base_url()}/v1/videos/{job.video_id}/content",
        }
        if job.seed is not None:
            payload["seed"] = job.seed
        if status == "failed":
            payload["error"] = {
                "message": "mock video generation failed",
                "type": "server_error",
                "code": "mock_video_failed",
            }
        return payload

    def _authorize(self) -> bool:
        token = self.server.config.bearer_token
        if not token:
            return True

        header = self.headers.get("Authorization", "")
        expected = f"Bearer {token}"
        if header == expected:
            return True

        self._openai_error(
            HTTPStatus.UNAUTHORIZED,
            "invalid_request_error",
            "missing or invalid bearer token",
            code="invalid_api_key",
        )
        return False

    def _read_json_body(self) -> tuple[dict[str, Any], str | None]:
        content_length = self.headers.get("Content-Length")
        if not content_length:
            return {}, None
        try:
            raw = self.rfile.read(int(content_length))
        except ValueError:
            return {}, "invalid Content-Length"
        if not raw:
            return {}, None
        try:
            body = json.loads(raw)
        except json.JSONDecodeError as exc:
            return {}, f"invalid JSON body: {exc.msg}"
        if not isinstance(body, dict):
            return {}, "JSON body must be an object"
        return body, None

    def _json_response(self, status: HTTPStatus, payload: dict[str, Any]) -> None:
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(body)

    def _bytes_response(self, status: HTTPStatus, payload: bytes, content_type: str) -> None:
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(payload)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(payload)

    def _openai_error(
        self,
        status: HTTPStatus,
        err_type: str,
        message: str,
        *,
        code: str,
        param: str | None = None,
    ) -> None:
        payload: dict[str, Any] = {
            "error": {
                "message": message,
                "type": err_type,
                "code": code,
            }
        }
        if param is not None:
            payload["error"]["param"] = param
        self._json_response(status, payload)

    def _string_field(self, payload: dict[str, Any], key: str) -> str:
        value = payload.get(key)
        if value is None:
            return ""
        return str(value).strip()

    def _int_field(self, payload: dict[str, Any], key: str, *, default: int) -> tuple[int, str | None]:
        value = payload.get(key)
        if value is None or value == "":
            return default, None
        try:
            return int(value), None
        except (TypeError, ValueError):
            return default, f"{key} must be an integer"

    def _base_url(self) -> str:
        if self.server.config.base_url:
            return self.server.config.base_url.rstrip("/")
        host = self.headers.get("Host") or f"{self.server.config.host}:{self.server.config.port}"
        return f"http://{host}"

    def _should_fail(self, prompt: str, header_name: str) -> bool:
        header = self.headers.get(header_name, "")
        if header.lower() in ("1", "true", "yes"):
            return True
        return "[mock:fail]" in prompt.lower()

    def _revised_prompt(self, prompt: str, model: str) -> str:
        return f"{prompt} [mocked by {model}]"

    def _sleep_ms(self, delay_ms: int) -> None:
        if delay_ms <= 0:
            return
        time.sleep(float(delay_ms) / 1000.0)


def env_or_default(name: str, default: str) -> str:
    value = os.getenv(name)
    if value is None or value == "":
        return default
    return value


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Mock server for a small sglang/vLLM diffusion API subset."
    )
    parser.add_argument("--host", default=env_or_default("MOCK_DIFFUSION_HOST", "127.0.0.1"))
    parser.add_argument(
        "--port",
        type=int,
        default=int(env_or_default("MOCK_DIFFUSION_PORT", "30010")),
    )
    parser.add_argument("--base-url", default=env_or_default("MOCK_DIFFUSION_BASE_URL", ""))
    parser.add_argument(
        "--image-model",
        default=env_or_default("MOCK_DIFFUSION_IMAGE_MODEL", "Qwen/Qwen-Image"),
    )
    parser.add_argument(
        "--video-model",
        default=env_or_default("MOCK_DIFFUSION_VIDEO_MODEL", "THUDM/CogVideoX-5b"),
    )
    parser.add_argument(
        "--image-delay-ms",
        type=int,
        default=int(env_or_default("MOCK_DIFFUSION_IMAGE_DELAY_MS", "100")),
    )
    parser.add_argument(
        "--video-queue-seconds",
        type=float,
        default=float(env_or_default("MOCK_DIFFUSION_VIDEO_QUEUE_SECONDS", "1.0")),
    )
    parser.add_argument(
        "--video-process-seconds",
        type=float,
        default=float(env_or_default("MOCK_DIFFUSION_VIDEO_PROCESS_SECONDS", "2.0")),
    )
    parser.add_argument(
        "--bearer-token",
        default=env_or_default("MOCK_DIFFUSION_BEARER_TOKEN", ""),
        help="Optional bearer token. Leave empty to disable auth.",
    )
    return parser


def main() -> None:
    args = build_parser().parse_args()
    config = ServerConfig(
        host=args.host,
        port=args.port,
        base_url=args.base_url,
        image_model=args.image_model,
        video_model=args.video_model,
        image_delay_ms=args.image_delay_ms,
        video_queue_seconds=args.video_queue_seconds,
        video_process_seconds=args.video_process_seconds,
        bearer_token=args.bearer_token or None,
    )
    server = DiffusionMockHTTPServer((config.host, config.port), config)
    print(
        f"Mock diffusion server listening on http://{config.host}:{config.port} "
        f"(image model: {config.image_model}, video model: {config.video_model})"
    )
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
