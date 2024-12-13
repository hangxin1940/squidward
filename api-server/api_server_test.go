package api_server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"squidward/backend"
	"squidward/lib"
	"testing"
)

func _initApiServer() *ApiServer {
	logger := lib.NewLogger(6, "test", 9)

	cofnig := map[string]string{
		"name":      "ollama",
		"type":      "openai",
		"api_base":  "http://127.0.0.1:1234/v1/",
		"api_token": "123456",
	}
	bkOpenai, err := backend.NewOpenAIStyleBackend("Ollama", cofnig, nil)
	if err != nil {
		panic(err)
	}

	aServcie := &backend.AdapterService{}
	aServcie.SetLLMBackend(bkOpenai)
	aServcie.SetTTSBackend(bkOpenai)
	aServcie.SetSTTBackend(bkOpenai)
	aServcie.SetImageBackend(bkOpenai)

	return &ApiServer{
		logger:   logger,
		apiBase:  "/v1",
		aService: aServcie,
	}
}

func TestApiServer_Models(t *testing.T) {
	mserver := _initApiServer()
	router := mserver.SetupRouter()

	w := httptest.NewRecorder()

	req, _ := http.NewRequest("GET", "/v1/models", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	fmt.Println(w.Body.String())
}

func TestApiServer_ChatCompletions(t *testing.T) {

	mserver := _initApiServer()
	router := mserver.SetupRouter()

	w := httptest.NewRecorder()

	seed := -9007199254740991
	body := openai.ChatCompletionRequest{
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
	}

	jsondata, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsondata))
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	fmt.Println(w.Body.String())

}

type TestResponseRecorder struct {
	*httptest.ResponseRecorder
	closeChannel chan bool
}

func (r *TestResponseRecorder) CloseNotify() <-chan bool {
	return r.closeChannel
}

func (r *TestResponseRecorder) closeClient() {
	r.closeChannel <- true
}

func CreateTestResponseRecorder() *TestResponseRecorder {
	return &TestResponseRecorder{
		httptest.NewRecorder(),
		make(chan bool, 1),
	}
}

func TestApiServer_ChatCompletionsStreaming(t *testing.T) {

	mserver := _initApiServer()
	router := mserver.SetupRouter()

	w := CreateTestResponseRecorder()

	seed := -9007199254740991
	body := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{
				Content: "你好, 简单介绍一下自己。",
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
		Stream:      true,
	}

	jsondata, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsondata))
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	fmt.Println(w.Body.String())

}

func TestApiServer_AudioSpeech(t *testing.T) {

	mserver := _initApiServer()
	router := mserver.SetupRouter()

	w := httptest.NewRecorder()

	body := openai.CreateSpeechRequest{
		Input:          "你好，我是章鱼哥",
		Model:          openai.TTSModel1,
		Voice:          openai.VoiceAlloy,
		ResponseFormat: openai.SpeechResponseFormatMp3,
		Speed:          0.8,
	}

	jsondata, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/v1/audio/speech", bytes.NewReader(jsondata))
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	rpath := filepath.Join(lib.RuntimeDir(), "../", "tmp")
	os.MkdirAll(rpath, os.ModePerm)

	outFile, err := os.Create(filepath.Join(rpath, "test111.mp3"))
	assert.Empty(t, err)
	defer outFile.Close()
	_, err = io.Copy(outFile, w.Body)

	fmt.Println(w.Header())

}

func TestApiServer_AudioTranscriptions(t *testing.T) {

	mserver := _initApiServer()
	router := mserver.SetupRouter()

	rpath := filepath.Join(lib.RuntimeDir(), "../", "tmp")

	extraParams := map[string][]string{
		"model":           {openai.Whisper1},
		"prompt":          {"提示词"},
		"response_format": {string(openai.AudioResponseFormatJSON)},
		"temperature":     {fmt.Sprintf("%.2f", 0.000000)},
		"language":        {"zh"},
		//"timestamp_granularities": {"word", "segment"},
	}

	req, err := _newfileUploadRequest("/v1/audio/transcriptions", extraParams, "file", filepath.Join(rpath, "hhll.wav"))
	assert.Empty(t, err)

	rc := httptest.NewRecorder()
	router.ServeHTTP(rc, req)
	assert.Equal(t, 200, rc.Code)
}

func _newfileUploadRequest(uri string, params map[string][]string, paramName, path string) (*http.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for key, val := range params {
		if len(val) > 1 {
			for _, v := range val {
				_ = writer.WriteField(key+"[]", v)
			}
		} else {
			_ = writer.WriteField(key, val[0])
		}
	}

	part, err := writer.CreateFormFile(paramName, filepath.Base(path))
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", uri, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, err
}

func TestApiServer_ImagesGenerations(t *testing.T) {

	mserver := _initApiServer()
	router := mserver.SetupRouter()

	w := httptest.NewRecorder()

	body := openai.ImageRequest{
		Prompt:         "章鱼哥抱着海绵宝宝",
		Model:          openai.CreateImageModelDallE3,
		N:              1,
		Quality:        openai.CreateImageQualityHD,
		Size:           openai.CreateImageSize1024x1024,
		Style:          openai.CreateImageStyleVivid,
		ResponseFormat: openai.CreateImageResponseFormatURL,
		User:           "user",
	}

	jsondata, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/v1/images/generations", bytes.NewReader(jsondata))
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	fmt.Println(w.Body.String())

}
