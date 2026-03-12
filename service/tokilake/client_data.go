package tokilake

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

	"one-api/common"

	"github.com/xtaci/smux"
)

const tunnelChunkSize = 32 * 1024

var allowedTunnelRequestHeaders = map[string]struct{}{
	"accept":       {},
	"content-type": {},
}

func (c *Client) acceptDataStreams(ctx context.Context, smuxSession *smux.Session, errCh chan<- error) {
	for {
		stream, err := smuxSession.AcceptStream()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			select {
			case errCh <- fmt.Errorf("accept data stream: %w", err):
			default:
			}
			return
		}

		go c.handleDataStream(ctx, stream)
	}
}

func (c *Client) handleDataStream(ctx context.Context, stream io.ReadWriteCloser) {
	defer stream.Close()

	codec := NewTunnelStreamCodec(stream)
	request, err := codec.ReadRequest()
	if err != nil {
		c.warn("read tunnel request failed err=%v", err)
		return
	}

	target, err := c.resolveModelTarget(request.Model)
	if err != nil {
		_ = codec.WriteResponse(&TunnelResponse{
			RequestID: request.RequestID,
			Error: &ErrorMessage{
				Code:    "target_not_found",
				Message: err.Error(),
			},
		})
		return
	}

	requestURL, err := buildLocalTargetURL(target.URL, request.Path)
	if err != nil {
		_ = codec.WriteResponse(&TunnelResponse{
			RequestID: request.RequestID,
			Error: &ErrorMessage{
				Code:    "invalid_target_url",
				Message: err.Error(),
			},
		})
		return
	}

	requestHeaders := mergeRequestHeaders(request.Headers, target.Headers)
	requestBody, requestHeaders, err := prepareRequestForTarget(request.Body, requestHeaders, target)
	if err != nil {
		_ = codec.WriteResponse(&TunnelResponse{
			RequestID: request.RequestID,
			Error: &ErrorMessage{
				Code:    "rewrite_request_failed",
				Message: err.Error(),
			},
		})
		return
	}

	response, cleanup, err := c.doLocalRoundTrip(ctx, request.RequestID, request.Method, requestURL, requestBody, requestHeaders)
	if err != nil {
		_ = codec.WriteResponse(&TunnelResponse{
			RequestID: request.RequestID,
			Error: &ErrorMessage{
				Code:    "local_request_failed",
				Message: err.Error(),
			},
		})
		return
	}
	defer cleanup()
	defer response.Body.Close()

	if err = c.writeHTTPResponse(codec, request.RequestID, response); err != nil {
		c.warn("write tunnel response failed request_id=%s err=%v", request.RequestID, err)
	}
}

func (c *Client) doLocalRoundTrip(ctx context.Context, requestID string, method string, requestURL string, body []byte, headers map[string]string) (*http.Response, func(), error) {
	requestCtx, cancel := context.WithCancel(ctx)
	c.trackLocalRequest(requestID, cancel)
	cleanup := func() {
		c.removeLocalRequest(requestID)
		cancel()
	}

	httpRequest, err := http.NewRequestWithContext(requestCtx, method, requestURL, bytes.NewReader(body))
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("build request failed: %w", err)
	}
	applyLocalRequestHeaders(httpRequest, headers)

	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("do request failed: %w", err)
	}
	return response, cleanup, nil
}

func (c *Client) writeHTTPResponse(codec *TunnelStreamCodec, requestID string, response *http.Response) error {
	if response == nil {
		return fmt.Errorf("response is nil")
	}
	if err := codec.WriteResponse(&TunnelResponse{
		RequestID:  requestID,
		StatusCode: response.StatusCode,
		Headers:    flattenHTTPHeaders(response.Header),
	}); err != nil {
		return err
	}
	buffer := make([]byte, tunnelChunkSize)
	for {
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			if writeErr := codec.WriteResponse(&TunnelResponse{
				RequestID: requestID,
				BodyChunk: append([]byte(nil), buffer[:n]...),
			}); writeErr != nil {
				return writeErr
			}
		}
		if readErr == io.EOF {
			return codec.WriteResponse(&TunnelResponse{
				RequestID: requestID,
				EOF:       true,
			})
		}
		if readErr != nil {
			_ = codec.WriteResponse(&TunnelResponse{
				RequestID: requestID,
				Error: &ErrorMessage{
					Code:    "read_local_response_failed",
					Message: readErr.Error(),
				},
			})
			return readErr
		}
	}
}

func buildLocalTargetURL(baseTarget string, requestPath string) (string, error) {
	baseURL, err := url.Parse(strings.TrimSpace(baseTarget))
	if err != nil {
		return "", fmt.Errorf("parse base target %q: %w", baseTarget, err)
	}
	if baseURL.Scheme == "" || baseURL.Host == "" {
		return "", fmt.Errorf("base target must include scheme and host: %s", baseTarget)
	}

	requestURL, err := url.Parse(strings.TrimSpace(requestPath))
	if err != nil {
		return "", fmt.Errorf("parse request path %q: %w", requestPath, err)
	}

	baseURL.Path = mergeTargetPath(baseURL.Path, requestURL.Path)
	baseURL.RawQuery = requestURL.RawQuery
	return baseURL.String(), nil
}

func mergeTargetPath(basePath string, requestPath string) string {
	baseSegments := splitURLPathSegments(basePath)
	requestSegments := splitURLPathSegments(requestPath)

	switch {
	case len(requestSegments) == 0:
		if len(baseSegments) == 0 {
			return "/"
		}
		return "/" + strings.Join(baseSegments, "/")
	case len(baseSegments) == 0:
		return "/" + strings.Join(requestSegments, "/")
	}

	overlap := 0
	maxOverlap := minInt(len(baseSegments), len(requestSegments))
	for size := maxOverlap; size > 0; size-- {
		if pathSegmentsEqual(baseSegments[len(baseSegments)-size:], requestSegments[:size]) {
			overlap = size
			break
		}
	}

	merged := make([]string, 0, len(baseSegments)+len(requestSegments)-overlap)
	merged = append(merged, baseSegments...)
	merged = append(merged, requestSegments[overlap:]...)
	if len(merged) == 0 {
		return "/"
	}
	return "/" + strings.Join(merged, "/")
}

