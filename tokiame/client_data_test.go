package tokilake

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"testing"

	core "github.com/Tokimorphling/Tokilake/tokilake-core"
)

func TestPrepareVLLMOmniVideoCreateConvertsJSONToMultipart(t *testing.T) {
	request := &core.TunnelRequest{
		RouteKind: core.TunnelRouteKindVideosCreate,
		Method:    http.MethodPost,
		Path:      "/v1/videos",
		Body: []byte(`{
			"model":"wan-public",
			"prompt":"a cat playing piano",
			"size":"1280x720",
			"num_frames":33,
			"guidance_scale":4.5
		}`),
	}
	headers := map[string]string{"Content-Type": "application/json"}
	target := &ResolvedTarget{
		ModelName:     "wan-public",
		UpstreamModel: "wan-backend",
		BackendType:   "vllm_omni",
	}

	body, outHeaders, err := prepareRequestForTarget(request, headers, target)
	if err != nil {
		t.Fatalf("prepare request: %v", err)
	}

	fields := parseMultipartFields(t, body, headerValue(outHeaders, "Content-Type"))
	if fields["model"] != "wan-backend" {
		t.Fatalf("model = %q, want wan-backend", fields["model"])
	}
	if fields["prompt"] != "a cat playing piano" {
		t.Fatalf("prompt = %q", fields["prompt"])
	}
	if fields["num_frames"] != "33" {
		t.Fatalf("num_frames = %q, want 33", fields["num_frames"])
	}
	if fields["guidance_scale"] != "4.5" {
		t.Fatalf("guidance_scale = %q, want 4.5", fields["guidance_scale"])
	}
}

func TestPrepareSGLangVideoCreateKeepsJSONAndMapsReferenceURL(t *testing.T) {
	request := &core.TunnelRequest{
		RouteKind: core.TunnelRouteKindVideosCreate,
		Method:    http.MethodPost,
		Path:      "/v1/videos",
		Body: []byte(`{
			"model":"wan-public",
			"prompt":"animate this image",
			"image_url":"https://example.com/input.png",
			"size":"1280x720"
		}`),
	}
	headers := map[string]string{"Content-Type": "application/json"}
	target := &ResolvedTarget{
		ModelName:     "wan-public",
		UpstreamModel: "wan-backend",
		BackendType:   "sglang",
	}

	body, outHeaders, err := prepareRequestForTarget(request, headers, target)
	if err != nil {
		t.Fatalf("prepare request: %v", err)
	}
	if headerValue(outHeaders, "Content-Type") != "application/json" {
		t.Fatalf("content type = %q", headerValue(outHeaders, "Content-Type"))
	}

	var payload map[string]any
	if err = json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode output json: %v", err)
	}
	if payload["model"] != "wan-backend" {
		t.Fatalf("model = %q, want wan-backend", payload["model"])
	}
	if payload["reference_url"] != "https://example.com/input.png" {
		t.Fatalf("reference_url = %q", payload["reference_url"])
	}
	if payload["image_url"] != "https://example.com/input.png" {
		t.Fatalf("image_url = %q", payload["image_url"])
	}
}

func TestPrepareVLLMOmniVideoCreateRewritesMultipartModelAndKeepsFile(t *testing.T) {
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	if err := writer.WriteField("model", "wan-public"); err != nil {
		t.Fatalf("write model: %v", err)
	}
	if err := writer.WriteField("prompt", "animate this image"); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	filePart, err := writer.CreateFormFile("input_reference", "frame.png")
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if _, err = filePart.Write([]byte("image-bytes")); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	request := &core.TunnelRequest{
		RouteKind: core.TunnelRouteKindVideosCreate,
		Method:    http.MethodPost,
		Path:      "/v1/videos",
		Body:      requestBody.Bytes(),
	}
	headers := map[string]string{"Content-Type": writer.FormDataContentType()}
	target := &ResolvedTarget{
		ModelName:     "wan-public",
		UpstreamModel: "wan-backend",
		BackendType:   "vllm_omni",
	}

	body, outHeaders, err := prepareRequestForTarget(request, headers, target)
	if err != nil {
		t.Fatalf("prepare request: %v", err)
	}

	fields := parseMultipartFields(t, body, headerValue(outHeaders, "Content-Type"))
	if fields["model"] != "wan-backend" {
		t.Fatalf("model = %q, want wan-backend", fields["model"])
	}
	if fields["prompt"] != "animate this image" {
		t.Fatalf("prompt = %q", fields["prompt"])
	}
	if fields["input_reference"] != "image-bytes" {
		t.Fatalf("input_reference = %q", fields["input_reference"])
	}
}

