package backend

import (
	"context"
	"github.com/sashabaranov/go-openai"
)

type AdapterConfig = func(Adapter) error

// Adapter 后端适配器
type Adapter interface {
	Name() string
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

func (s *AdapterService) SetLLMBackend(a Adapter) {
	s.llm = a
}

func (s *AdapterService) GetLLMBackend() Adapter {
	return s.llm
}

func (s *AdapterService) SetSTTBackend(a Adapter) {
	s.stt = a
}

func (s *AdapterService) GetSTTBackend() Adapter {
	return s.stt
}

func (s *AdapterService) SetTTSBackend(a Adapter) {
	s.tts = a
}

func (s *AdapterService) GetTTSBackend() Adapter {
	return s.tts
}

func (s *AdapterService) SetImageBackend(a Adapter) {
	s.image = a
}

func (s *AdapterService) GetImageBackend() Adapter {
	return s.image
}
