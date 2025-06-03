package openaiclient

import (
	"context"
	"errors"
	pb "tokiame/internal/pb"
	"tokiame/pkg/log"

	// For example usage
	// For example usage (streaming)
	// "log" // For example usage

	// For example usage
	"github.com/sashabaranov/go-openai"
)

// --- Configuration ---

type OpenAIClientConfig struct {
	apiKey   string // API key. For local models, this might be optional or a dummy string.
	baseURL  string // Crucial for local models (e.g., "http://localhost:11434/v1")
	model    string // Default model to use if not specified in a request
	messages *[]openai.ChatCompletionMessage
	stream   bool

	temperature float32
	top_p       float32
	max_tokens  int
}

type OpenAIClientConfigBuilder struct {
	config *OpenAIClientConfig
}

func NewOpenAIClientConfigBuilder() *OpenAIClientConfigBuilder {
	return &OpenAIClientConfigBuilder{
		config: &OpenAIClientConfig{
			// No default model here, as it's highly dependent on the local setup.
			// User should explicitly set it or provide it in each request.
		},
	}
}

// WithAPIKey sets the API key.
// For local OpenAI-compatible servers:
// - If your server doesn't require an API key and doesn't want an Authorization header, pass an empty string "".
// - If your server expects an Authorization header (even with a dummy key), pass any non-empty string (e.g., "NA", "local").
func (b *OpenAIClientConfigBuilder) APIKey(apiKey string) *OpenAIClientConfigBuilder {
	b.config.apiKey = apiKey
	return b
}

func (b *OpenAIClientConfigBuilder) MaxTokens(maxTokens int) *OpenAIClientConfigBuilder {
	b.config.max_tokens = maxTokens
	return b
}
func (b *OpenAIClientConfigBuilder) Tempratrue(temp float32) *OpenAIClientConfigBuilder {
	b.config.temperature = temp
	return b
}
func (b *OpenAIClientConfigBuilder) Topp(topp float32) *OpenAIClientConfigBuilder {
	b.config.top_p = topp
	return b
}

// WithBaseURL sets the base URL for the OpenAI API.
// Essential for connecting to local models (e.g., "http://localhost:11434/v1").
// If not set, it will default to the official OpenAI API URL.
func (b *OpenAIClientConfigBuilder) BaseURL(baseURL string) *OpenAIClientConfigBuilder {
	b.config.baseURL = baseURL
	return b
}

// WithModel sets a default model to be used if not specified in individual requests.
// This should be a model identifier recognized by your target OpenAI-compatible API.
func (b *OpenAIClientConfigBuilder) Model(model string) *OpenAIClientConfigBuilder {
	b.config.model = model
	return b
}

func (b *OpenAIClientConfigBuilder) Messages(messages []*pb.ChatMessage) *OpenAIClientConfigBuilder {
	chatMessages := make([]openai.ChatCompletionMessage, 0)
	for _, message := range messages {
		// message.GetContentType()
		chatMessage := openai.ChatCompletionMessage{Role: mapToOpenAIRole(message.Role)}
		if message.ContentType == nil {
			chatMessages = append(chatMessages, chatMessage)
			continue
		}

		switch v := message.ContentType.(type) {
		case *pb.ChatMessage_TextContent:
			chatMessage.Content = v.TextContent
		case *pb.ChatMessage_MultiContent:
			chatMessage.MultiContent = *mapToOpenAIContentPart(v.MultiContent.Parts)

		}
		chatMessages = append(chatMessages, chatMessage)
	}

	b.config.messages = &chatMessages
	return b
}

func (b *OpenAIClientConfigBuilder) OpenAIMessages(messages *[]openai.ChatCompletionMessage) *OpenAIClientConfigBuilder {
	b.config.messages = messages
	return b
}

func (b *OpenAIClientConfigBuilder) Build() OpenAIClientConfig {
	// Final validation of critical fields can happen in NewOpenAIClient
	return *b.config
}

// --- Client Wrapper ---