func TestAdaptVLLMOmniVideoResponseNormalizesTaskPayload(t *testing.T) {
	request := &core.TunnelRequest{
		RouteKind: core.TunnelRouteKindVideosGet,
		Method:    http.MethodGet,
		Path:      "/v1/videos/video_gen_123",
	}
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewBufferString(`{
			"id":"video_gen_123",
			"object":"video",
			"task_status":"in_progress",
			"model":"wan-backend",
			"prompt":"a cat playing piano",
			"created_at":1234567890,
			"data":[{"url":"file:///tmp/video.mp4"}]
		}`)),
	}
	target := &ResolvedTarget{
		ModelName:     "wan-public",
		UpstreamModel: "wan-backend",
		BackendType:   "vllm_omni",
	}

	adapted, err := adaptResponseForTarget(request, response, target)
	if err != nil {
		t.Fatalf("adapt response: %v", err)
	}
	defer adapted.Body.Close()

	body, err := io.ReadAll(adapted.Body)
	if err != nil {
		t.Fatalf("read adapted body: %v", err)
	}
	var payload map[string]any
	if err = json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode adapted body: %v", err)
	}
	if payload["status"] != "processing" {
		t.Fatalf("status = %q, want processing", payload["status"])
	}
	if payload["created"].(float64) != 1234567890 {
		t.Fatalf("created = %v", payload["created"])
	}
	if payload["download_url"] != "file:///tmp/video.mp4" {
		t.Fatalf("download_url = %q", payload["download_url"])
	}
	if adapted.Header.Get("Content-Length") != strconv.Itoa(len(body)) {
		t.Fatalf("content-length = %q, want %d", adapted.Header.Get("Content-Length"), len(body))
	}
}

func TestAdaptVideoResponseBuildsErrorObject(t *testing.T) {
	request := &core.TunnelRequest{
		RouteKind: core.TunnelRouteKindVideosGet,
		Method:    http.MethodGet,
		Path:      "/v1/videos/video_gen_123",
	}
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewBufferString(`{
			"id":"video_gen_123",
			"status":"failure",
			"fail_reason":"out of memory"
		}`)),
	}
	target := &ResolvedTarget{BackendType: "vllm_omni"}

	adapted, err := adaptResponseForTarget(request, response, target)
	if err != nil {
		t.Fatalf("adapt response: %v", err)
	}
	defer adapted.Body.Close()

	var payload map[string]any
	if err = json.NewDecoder(adapted.Body).Decode(&payload); err != nil {
		t.Fatalf("decode adapted body: %v", err)
	}
	if payload["status"] != "failed" {
		t.Fatalf("status = %q, want failed", payload["status"])
	}
	errorPayload, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("error payload missing: %#v", payload["error"])
	}
	if errorPayload["message"] != "out of memory" {
		t.Fatalf("error message = %q", errorPayload["message"])
	}
}

func parseMultipartFields(t *testing.T, body []byte, contentType string) map[string]string {
	t.Helper()

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse content type: %v", err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("media type = %q, want multipart/form-data", mediaType)
	}

	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	fields := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		partBody, err := io.ReadAll(part)
		_ = part.Close()
		if err != nil {
			t.Fatalf("read part body: %v", err)
		}
		fields[part.FormName()] = string(partBody)
	}
	return fields
}
