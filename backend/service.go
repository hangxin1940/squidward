package backend

import (
	"context"
	"github.com/sashabaranov/go-openai"
	"time"
)

type ModelType string

var (
	ModelTypeLLM   ModelType = "llm"
	ModelTypeSTT   ModelType = "stt"
	ModelTypeTTS   ModelType = "tts"
	ModelTypeImage ModelType = "image"
)

type AdapterConfig struct {
	Name         string                 `mapstructure:"name"`
	DefaultModel string                 `mapstructure:"default_model"`
	Type         ModelType              `mapstructure:"type"`
	ApiBase      string                 `mapstructure:"api_base"`
	ApiStyle     string                 `mapstructure:"api_style"`
	ApiToken     string                 `mapstructure:"api_token,omitempty"`
	HttpTimeout  time.Duration          `mapstructure:"http_timeout,omitempty"`
	HttpProxy    string                 `mapstructure:"http_proxy,omitempty"`
	Extras       map[string]interface{} `mapstructure:",remain"`
}

// Adapter 后端适配器
type Adapter interface {
	Name() string
	Type() ModelType
	Models(context.Context) (openai.ModelsList, error)
	ChatCompletions(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
	ChatCompletionsStreaming(context.Context, openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error)
	AudioSpeech(context.Context, openai.CreateSpeechRequest) (openai.RawResponse, error)
	AudioTranscriptions(context.Context, openai.AudioRequest) (openai.AudioResponse, error)
	ImagesGenerations(context.Context, openai.ImageRequest) (openai.ImageResponse, error)
}

// AdapterService 后端适配服务
type AdapterService struct {
	// 推理服务
	llm Adapter
	// STT服务
	stt Adapter
	// TTS服务
	tts Adapter
	// 图像服务
	image Adapter
}

func (s *AdapterService) SetBackend(a Adapter) {
	switch a.Type() {
	case ModelTypeLLM:
		s.llm = a
	case ModelTypeTTS:
		s.tts = a
	case ModelTypeSTT:
		s.stt = a
	case ModelTypeImage:
		s.image = a
	}
}

func (s *AdapterService) GetBackend(t ModelType) Adapter {
	switch t {
	case ModelTypeLLM:
		return s.llm
	case ModelTypeTTS:
		return s.tts
	case ModelTypeSTT:
		return s.stt
	case ModelTypeImage:
		return s.image
	}
	return nil
}