func mapToOpenAIRole(originalRole pb.ChatMessage_Role) string {
	switch originalRole {
	case pb.ChatMessage_ROLE_USER:
		return openai.ChatMessageRoleUser
	case pb.ChatMessage_ROLE_SYSTEM:
		return openai.ChatMessageRoleSystem
	case pb.ChatMessage_ROLE_ASSISTANT:
		return openai.ChatMessageRoleAssistant
	default:
		return openai.ChatMessageRoleAssistant
	}
}

func mapToOpenAIContentPart(parts []*pb.ContentPart) *[]openai.ChatMessagePart {

	newParts := make([]openai.ChatMessagePart, 0)

	for _, part := range parts {
		p := &openai.ChatMessagePart{}
		switch v := part.PartType.(type) {
		case *pb.ContentPart_Text:
			p.Type = openai.ChatMessagePartTypeText
			p.Text = v.Text
		case *pb.ContentPart_ImageData:
			p.Type = openai.ChatMessagePartTypeImageURL
			p.ImageURL = &openai.ChatMessageImageURL{URL: *v.ImageData.Uri}
		}

		newParts = append(newParts, *p)

	}
	return &newParts

}

type OpenAIClient struct {
	client *openai.Client
	config OpenAIClientConfig
}

// NewOpenAIClient creates a new instance of our wrapped OpenAI client.
// It configures the underlying go-openai client based on the provided config.
func NewOpenAIClient(config OpenAIClientConfig) (*OpenAIClient, error) {
	// If no custom baseURL is provided, it implies connection to the official OpenAI API,
	// which requires an API key.
	if config.baseURL == "" && config.apiKey == "" {
		return nil, errors.New("API key is required when no custom baseURL is provided (i.e., for official OpenAI API)")
	}

	// Configure the underlying go-openai client.
	// openai.DefaultConfig(apiKey) will:
	// - If apiKey is empty, not set the Authorization header.
	// - If apiKey is non-empty, set the Authorization header.
	// This behavior is suitable for both official OpenAI and various local server setups.
	clientConfig := openai.DefaultConfig(config.apiKey)

	if config.baseURL != "" {
		clientConfig.BaseURL = config.baseURL
	}
	if config.model == "" {
		log.Warn("Warning: No default model specified in OpenAIClientConfig. You'll need to specify the model in each request.")
	}

	client := openai.NewClientWithConfig(clientConfig)

	return &OpenAIClient{
		client: client,
		config: config,
	}, nil
}

// --- Wrapped Methods (Examples) ---

// CreateChatCompletion sends a chat completion request.
// Uses the default model from config if not specified in the request.
func (c *OpenAIClient) CreateChatCompletion(
	ctx context.Context,
	request openai.ChatCompletionRequest,
) (openai.ChatCompletionResponse, error) {
	if request.Model == "" {
		if c.config.model != "" {
			request.Model = c.config.model
		} else {
			return openai.ChatCompletionResponse{}, errors.New("model must be specified either in the request or in client configuration")
		}
	}
	return c.client.CreateChatCompletion(ctx, request)
}

// CreateChatCompletionStream sends a streaming chat completion request.
// Uses the default model from config if not specified in the request.
func (c *OpenAIClient) CreateChatCompletionStream(
	ctx context.Context,
	// request openai.ChatCompletionRequest,
) (*openai.ChatCompletionStream, error) {

	req := openai.ChatCompletionRequest{
		Model:       c.config.model,
		Messages:    *c.config.messages,
		MaxTokens:   int(c.config.max_tokens),
		Stream:      c.config.stream,
		Temperature: c.config.temperature,
		TopP:        c.config.top_p,
	}

	// if request.Model == "" {
	// 	if c.config.model != "" {
	// 		request.Model = c.config.model
	// 	} else {
	// 		return nil, errors.New("model must be specified either in the request or in client configuration for streaming")
	// 	}
	// }

	return c.client.CreateChatCompletionStream(ctx, req)
}

// GetConfig returns the current configuration of the client.
func (c *OpenAIClient) GetConfig() OpenAIClientConfig {
	return c.config
}

// GetUnderlyingClient returns the raw *openai.Client if direct access is needed.
func (c *OpenAIClient) GetUnderlyingClient() *openai.Client {
	return c.client
}
