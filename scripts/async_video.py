#!/usr/bin/env python3
import argparse
import json
import os
import time
from pathlib import Path

import requests


DEFAULT_PROMPT = "A red sports car driving along a coast road at sunset."


def parse_args():
    parser = argparse.ArgumentParser(description="Create, poll, and download a Tokilake async video task.")
    parser.add_argument("--base-url", default=os.getenv("TOKILAKE_BASE_URL", "http://127.0.0.1:19981"))
    parser.add_argument("--api-key", default=os.getenv("TOKILAKE_API_KEY"))
    parser.add_argument("--model", default=os.getenv("TOKILAKE_VIDEO_MODEL"))
    parser.add_argument("--prompt", default=os.getenv("TOKILAKE_VIDEO_PROMPT", DEFAULT_PROMPT))
    parser.add_argument("--size", default=os.getenv("TOKILAKE_VIDEO_SIZE", "832x480"))
    parser.add_argument("--image-url", default=os.getenv("TOKILAKE_IMAGE_URL"))
    parser.add_argument("--input-reference", type=Path, help="Local image file for image-to-video multipart upload.")
    parser.add_argument("--out", type=Path, default=Path("tokilake-video.mp4"))
    parser.add_argument("--poll-interval", type=float, default=2.0)
    parser.add_argument("--timeout", type=int, default=30 * 60)
    parser.add_argument("--num-frames", type=int, default=33)
    parser.add_argument("--fps", type=int, default=16)
    parser.add_argument("--steps", type=int, default=40)
    parser.add_argument("--guidance-scale", type=float, default=4.0)
    parser.add_argument("--seed", type=int, default=42)
    args = parser.parse_args()

    if not args.api_key:
        parser.error("--api-key or TOKILAKE_API_KEY is required")
    if not args.model:
        parser.error("--model or TOKILAKE_VIDEO_MODEL is required")
    if args.image_url and args.input_reference:
        parser.error("--image-url and --input-reference are mutually exclusive")
    if args.input_reference and not args.input_reference.exists():
        parser.error(f"--input-reference does not exist: {args.input_reference}")
    return args


def make_headers(api_key, accept="application/json", content_type=None):
    headers = {"Accept": accept}
    if content_type:
        headers["Content-Type"] = content_type
    headers["Authorization"] = f"Bearer {api_key}"
    return headers


def print_response(prefix, response):
    print(f"{prefix} HTTP:", response.status_code)
    print(f"{prefix} Content-Type:", response.headers.get("content-type"))


def print_error_response(response):
    try:
        print(json.dumps(response.json(), ensure_ascii=False, indent=2))
    except Exception:
        print(response.text[:4000])


def create_video(base_url, api_key, payload, input_reference=None):
    url = f"{base_url.rstrip('/')}/v1/videos"
    if input_reference:
        with input_reference.open("rb") as image_file:
            files = {"input_reference": (input_reference.name, image_file)}
            data = {key: str(value) for key, value in payload.items() if value is not None}
            response = requests.post(
                url,
                headers=make_headers(api_key),
                data=data,
                files=files,
                timeout=(10, 120),
            )
    else:
        response = requests.post(
            url,
            headers=make_headers(api_key, content_type="application/json"),
            json=payload,
            timeout=(10, 120),
        )

    print_response("Create", response)
    if response.status_code != 200:
        print_error_response(response)
        raise RuntimeError("failed to create video task")

    task = response.json()
    print(json.dumps(task, ensure_ascii=False, indent=2))
    video_id = task.get("id")
    if not video_id:
        raise RuntimeError(f"video id missing in response: {task}")
    return video_id


def wait_video(base_url, api_key, video_id, poll_interval, timeout):
    deadline = time.monotonic() + timeout
    pending_statuses = {"submitted", "queued", "processing", "in_progress", "pending", "running"}

    while True:
        if time.monotonic() > deadline:
            raise TimeoutError(f"video task timed out: {video_id}")

        response = requests.get(
            f"{base_url.rstrip('/')}/v1/videos/{video_id}",
            headers=make_headers(api_key),
            timeout=(10, 60),
        )
        print_response("Status", response)
        if response.status_code != 200:
            print_error_response(response)
            raise RuntimeError("failed to query video task")

        task = response.json()
        status = task.get("status")
        print("status:", status)

        if status == "completed":
            print(json.dumps(task, ensure_ascii=False, indent=2))
            return task
        if status == "failed":
            print(json.dumps(task, ensure_ascii=False, indent=2))
            raise RuntimeError("video task failed")
        if status not in pending_statuses:
            print(json.dumps(task, ensure_ascii=False, indent=2))
            raise RuntimeError(f"unknown video task status: {status}")

        time.sleep(poll_interval)


def download_video(base_url, api_key, video_id, out_path):
    response = requests.get(
        f"{base_url.rstrip('/')}/v1/videos/{video_id}/content",
        headers=make_headers(api_key, accept="video/mp4"),
        timeout=(10, 60 * 30),
    )
    print_response("Download", response)
    if response.status_code != 200:
        print_error_response(response)
        raise RuntimeError("failed to download video")

    out_path.write_bytes(response.content)
    print(f"Saved: {out_path.resolve()} | {out_path.stat().st_size / 1024 / 1024:.2f} MB")


def build_payload(args):
    mode = "image2video" if args.image_url or args.input_reference else "text2video"
    payload = {
        "model": args.model,
        "mode": mode,
        "prompt": args.prompt,
        "size": args.size,
        "num_frames": args.num_frames,
        "fps": args.fps,
        "num_inference_steps": args.steps,
        "guidance_scale": args.guidance_scale,
        "seed": args.seed,
    }
    if args.image_url:
        payload["reference_url"] = args.image_url
    return payload


def main():
    args = parse_args()
    payload = build_payload(args)
    video_id = create_video(args.base_url, args.api_key, payload, args.input_reference)
    print("video_id:", video_id)
    wait_video(args.base_url, args.api_key, video_id, args.poll_interval, args.timeout)
    download_video(args.base_url, args.api_key, video_id, args.out)


if __name__ == "__main__":
    main()
