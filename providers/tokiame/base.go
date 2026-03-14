package tokiame

import (
	"context"
	"io"
	"net/http"
	"strings"

	"one-api/common"
	"one-api/common/config"
	"one-api/common/requester"
	"one-api/model"
	"one-api/providers/base"
	"one-api/providers/openai"
	tokilakesvc "one-api/service/tokilake"
	"one-api/types"

	"github.com/gin-gonic/gin"
)

type ProviderFactory struct{}

type Provider struct {
	base.BaseProvider
}

func (f ProviderFactory) Create(channel *model.Channel) base.ProviderInterface {
	return &Provider{
		BaseProvider: base.BaseProvider{
			Config:          getConfig(),
			Channel:         channel,
			Requester:       requester.NewHTTPRequester("", openai.RequestErrorHandle),
			SupportResponse: true,
		},
	}
}

func getConfig() base.ProviderConfig {
	return base.ProviderConfig{
		Completions:         "/v1/completions",
		ChatCompletions:     "/v1/chat/completions",
		Embeddings:          "/v1/embeddings",
		AudioSpeech:         "/v1/audio/speech",
		AudioTranscriptions: "/v1/audio/transcriptions",
		AudioTranslations:   "/v1/audio/translations",
		ImagesGenerations:   "/v1/images/generations",
		ImagesEdit:          "/v1/images/edits",
		ImagesVariations:    "/v1/images/variations",
		Videos:              "/v1/videos",
		ModelList:           "/v1/models",
		Rerank:              "/v1/rerank",
		Responses:           "/v1/responses",
	}
}

func (p *Provider) SetContext(c *gin.Context) {
	p.BaseProvider.SetContext(c)
}

func (p *Provider) GetRequestHeaders() map[string]string {
	headers := make(map[string]string)
	p.CommonRequestHeaders(headers)
	delete(headers, "Authorization")
	delete(headers, "OpenAI-Organization")
	return headers
}

