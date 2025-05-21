package openaiclient_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	openaiclient "tokiame/pkg/openai_client"

	"github.com/sashabaranov/go-openai"
)

func TestClient(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "You are a concise assistant."},
		{Role: openai.ChatMessageRoleUser, Content: "Who are you?"},
	}
	config := openaiclient.NewOpenAIClientConfigBuilder().OpenAIMessages(&messages).APIKey("test-key").BaseURL("http://127.0.0.1:19981/v1").Model("zhipu:GLM-4-Flash-250414").Build()
	client, _ := openaiclient.NewOpenAIClient(config)

	stream, err := client.CreateChatCompletionStream(context.Background())
	if err != nil {
		fmt.Printf("Error creating stream: %v", err)
	}
	for {
		resp, streamErr := stream.Recv()
		if errors.Is(streamErr, io.EOF) {
			fmt.Println("\nStream finished.")
			break
		}
		if streamErr != nil {
			fmt.Printf("\nStream error: %v\n", streamErr)
			break
		}
		fmt.Println(resp.Choices[0].Delta.Content)
	}

}
