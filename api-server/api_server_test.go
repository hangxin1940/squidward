package api_server

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"slices"
	"squidward/backend"
	"squidward/lib"
	"squidward/modules/audio"
	"strconv"
	"strings"
	"testing"
)

func _initApiServer() *ApiServer {
	logger := lib.NewLogger(6, "test", 9)

	bkLLM, err := backend.NewOpenAIStyleBackend(&backend.AdapterConfig{
		Name:         "ollama",
		DefaultModel: "gemma2:9b",
		Type:         backend.ModelTypeLLM,
		ApiStyle:     "openai",
		ApiBase:      "http://127.0.0.1:1234/v1/",
		ApiToken:     "123456",
	})

	bkTTS, err := backend.NewOpenAIStyleBackend(&backend.AdapterConfig{
		Name:         "ollama",
		DefaultModel: "tts-1",
		Type:         backend.ModelTypeTTS,
		ApiStyle:     "openai",
		ApiBase:      "http://127.0.0.1:1234/v1/",
		ApiToken:     "123456",
	})

	bkSTT, err := backend.NewOpenAIStyleBackend(&backend.AdapterConfig{
		Name:         "ollama",
		DefaultModel: openai.Whisper1,
		Type:         backend.ModelTypeSTT,
		ApiStyle:     "openai",
		ApiBase:      "https://api.openai.com/v1/",
		ApiToken:     "123456",
	})

	bkIMG, err := backend.NewOpenAIStyleBackend(&backend.AdapterConfig{
		Name:         "ollama",
		DefaultModel: "gemma2:9b",
		Type:         backend.ModelTypeImage,
		ApiStyle:     "openai",
		ApiBase:      "http://127.0.0.1:1234/v1/",
		ApiToken:     "123456",
	})
	if err != nil {
		panic(err)
	}

	aServcie := &backend.AdapterService{}
	aServcie.SetBackend(bkLLM)
	aServcie.SetBackend(bkTTS)
	aServcie.SetBackend(bkSTT)
	aServcie.SetBackend(bkIMG)

	return &ApiServer{
		logger:      logger,
		apiBase:     "/v1",
		aService:    aServcie,
		audioFrames: map[string]*audio.Audio{},
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

func TestApiServer_AudioTTS(t *testing.T) {

	mserver := _initApiServer()
	router := mserver.SetupRouter()

	w := httptest.NewRecorder()

	req, _ := http.NewRequest("GET", "/v1/audio/tts", nil)

	q := req.URL.Query()
	q.Add("input", "你好，我是章鱼哥")
	q.Add("voice", "fable")
	//q.Add("response_format", "wav")
	//q.Add("speed", "0.8")
	req.URL.RawQuery = q.Encode()

	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	rpath := filepath.Join(lib.RuntimeDir(), "../", "tmp")
	os.MkdirAll(rpath, os.ModePerm)

	outFile, err := os.Create(filepath.Join(rpath, "test22.wav"))
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

func TestApiServer_sampleServer(t *testing.T) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		js := `{
  "task" : "",
  "language" : "",
  "duration" : 0,
  "segments" : null,
  "words" : null,
  "text" : "一加二等于几?"
}`
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, js)
	})

	err := http.ListenAndServe(fmt.Sprintf(":%d", 12345), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		fmt.Println()
		for k, h := range r.Header {
			fmt.Printf("%s: %s\n", k, strings.Join(h, "; "))
		}
		fmt.Println()
		buf := new(strings.Builder)
		io.Copy(buf, r.Body)
		// check errors
		fmt.Println(buf.String())
		fmt.Println("------------------------")

		http.DefaultServeMux.ServeHTTP(w, r)
	}))
	if err != nil {
		log.Fatal(err)
	}
}

func TestApiServer_saveAudioFrames(t *testing.T) {
	http.HandleFunc("/v1/audio/transcriptions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			r.ParseMultipartForm(32 << 20)
			fmt.Println("model:", r.FormValue("model"))
			fmt.Println("prompt:", r.FormValue("prompt"))
			fmt.Println("response_format:", r.FormValue("response_format"))
			fmt.Println("temperature:", r.FormValue("temperature"))
			fmt.Println("language:", r.FormValue("language"))
			fmt.Println("timestamp_granularities:", r.FormValue("timestamp_granularities"))
			fmt.Println("audio_mime:", r.FormValue("audio_mime"))
			fmt.Println("audio_id:", r.FormValue("audio_id"))
			fmt.Println("is_finish:", r.FormValue("is_finish"))
			fmt.Println("is_frame:", r.FormValue("is_frame"))
			fmt.Println("frame_index:", r.FormValue("frame_index"))
			f, h, _ := r.FormFile("file")
			fmt.Println("file", h.Filename)

			rpath := filepath.Join(lib.RuntimeDir(), "../", "tmp", fmt.Sprintf("audioframe_%s", r.FormValue("audio_id")))
			os.MkdirAll(rpath, os.ModePerm)

			outFile, _ := os.Create(filepath.Join(rpath, fmt.Sprintf("%s_%s", r.FormValue("frame_index"), h.Filename)))

			// handle err
			defer outFile.Close()
			_, _ = io.Copy(outFile, f)

		}

		fmt.Fprintf(w, "Hello, %s!", r.URL.Path[1:])
	})

	err := http.ListenAndServe(fmt.Sprintf(":%d", 12345), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		fmt.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		//for k, h := range r.Header {
		//	fmt.Printf("\t%s: %s\n", k, strings.Join(h, "; "))
		//}
		fmt.Println()

		http.DefaultServeMux.ServeHTTP(w, r)
	}))
	if err != nil {
		log.Fatal(err)
	}
}

func TestApiServer_AudioTranscriptionsFrame(t *testing.T) {

	mserver := _initApiServer()
	router := mserver.SetupRouter()
	id := "1"

	rpath := filepath.Join(lib.RuntimeDir(), "../", "tmp", fmt.Sprintf("audioframe_%s", id))
	entries, err := os.ReadDir(rpath)
	if err != nil {
		panic(err)
	}

	files := []string{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fns := strings.Split(entry.Name(), "_")
		if len(fns) != 2 {
			continue
		}

		_, erra := strconv.Atoi(fns[0])
		if erra != nil {
			continue
		}

		files = append(files, entry.Name())
	}

	files = slices.SortedFunc(slices.Values(files), func(m1, m2 string) int {
		fns1 := strings.Split(m1, "_")
		fns2 := strings.Split(m2, "_")

		i1, _ := strconv.Atoi(fns1[0])
		i2, _ := strconv.Atoi(fns2[0])
		return cmp.Compare(i1, i2)
	})

	for i, f := range files {
		extraParams := map[string][]string{
			"model":       {openai.Whisper1},
			"audio_id":    {id},
			"frame_index": {fmt.Sprintf("%d", i)},
			"is_finish":   {"0"},
			"is_frame":    {"1"},
			"audio_mime":  {"audio/L16;rate=8000"},
		}

		if i == len(files)-1 {
			extraParams["is_finish"] = []string{"1"}
		}

		req, err := _newfileUploadRequest("/v1/audio/transcriptions", extraParams, "file", path.Join(rpath, f))
		assert.Empty(t, err)

		rc := httptest.NewRecorder()
		router.ServeHTTP(rc, req)
		assert.Equal(t, 200, rc.Code)

		if i == len(files)-1 {
			fmt.Println(rc.Body.String())
		}
	}

}
