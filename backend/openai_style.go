package backend

import (
	"context"
	"github.com/sashabaranov/go-openai"
	"net/http"
	"net/url"
	"time"
)

var _ Adapter = (*OpenAIStyleBackend)(nil)

func NewOpenAIStyleBackend(cfg *AdapterConfig) (*OpenAIStyleBackend, error) {
	config := openai.DefaultConfig(cfg.ApiToken)
	config.BaseURL = cfg.ApiBase
	if cfg.HttpTimeout == 0 {
		cfg.HttpTimeout = 10 * time.Second
	}

	httpClient := &http.Client{
		Timeout: cfg.HttpTimeout,
	}

	if cfg.HttpProxy != "" {
		pu, err := url.Parse(cfg.HttpProxy)
		if err != nil {
			return nil, err
		}
		httpClient.Transport = &http.Transport{Proxy: http.ProxyURL(pu)}
	}

	config.HTTPClient = httpClient

	return &OpenAIStyleBackend{name: cfg.Name, modelType: cfg.Type, client: openai.NewClientWithConfig(config)}, nil
}

// OpenAIStyleBackend openai风格api
type OpenAIStyleBackend struct {
	name      string
	modelType ModelType
	client    *openai.Client
}

func (o *OpenAIStyleBackend) Type() ModelType {
	return o.modelType
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
