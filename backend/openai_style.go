package backend

import (
	"context"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"net/http"
)

var _ Adapter = (*OpenAIStyleBackend)(nil)

func NewOpenAIStyleBackend(name string, m map[string]string, httpclient *http.Client) (*OpenAIStyleBackend, error) {
	var config openai.ClientConfig
	if apiToken, has := m["api_token"]; has {
		config = openai.DefaultConfig(apiToken)
	} else {
		return nil, fmt.Errorf("%s key `api_token` not found", name)
	}

	if base_url, has := m["api_base"]; has {
		config.BaseURL = base_url
	} else {
		return nil, fmt.Errorf("%s key `api_token` not found", name)
	}

	if httpclient != nil {
		config.HTTPClient = &http.Client{}
	}
	return &OpenAIStyleBackend{name: name, client: openai.NewClientWithConfig(config)}, nil
}

// OpenAIStyleBackend openai风格api
type OpenAIStyleBackend struct {
	name   string
	client *openai.Client
}

func (o *OpenAIStyleBackend) Name() string {
	return o.name
}

func (o *OpenAIStyleBackend) Models(ctx context.Context) (openai.ModelsList, error) {
	return o.client.ListModels(ctx)
}

func (o *OpenAIStyleBackend) ChatCompletions(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return o.client.CreateChatCompletion(ctx, request)
}

func (o *OpenAIStyleBackend) ChatCompletionsStreaming(ctx context.Context, request openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return o.client.CreateChatCompletionStream(ctx, request)
}

func (o *OpenAIStyleBackend) AudioSpeech(ctx context.Context, request openai.CreateSpeechRequest) (openai.RawResponse, error) {
	return o.client.CreateSpeech(ctx, request)
}

func (o *OpenAIStyleBackend) AudioTranscriptions(ctx context.Context, request openai.AudioRequest) (openai.AudioResponse, error) {
	return o.client.CreateTranscription(ctx, request)
}

func (o *OpenAIStyleBackend) ImagesGenerations(ctx context.Context, request openai.ImageRequest) (openai.ImageResponse, error) {
	return o.client.CreateImage(ctx, request)
}
