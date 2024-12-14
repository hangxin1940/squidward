package backend

import (
	"context"
	"errors"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"path/filepath"
	"squidward/lib"
	"testing"
)

func TestOpenAIStyleBackend_Models(t *testing.T) {
	client, _ := NewOpenAIStyleBackend(&AdapterConfig{
		Name:         "ollama",
		DefaultModel: "gemma2:9b",
		Type:         ModelTypeLLM,
		ApiStyle:     "openai",
		ApiBase:      "http://127.0.0.1:1234/v1/",
		ApiToken:     "123456",
	})
	models, err := client.Models(context.TODO())
	assert.Empty(t, err)
	for _, model := range models.Models {
		fmt.Println(model.ID)
	}
}

func TestOpenAIStyleBackend_ChatCompletions(t *testing.T) {
	client, _ := NewOpenAIStyleBackend(&AdapterConfig{
		Name:         "ollama",
		DefaultModel: "gemma2:9b",
		Type:         ModelTypeLLM,
		ApiStyle:     "openai",
		ApiBase:      "http://127.0.0.1:1234/v1/",
		ApiToken:     "123456",
	})
	seed := -9007199254740991
	res, err := client.ChatCompletions(context.TODO(), openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{
				Content: "你好",
				Role:    openai.ChatMessageRoleSystem,
				Name:    "张三",
			},
		},
		Model:            "gemma2:9b",
		FrequencyPenalty: -2.000000,
		MaxTokens:        0,
		PresencePenalty:  -2.000000,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeText,
		},
		Seed:        &seed,
		Temperature: 1.000000,
		TopP:        1.000000,
	})
	assert.Empty(t, err)
	for _, choice := range res.Choices {
		fmt.Println(choice.Message.Role, ":", choice.Message.Content)
	}
	fmt.Println("OK!")
}

func TestOpenAIStyleBackend_ChatCompletionsStreaming(t *testing.T) {
	client, _ := NewOpenAIStyleBackend(&AdapterConfig{
		Name:         "ollama",
		DefaultModel: "gemma2:9b",
		Type:         ModelTypeLLM,
		ApiStyle:     "openai",
		ApiBase:      "http://127.0.0.1:1234/v1/",
		ApiToken:     "123456",
	})
	seed := -9007199254740991
	stream, err := client.ChatCompletionsStreaming(context.TODO(), openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{
				Content: "你好,介绍一下自己",
				Role:    openai.ChatMessageRoleSystem,
				Name:    "张三",
			},
		},
		Model:            "gemma2:9b",
		FrequencyPenalty: -2.000000,
		MaxTokens:        0,
		PresencePenalty:  -2.000000,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeText,
		},
		Seed:        &seed,
		Temperature: 1.000000,
		TopP:        1.000000,
	})

	assert.Empty(t, err)
	defer stream.Close()

	roleprint := false
	for {
		res, rerr := stream.Recv()
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				break
			}
			continue
		}

		for _, choice := range res.Choices {
			if !roleprint {
				fmt.Print(choice.Delta.Role + ": ")
				roleprint = true
			}
			fmt.Print(choice.Delta.Content)
		}
	}
	fmt.Println()
	fmt.Println("OK!")
}

func TestOpenAIStyleBackend_AudioSpeech(t *testing.T) {
	client, _ := NewOpenAIStyleBackend(&AdapterConfig{
		Name:         "ollama",
		DefaultModel: "gemma2:9b",
		Type:         ModelTypeTTS,
		ApiStyle:     "openai",
		ApiBase:      "http://127.0.0.1:1234/v1/",
		ApiToken:     "123456",
	})
	res, err := client.AudioSpeech(context.TODO(), openai.CreateSpeechRequest{
		Input:          "Hello, my name is Musk.",
		Model:          openai.TTSModel1,
		Voice:          openai.VoiceAlloy,
		ResponseFormat: openai.SpeechResponseFormatMp3,
		Speed:          1,
	})

	assert.Empty(t, err)

	rpath := filepath.Join(lib.RuntimeDir(), "../", "tmp")
	os.MkdirAll(rpath, os.ModePerm)

	outFile, err := os.Create(filepath.Join(rpath, "test.mp3"))
	assert.Empty(t, err)
	// handle err
	defer outFile.Close()
	_, err = io.Copy(outFile, res)

	fmt.Println(res.Header())
}

func TestOpenAIStyleBackend_AudioTranscriptions(t *testing.T) {

	rpath := filepath.Join(lib.RuntimeDir(), "../", "tmp")

	fs, err := os.Open(filepath.Join(rpath, "audio.wav"))
	assert.Empty(t, err)
	defer fs.Close()

	client, _ := NewOpenAIStyleBackend(&AdapterConfig{
		Name:         "ollama",
		DefaultModel: "gemma2:9b",
		Type:         ModelTypeSTT,
		ApiStyle:     "openai",
		ApiBase:      "http://127.0.0.1:1234/v1/",
		ApiToken:     "123456",
	})
	res, err := client.AudioTranscriptions(context.TODO(), openai.AudioRequest{
		Reader:      fs,
		FilePath:    "a.wav",
		Model:       openai.Whisper1,
		Language:    "zh",
		Format:      openai.AudioResponseFormatJSON,
		Temperature: 0.000000,
	})
	assert.Empty(t, err)

	fmt.Println(res.Text)
}

func TestOpenAIStyleBackend_ImagesGenerations(t *testing.T) {

	client, _ := NewOpenAIStyleBackend(&AdapterConfig{
		Name:         "ollama",
		DefaultModel: "gemma2:9b",
		Type:         ModelTypeTTS,
		ApiStyle:     "openai",
		ApiBase:      "http://127.0.0.1:1234/v1/",
		ApiToken:     "123456",
	})
	res, err := client.ImagesGenerations(context.TODO(), openai.ImageRequest{
		Prompt:         "章鱼哥",
		Model:          openai.CreateImageModelDallE3,
		N:              1,
		Quality:        openai.CreateImageQualityHD,
		Size:           openai.CreateImageSize1024x1024,
		Style:          openai.CreateImageStyleVivid,
		ResponseFormat: openai.CreateImageResponseFormatURL,
		User:           "user",
	})
	assert.Empty(t, err)

	for _, iu := range res.Data {
		fmt.Println(iu.URL)
	}
}
