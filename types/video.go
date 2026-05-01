package types

import (
	"encoding/json"
	"strings"
)

const (
	VideoModeTextToVideo  = "text2video"
	VideoModeImageToVideo = "image2video"

	VideoStatusSubmitted  = "submitted"
	VideoStatusQueued     = "queued"
	VideoStatusProcessing = "processing"
	VideoStatusCompleted  = "completed"
	VideoStatusFailed     = "failed"
)

var videoRequestKnownFields = map[string]struct{}{
	"model":          {},
	"mode":           {},
	"prompt":         {},
	"image_url":      {},
	"image_b64_json": {},
	"reference_url":  {},
	"size":           {},
	"duration":       {},
	"fps":            {},
	"seed":           {},
	"n":              {},
}

type VideoRequest struct {
	Model             string         `json:"model,omitempty" form:"model"`
	Mode              string         `json:"mode,omitempty" form:"mode"`
	Prompt            string         `json:"prompt,omitempty" form:"prompt"`
	ImageURL          string         `json:"image_url,omitempty" form:"image_url"`
	ImageB64JSON      string         `json:"image_b64_json,omitempty" form:"image_b64_json"`
	ReferenceURL      string         `json:"reference_url,omitempty" form:"reference_url"`
	Size              string         `json:"size,omitempty" form:"size"`
	Duration          *int           `json:"duration,omitempty" form:"duration"`
	FPS               *int           `json:"fps,omitempty" form:"fps"`
	Seed              *int           `json:"seed,omitempty" form:"seed"`
	N                 *int           `json:"n,omitempty" form:"n"`
	ExtraFields       map[string]any `json:"-" form:"-"`
	HasInputReference bool           `json:"-" form:"-"`
}

func (r *VideoRequest) UnmarshalJSON(data []byte) error {
	type alias struct {
		Model        string `json:"model"`
		Mode         string `json:"mode"`
		Prompt       string `json:"prompt"`
		ImageURL     string `json:"image_url"`
		ImageB64JSON string `json:"image_b64_json"`
		ReferenceURL string `json:"reference_url"`
		Size         string `json:"size"`
		Duration     *int   `json:"duration"`
		FPS          *int   `json:"fps"`
		Seed         *int   `json:"seed"`
		N            *int   `json:"n"`
	}

	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	rawFields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &rawFields); err != nil {
		return err
	}

	extraFields := make(map[string]any)
	for key, value := range rawFields {
		if _, ok := videoRequestKnownFields[key]; ok {
			continue
		}
		var decoded any
		if err := json.Unmarshal(value, &decoded); err != nil {
			continue
		}
		extraFields[key] = decoded
	}

	r.Model = strings.TrimSpace(parsed.Model)
	r.Mode = strings.TrimSpace(parsed.Mode)
	r.Prompt = strings.TrimSpace(parsed.Prompt)
	r.ImageURL = strings.TrimSpace(parsed.ImageURL)
	r.ImageB64JSON = strings.TrimSpace(parsed.ImageB64JSON)
	r.ReferenceURL = strings.TrimSpace(parsed.ReferenceURL)
	r.Size = strings.TrimSpace(parsed.Size)
	r.Duration = parsed.Duration
	r.FPS = parsed.FPS
	r.Seed = parsed.Seed
	r.N = parsed.N
	if len(extraFields) == 0 {
		r.ExtraFields = nil
	} else {
		r.ExtraFields = extraFields
	}
	return nil
}

func (r VideoRequest) MarshalJSON() ([]byte, error) {
	payload := make(map[string]any, len(r.ExtraFields)+10)
	for key, value := range r.ExtraFields {
		payload[key] = value
	}
	if r.Model != "" {
		payload["model"] = r.Model
	}
	if r.Mode != "" {
		payload["mode"] = r.Mode
	}
	if r.Prompt != "" {
		payload["prompt"] = r.Prompt
	}
	if r.ImageURL != "" {
		payload["image_url"] = r.ImageURL
	}
	if r.ImageB64JSON != "" {
		payload["image_b64_json"] = r.ImageB64JSON
	}
	if r.ReferenceURL != "" {
		payload["reference_url"] = r.ReferenceURL
	}
	if r.Size != "" {
		payload["size"] = r.Size
	}
	if r.Duration != nil {
		payload["duration"] = *r.Duration
	}
	if r.FPS != nil {
		payload["fps"] = *r.FPS
	}
	if r.Seed != nil {
		payload["seed"] = *r.Seed
	}
	if r.N != nil {
		payload["n"] = *r.N
	}
	return json.Marshal(payload)
}

type VideoTaskError struct {
	Message string `json:"message,omitempty"`
	Type    string `json:"type,omitempty"`
	Code    any    `json:"code,omitempty"`
}

type VideoTaskObject struct {
	ID          string          `json:"id,omitempty"`
	Object      string          `json:"object,omitempty"`
	Created     int64           `json:"created,omitempty"`
	Model       string          `json:"model,omitempty"`
	Mode        string          `json:"mode,omitempty"`
	Status      string          `json:"status,omitempty"`
	Prompt      string          `json:"prompt,omitempty"`
	Size        string          `json:"size,omitempty"`
	Duration    *int            `json:"duration,omitempty"`
	FPS         *int            `json:"fps,omitempty"`
	Seed        *int            `json:"seed,omitempty"`
	ContentURL  string          `json:"content_url,omitempty"`
	DownloadURL string          `json:"download_url,omitempty"`
	Error       *VideoTaskError `json:"error,omitempty"`
}

type VideoListResponse struct {
	Object string            `json:"object"`
	Data   []VideoTaskObject `json:"data"`
}

type VideoTaskProperties struct {
	Model       string `json:"model,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
	Size        string `json:"size,omitempty"`
	ImageSource string `json:"image_source,omitempty"`
	HasImageB64 bool   `json:"has_image_b64,omitempty"`
	Duration    *int   `json:"duration,omitempty"`
	FPS         *int   `json:"fps,omitempty"`
	Seed        *int   `json:"seed,omitempty"`
}