func splitURLPathSegments(rawPath string) []string {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" || rawPath == "/" {
		return nil
	}
	parts := strings.Split(rawPath, "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segments = append(segments, part)
	}
	if len(segments) == 0 {
		return nil
	}
	return segments
}

func pathSegmentsEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func applyLocalRequestHeaders(request *http.Request, headers map[string]string) {
	if request == nil {
		return
	}
	for key, value := range headers {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" || strings.EqualFold(key, "Host") {
			continue
		}
		request.Header.Set(key, value)
	}
}

func mergeRequestHeaders(tunnelHeaders map[string]string, targetHeaders map[string]string) map[string]string {
	merged := make(map[string]string, len(tunnelHeaders)+len(targetHeaders))
	for key, value := range tunnelHeaders {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		if _, ok := allowedTunnelRequestHeaders[normalizedKey]; !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		merged[key] = value
	}
	for key, value := range targetHeaders {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" || strings.EqualFold(key, "Host") {
			continue
		}
		merged[key] = value
	}
	return merged
}

func prepareRequestForTarget(body []byte, headers map[string]string, target *ResolvedTarget) ([]byte, map[string]string, error) {
	if target == nil {
		return body, headers, nil
	}
	upstreamModel := strings.TrimSpace(target.UpstreamModel)
	if upstreamModel == "" || upstreamModel == strings.TrimSpace(target.ModelName) {
		return body, headers, nil
	}

	contentType := headerValue(headers, "Content-Type")
	switch {
	case strings.HasPrefix(strings.ToLower(contentType), "application/json"):
		rewrittenBody, err := rewriteJSONModelField(body, upstreamModel)
		if err != nil {
			return nil, nil, err
		}
		return rewrittenBody, headers, nil
	case strings.HasPrefix(strings.ToLower(contentType), "application/x-www-form-urlencoded"):
		rewrittenBody, err := rewriteFormModelField(body, upstreamModel)
		if err != nil {
			return nil, nil, err
		}
		return rewrittenBody, headers, nil
	case strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data"):
		rewrittenBody, rewrittenContentType, err := rewriteMultipartModelField(body, contentType, upstreamModel)
		if err != nil {
			return nil, nil, err
		}
		setHeaderValue(headers, "Content-Type", rewrittenContentType)
		return rewrittenBody, headers, nil
	default:
		return body, headers, nil
	}
}

func rewriteJSONModelField(body []byte, upstreamModel string) ([]byte, error) {
	if len(body) == 0 {
		payload := map[string]any{"model": upstreamModel}
		return common.Marshal(payload)
	}
	var payload map[string]any
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("rewrite json model: %w", err)
	}
	payload["model"] = upstreamModel
	return common.Marshal(payload)
}

func rewriteFormModelField(body []byte, upstreamModel string) ([]byte, error) {
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, fmt.Errorf("rewrite form model: %w", err)
	}
	values.Set("model", upstreamModel)
	return []byte(values.Encode()), nil
}

func rewriteMultipartModelField(body []byte, contentType string, upstreamModel string) ([]byte, string, error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, "", fmt.Errorf("parse multipart content type: %w", err)
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return nil, "", fmt.Errorf("multipart boundary is missing")
	}

	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	modelWritten := false

	for {
		part, partErr := reader.NextPart()
		if partErr == io.EOF {
			break
		}
		if partErr != nil {
			return nil, "", fmt.Errorf("read multipart part: %w", partErr)
		}

		partBody, readErr := io.ReadAll(part)
		_ = part.Close()
		if readErr != nil {
			return nil, "", fmt.Errorf("read multipart part body: %w", readErr)
		}

		if part.FormName() == "model" && part.FileName() == "" {
			partBody = []byte(upstreamModel)
			modelWritten = true
		}

		mimeHeader := make(textproto.MIMEHeader)
		for key, values := range part.Header {
			for _, value := range values {
				mimeHeader.Add(key, value)
			}
		}
		newPart, createErr := writer.CreatePart(mimeHeader)
		if createErr != nil {
			return nil, "", fmt.Errorf("create multipart part: %w", createErr)
		}
		if _, writeErr := newPart.Write(partBody); writeErr != nil {
			return nil, "", fmt.Errorf("write multipart part: %w", writeErr)
		}
	}

	if !modelWritten {
		if err = writer.WriteField("model", upstreamModel); err != nil {
			return nil, "", fmt.Errorf("append multipart model field: %w", err)
		}
	}
	if err = writer.Close(); err != nil {
		return nil, "", fmt.Errorf("close multipart writer: %w", err)
	}
	return buffer.Bytes(), writer.FormDataContentType(), nil
}

func flattenHTTPHeaders(headers http.Header) map[string]string {
	flattened := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) == 0 {
			continue
		}
		flattened[key] = strings.Join(values, ", ")
	}
	return flattened
}

func headerValue(headers map[string]string, key string) string {
	for headerKey, value := range headers {
		if strings.EqualFold(headerKey, key) {
			return value
		}
	}
	return ""
}

func setHeaderValue(headers map[string]string, key string, value string) {
	for headerKey := range headers {
		if strings.EqualFold(headerKey, key) {
			headers[headerKey] = value
			return
		}
	}
	headers[key] = value
}