func (p *Provider) CreateChatCompletion(request *types.ChatCompletionRequest) (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode) {
	resp, err := p.doJSONRequest(config.RelayModeChatCompletions, tokilakesvc.TunnelRouteKindChatCompletions, request.Model, request, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	response := &openai.OpenAIProviderChatResponse{}
	if errWithCode := p.decodeOpenAIResponse(resp, response); errWithCode != nil {
		return nil, errWithCode
	}

	if response.Usage == nil || response.Usage.CompletionTokens == 0 {
		response.Usage = &types.Usage{
			PromptTokens:     p.Usage.PromptTokens,
			CompletionTokens: common.CountTokenText(response.GetContent(), request.Model),
		}
		response.Usage.TotalTokens = response.Usage.PromptTokens + response.Usage.CompletionTokens
	}
	*p.Usage = *response.Usage
	return &response.ChatCompletionResponse, nil
}

func (p *Provider) CreateChatCompletionStream(request *types.ChatCompletionRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	resp, err := p.doJSONRequest(config.RelayModeChatCompletions, tokilakesvc.TunnelRouteKindChatCompletions, request.Model, request, true)
	if err != nil {
		return nil, err
	}

	handler := openai.OpenAIStreamHandler{
		Usage:     p.Usage,
		ModelName: request.Model,
	}
	return requester.RequestStream[string](p.Requester, resp, handler.HandlerChatStream)
}

func (p *Provider) CreateCompletion(request *types.CompletionRequest) (*types.CompletionResponse, *types.OpenAIErrorWithStatusCode) {
	resp, err := p.doJSONRequest(config.RelayModeCompletions, tokilakesvc.TunnelRouteKindCompletions, request.Model, request, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	response := &openai.OpenAIProviderCompletionResponse{}
	if errWithCode := p.decodeOpenAIResponse(resp, response); errWithCode != nil {
		return nil, errWithCode
	}
	if response.Usage != nil {
		*p.Usage = *response.Usage
	}
	return &response.CompletionResponse, nil
}

func (p *Provider) CreateCompletionStream(request *types.CompletionRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	resp, err := p.doJSONRequest(config.RelayModeCompletions, tokilakesvc.TunnelRouteKindCompletions, request.Model, request, true)
	if err != nil {
		return nil, err
	}

	handler := completionStreamHandler{
		usage: p.Usage,
		model: request.Model,
	}
	return requester.RequestStream[string](p.Requester, resp, handler.handle)
}

func (p *Provider) CreateEmbeddings(request *types.EmbeddingRequest) (*types.EmbeddingResponse, *types.OpenAIErrorWithStatusCode) {
	resp, err := p.doJSONRequest(config.RelayModeEmbeddings, tokilakesvc.TunnelRouteKindEmbeddings, request.Model, request, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	response := &openai.OpenAIProviderEmbeddingsResponse{}
	if errWithCode := p.decodeOpenAIResponse(resp, response); errWithCode != nil {
		return nil, errWithCode
	}
	if response.Usage != nil {
		*p.Usage = *response.Usage
	}
	return &response.EmbeddingResponse, nil
}

func (p *Provider) CreateSpeech(request *types.SpeechAudioRequest) (*http.Response, *types.OpenAIErrorWithStatusCode) {
	resp, err := p.doJSONRequest(config.RelayModeAudioSpeech, tokilakesvc.TunnelRouteKindAudioSpeech, request.Model, request, false)
	if err != nil {
		return nil, err
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "application/json") {
		return nil, requester.HandleErrorResp(resp, openai.RequestErrorHandle, p.Requester.IsOpenAI)
	}
	p.Usage.TotalTokens = p.Usage.PromptTokens
	return resp, nil
}

func (p *Provider) CreateTranscriptions(request *types.AudioRequest) (*types.AudioResponseWrapper, *types.OpenAIErrorWithStatusCode) {
	return p.doAudioRequest(config.RelayModeAudioTranscription, tokilakesvc.TunnelRouteKindAudioTranscription, request)
}

func (p *Provider) CreateTranslation(request *types.AudioRequest) (*types.AudioResponseWrapper, *types.OpenAIErrorWithStatusCode) {
	return p.doAudioRequest(config.RelayModeAudioTranslation, tokilakesvc.TunnelRouteKindAudioTranslation, request)
}

func (p *Provider) CreateImageGenerations(request *types.ImageRequest) (*types.ImageResponse, *types.OpenAIErrorWithStatusCode) {
	resp, err := p.doJSONRequest(config.RelayModeImagesGenerations, tokilakesvc.TunnelRouteKindImagesGenerations, request.Model, request, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	response := &openai.OpenAIProviderImageResponse{}
	if errWithCode := p.decodeOpenAIResponse(resp, response); errWithCode != nil {
		return nil, errWithCode
	}
	if response.Usage != nil && response.Usage.TotalTokens > 0 {
		*p.Usage = *response.Usage.ToOpenAIUsage()
	} else {
		p.Usage.TotalTokens = p.Usage.PromptTokens
	}
	return &response.ImageResponse, nil
}

func (p *Provider) CreateImageEdits(request *types.ImageEditRequest) (*types.ImageResponse, *types.OpenAIErrorWithStatusCode) {
	return p.doImageMultipartRequest(config.RelayModeImagesEdits, tokilakesvc.TunnelRouteKindImagesEdits, request)
}

func (p *Provider) CreateImageVariations(request *types.ImageEditRequest) (*types.ImageResponse, *types.OpenAIErrorWithStatusCode) {
	return p.doImageMultipartRequest(config.RelayModeImagesVariations, tokilakesvc.TunnelRouteKindImagesVariations, request)
}

func (p *Provider) CreateResponses(request *types.OpenAIResponsesRequest) (*types.OpenAIResponsesResponses, *types.OpenAIErrorWithStatusCode) {
	resp, err := p.doJSONRequest(config.RelayModeResponses, tokilakesvc.TunnelRouteKindResponses, request.Model, request, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	response := &types.OpenAIResponsesResponses{}
	if errWithCode := p.decodeJSONResponse(resp, response); errWithCode != nil {
		return nil, errWithCode
	}
	if response.Usage == nil || response.Usage.OutputTokens == 0 {
		response.Usage = &types.ResponsesUsage{
			InputTokens:  p.Usage.PromptTokens,
			OutputTokens: common.CountTokenText(response.GetContent(), request.Model),
		}
		response.Usage.TotalTokens = response.Usage.InputTokens + response.Usage.OutputTokens
	}
	*p.Usage = *response.Usage.ToOpenAIUsage()
	return response, nil
}

func (p *Provider) CreateResponsesStream(request *types.OpenAIResponsesRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	resp, err := p.doJSONRequest(config.RelayModeResponses, tokilakesvc.TunnelRouteKindResponses, request.Model, request, true)
	if err != nil {
		return nil, err
	}

	handler := openai.OpenAIResponsesStreamHandler{
		Usage:  p.Usage,
		Prefix: "data: ",
		Model:  request.Model,
	}
	if request.ConvertChat {
		return requester.RequestStream[string](p.Requester, resp, handler.HandlerChatStream)
	}
	return requester.RequestNoTrimStream[string](p.Requester, resp, handler.HandlerResponsesStream)
}

func (p *Provider) CreateRerank(request *types.RerankRequest) (*types.RerankResponse, *types.OpenAIErrorWithStatusCode) {
	resp, err := p.doJSONRequest(config.RelayModeRerank, tokilakesvc.TunnelRouteKindRerank, request.Model, request, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	response := &types.RerankResponse{}
	if errWithCode := p.decodeJSONResponse(resp, response); errWithCode != nil {
		return nil, errWithCode
	}
	if response.Usage != nil {
		*p.Usage = *response.Usage
	} else {
		p.Usage.TotalTokens = p.Usage.PromptTokens
	}
	return response, nil
}

func (p *Provider) GetModelList() ([]string, error) {
	if p.Channel == nil || strings.TrimSpace(p.Channel.Models) == "" {
		return nil, nil
	}
	models := strings.Split(p.Channel.Models, ",")
	result := make([]string, 0, len(models))
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		result = append(result, modelName)
	}
	return result, nil
}

func (p *Provider) doAudioRequest(relayMode int, routeKind string, request *types.AudioRequest) (*types.AudioResponseWrapper, *types.OpenAIErrorWithStatusCode) {
	resp, err := p.doMultipartRequest(relayMode, routeKind, request.Model)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, common.ErrorWrapper(readErr, "read_response_body_failed", http.StatusInternalServerError)
	}

	var textResponse string
	if hasJSONAudioResponse(request) {
		audioResponse := &openai.OpenAIProviderTranscriptionsResponse{}
		if decodeErr := common.Unmarshal(body, audioResponse); decodeErr != nil {
			return nil, common.ErrorWrapper(decodeErr, "decode_response_failed", http.StatusInternalServerError)
		}
		if openAIError := openai.ErrorHandle(&audioResponse.OpenAIErrorResponse); openAIError != nil {
			return nil, &types.OpenAIErrorWithStatusCode{
				OpenAIError: *openAIError,
				StatusCode:  http.StatusBadRequest,
			}
		}
		textResponse = audioResponse.Text
	} else {
		textResponse = getAudioTextContent(string(body), request.ResponseFormat)
	}

	p.Usage.CompletionTokens = common.CountTokenText(textResponse, request.Model)
	p.Usage.TotalTokens = p.Usage.PromptTokens + p.Usage.CompletionTokens

	return &types.AudioResponseWrapper{
		Headers: map[string]string{
			"Content-Type": resp.Header.Get("Content-Type"),
		},
		Body: body,
	}, nil
}

func (p *Provider) doImageMultipartRequest(relayMode int, routeKind string, request *types.ImageEditRequest) (*types.ImageResponse, *types.OpenAIErrorWithStatusCode) {
	resp, err := p.doMultipartRequest(relayMode, routeKind, request.Model)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	response := &openai.OpenAIProviderImageResponse{}
	if errWithCode := p.decodeOpenAIResponse(resp, response); errWithCode != nil {
		return nil, errWithCode
	}
	if response.Usage != nil && response.Usage.TotalTokens > 0 {
		*p.Usage = *response.Usage.ToOpenAIUsage()
	} else {
		p.Usage.TotalTokens = p.Usage.PromptTokens
	}
	return &response.ImageResponse, nil
}

func (p *Provider) doJSONRequest(relayMode int, routeKind string, modelName string, request any, isStream bool) (*http.Response, *types.OpenAIErrorWithStatusCode) {
	path, errWithCode := p.GetSupportedAPIUri(relayMode)
	if errWithCode != nil {
		return nil, errWithCode
	}

	body, err := p.buildJSONBody(path, request)
	if err != nil {
		return nil, common.ErrorWrapper(err, "marshal_request_failed", http.StatusInternalServerError)
	}

	return p.doTunnelHTTPRequest(http.MethodPost, path, routeKind, modelName, body, "application/json", isStream)
}

func (p *Provider) doMultipartRequest(relayMode int, routeKind string, modelName string) (*http.Response, *types.OpenAIErrorWithStatusCode) {
	path, errWithCode := p.GetSupportedAPIUri(relayMode)
	if errWithCode != nil {
		return nil, errWithCode
	}

	body, contentType, ok := p.rawBodyForPath(path)
	if !ok {
		return nil, common.StringErrorWrapperLocal("request body not found", "request_body_not_found", http.StatusInternalServerError)
	}

	resp, errWithCode := p.doTunnelHTTPRequest(http.MethodPost, path, routeKind, modelName, body, contentType, false)
	if errWithCode != nil {
		return nil, errWithCode
	}
	if p.Requester.IsFailureStatusCode(resp) {
		return nil, requester.HandleErrorResp(resp, openai.RequestErrorHandle, p.Requester.IsOpenAI)
	}
	return resp, nil
}

func (p *Provider) doTunnelHTTPRequest(method string, path string, routeKind string, modelName string, body []byte, contentType string, isStream bool) (*http.Response, *types.OpenAIErrorWithStatusCode) {
	resp, _, requestErr := tokilakesvc.DoTunnelRequest(p.requestContext(), p.Channel.Id, &tokilakesvc.TunnelRequest{
		RouteKind: routeKind,
		Method:    method,
		Path:      path,
		Model:     modelName,
		Headers:   p.buildTunnelHeaders(path, contentType, isStream),
		IsStream:  isStream,
		Body:      body,
	})
	if requestErr != nil {
		return nil, common.ErrorWrapperLocal(requestErr, "tokiame_request_failed", http.StatusServiceUnavailable)
	}
	return resp, nil
}

func (p *Provider) buildJSONBody(path string, request any) ([]byte, error) {
	if rawBody, contentType, ok := p.rawBodyForPath(path); ok && strings.HasPrefix(strings.ToLower(contentType), "application/json") {
		return rawBody, nil
	}
	return common.Marshal(request)
}

func (p *Provider) rawBodyForPath(path string) ([]byte, string, bool) {
	if p.Context == nil || p.Context.Request == nil {
		return nil, "", false
	}
	rawBody, ok := p.GetRawBody()
	if !ok {
		return nil, "", false
	}
	requestPath := p.Context.Request.URL.Path
	if requestPath != path && !strings.HasPrefix(requestPath, path) {
		return nil, "", false
	}
	return rawBody, p.Context.Request.Header.Get("Content-Type"), true
}

func (p *Provider) buildTunnelHeaders(path string, contentType string, isStream bool) map[string]string {
	headers := p.GetRequestHeaders()
	if contentType != "" {
		headers["Content-Type"] = contentType
	}
	if headers["Content-Type"] == "" {
		headers["Content-Type"] = "application/json"
	}
	if p.Context != nil && p.Context.Request != nil {
		if accept := p.Context.Request.Header.Get("Accept"); accept != "" {
			headers["Accept"] = accept
		}
	}
	if headers["Accept"] == "" && isStream {
		headers["Accept"] = "text/event-stream"
	}
	return headers
}

func (p *Provider) requestContext() context.Context {
	if p.Context != nil && p.Context.Request != nil {
		return p.Context.Request.Context()
	}
	return context.Background()
}

func (p *Provider) decodeJSONResponse(resp *http.Response, target any) *types.OpenAIErrorWithStatusCode {
	if p.Requester.IsFailureStatusCode(resp) {
		return requester.HandleErrorResp(resp, openai.RequestErrorHandle, p.Requester.IsOpenAI)
	}
	if err := common.DecodeJson(resp.Body, target); err != nil {
		return common.ErrorWrapper(err, "decode_response_failed", http.StatusInternalServerError)
	}
	return nil
}

func (p *Provider) decodeOpenAIResponse(resp *http.Response, target any) *types.OpenAIErrorWithStatusCode {
	if errWithCode := p.decodeJSONResponse(resp, target); errWithCode != nil {
		return errWithCode
	}
	switch payload := target.(type) {
	case *openai.OpenAIProviderChatResponse:
		if openAIError := openai.ErrorHandle(&payload.OpenAIErrorResponse); openAIError != nil {
			return &types.OpenAIErrorWithStatusCode{
				OpenAIError: *openAIError,
				StatusCode:  http.StatusBadRequest,
			}
		}
	case *openai.OpenAIProviderCompletionResponse:
		if openAIError := openai.ErrorHandle(&payload.OpenAIErrorResponse); openAIError != nil {
			return &types.OpenAIErrorWithStatusCode{
				OpenAIError: *openAIError,
				StatusCode:  http.StatusBadRequest,
			}
		}
	case *openai.OpenAIProviderEmbeddingsResponse:
		if openAIError := openai.ErrorHandle(&payload.OpenAIErrorResponse); openAIError != nil {
			return &types.OpenAIErrorWithStatusCode{
				OpenAIError: *openAIError,
				StatusCode:  http.StatusBadRequest,
			}
		}
	case *openai.OpenAIProviderImageResponse:
		if openAIError := openai.ErrorHandle(&payload.OpenAIErrorResponse); openAIError != nil {
			return &types.OpenAIErrorWithStatusCode{
				OpenAIError: *openAIError,
				StatusCode:  http.StatusBadRequest,
			}
		}
	}
	return nil
}
