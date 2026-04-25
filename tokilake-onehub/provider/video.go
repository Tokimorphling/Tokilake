package provider

import (
	"fmt"
	"net/http"
	"strings"

	"one-api/common"
	"one-api/common/config"
	"one-api/common/requester"
	"one-api/providers/openai"
	"one-api/types"
	tokilakesvc "github.com/Tokimorphling/Tokilake/tokilake-core"
)

func (p *Provider) CreateVideo(request *types.VideoRequest) (*types.VideoTaskObject, *types.OpenAIErrorWithStatusCode) {
	if request == nil {
		return nil, common.StringErrorWrapperLocal("request is required", "invalid_request", http.StatusBadRequest)
	}

	resp, errWithCode := p.doJSONRequest(
		config.RelayModeVideos,
		tokilakesvc.TunnelRouteKindVideosCreate,
		request.Model,
		request,
		false,
	)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer resp.Body.Close()

	response := &types.VideoTaskObject{}
	if errWithCode = p.decodeJSONResponse(resp, response); errWithCode != nil {
		return nil, errWithCode
	}
	return response, nil
}

func (p *Provider) GetVideo(taskID string) (*types.VideoTaskObject, *types.OpenAIErrorWithStatusCode) {
	modelName := strings.TrimSpace(p.GetOriginalModel())
	if modelName == "" {
		return nil, common.StringErrorWrapperLocal("video model is required", "video_model_required", http.StatusBadRequest)
	}

	resp, errWithCode := p.doTunnelHTTPRequest(
		http.MethodGet,
		fmt.Sprintf("/v1/videos/%s", strings.TrimSpace(taskID)),
		tokilakesvc.TunnelRouteKindVideosGet,
		modelName,
		nil,
		"application/json",
		false,
	)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer resp.Body.Close()

	response := &types.VideoTaskObject{}
	if errWithCode = p.decodeJSONResponse(resp, response); errWithCode != nil {
		return nil, errWithCode
	}
	return response, nil
}

func (p *Provider) GetVideoContent(taskID string) (*http.Response, *types.OpenAIErrorWithStatusCode) {
	modelName := strings.TrimSpace(p.GetOriginalModel())
	if modelName == "" {
		return nil, common.StringErrorWrapperLocal("video model is required", "video_model_required", http.StatusBadRequest)
	}

	resp, errWithCode := p.doTunnelHTTPRequest(
		http.MethodGet,
		fmt.Sprintf("/v1/videos/%s/content", strings.TrimSpace(taskID)),
		tokilakesvc.TunnelRouteKindVideosContent,
		modelName,
		nil,
		"",
		false,
	)
	if errWithCode != nil {
		return nil, errWithCode
	}
	if p.Requester.IsFailureStatusCode(resp) {
		return nil, requester.HandleErrorResp(resp, openai.RequestErrorHandle, p.Requester.IsOpenAI)
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "application/json") {
		return nil, requester.HandleErrorResp(resp, openai.RequestErrorHandle, p.Requester.IsOpenAI)
	}
	return resp, nil
}
