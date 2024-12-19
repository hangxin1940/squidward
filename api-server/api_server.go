package api_server

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"slices"
	"squidward/backend"
	"squidward/modules/audio"
	"strconv"
	"strings"
)

func NewApiServer(logger *logrus.Logger, aService *backend.AdapterService) *ApiServer {
	return &ApiServer{
		logger:      logger,
		apiBase:     "/v1",
		aService:    aService,
		audioFrames: map[string]*audio.Audio{},
	}
}

type ApiServer struct {
	logger      *logrus.Logger
	apiBase     string
	netListener net.Listener
	aService    *backend.AdapterService

	audioFrames map[string]*audio.Audio // TODO 线程安全
}

func (s *ApiServer) Serve(netListener net.Listener) error {
	s.netListener = netListener
	router := s.SetupRouter()
	s.logger.WithField("prefix", "Serve").Infof("%s%s", netListener.Addr(), s.apiBase)

	return http.Serve(netListener, router)
}

// MiddlePanic 异常恢复
func (s *ApiServer) MiddlePanic(c *gin.Context, recovered interface{}) {
	if err, ok := recovered.(string); ok {
		s.logger.WithField("prefix", "PANIC").Error(err)
	}
	// TODO
}

// SetupRouter 装载路由
func (s *ApiServer) SetupRouter() *gin.Engine {
	router := gin.New()
	router.RedirectTrailingSlash = true
	router.Use(gin.Logger())
	// 异常恢复
	router.Use(gin.CustomRecovery(s.MiddlePanic))

	// TODO 添加认证

	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "*")
		c.Header("Access-Control-Expose-Headers", "*")
		c.Header("Allow", "HEAD,GET,POST,PUT,PATCH,DELETE,OPTIONS")
		if c.Request.Method != "OPTIONS" {
			c.Next()
		} else {
			c.AbortWithStatus(http.StatusOK)
		}
	})

	apiRouter := router.Group(s.apiBase)
	{
		apiRouter.POST("/chat/completions", s.chatCompletions)
		apiRouter.POST("/images/generations", s.imagesGenerations)
		apiRouter.POST("/audio/speech", s.audioSpeech)
		apiRouter.GET("/audio/speech", s.audioSpeech)
		apiRouter.POST("/audio/transcriptions", s.audioTranscriptions)
		apiRouter.GET("/audio/transcriptions/ws", s.wsAudioTranscriptions)
		apiRouter.GET("/models", s.models)
	}

	return router
}

func (s *ApiServer) Stop() {
	_ = s.netListener.Close()
}

type chatCompletionStreamChoice struct {
	Index        int                                    `json:"index"`
	Delta        openai.ChatCompletionStreamChoiceDelta `json:"delta"`
	FinishReason openai.FinishReason                    `json:"finish_reason"`
}

type chatCompletionStreamResponse struct {
	Choices []chatCompletionStreamChoice `json:"choices"`
}

type chatCompletionChoice struct {
	Message openai.ChatCompletionMessage `json:"message"`
}

type chatCompletionResponse struct {
	Choices []chatCompletionChoice `json:"choices"`
}

// chatCompletions 聊天
func (s *ApiServer) chatCompletions(c *gin.Context) {
	bk := s.aService.GetBackend(backend.ModelTypeLLM)
	if bk == nil {
		s.logger.Error("未配置LLM")
		c.Status(http.StatusInternalServerError)
		return
	}

	req := openai.ChatCompletionRequest{}

	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	if req.Stream {
		c.Stream(func(w io.Writer) bool {
			res, err := bk.ChatCompletionsStreaming(context.Background(), req)
			if err != nil {
				s.logger.Error(err)
				return false
			}
			for {
				streamObj, rerr := res.Recv()
				if rerr != nil {
					if errors.Is(rerr, io.EOF) {
						return false
					}
					return true
				}

				stream := chatCompletionStreamResponse{
					Choices: []chatCompletionStreamChoice{},
				}
				for _, ch := range streamObj.Choices {
					stream.Choices = append(stream.Choices, chatCompletionStreamChoice{
						Index:        ch.Index,
						Delta:        ch.Delta,
						FinishReason: ch.FinishReason,
					})
				}

				bs, _ := json.Marshal(stream)

				line := "data: " + string(bs) + "\n\n"
				_, _ = w.Write([]byte(line))
			}
		})
	} else {
		res, err := bk.ChatCompletions(context.Background(), req)
		if err != nil {
			s.logger.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}

		res1 := chatCompletionResponse{
			Choices: []chatCompletionChoice{},
		}
		for _, ch := range res.Choices {
			res1.Choices = append(res1.Choices, chatCompletionChoice{
				Message: ch.Message,
			})
		}

		c.JSON(200, res1)
	}

}

// imagesGenerations 创建图像
func (s *ApiServer) imagesGenerations(c *gin.Context) {
	bk := s.aService.GetBackend(backend.ModelTypeImage)
	if bk == nil {
		s.logger.Error("未配置图像服务")
		c.Status(http.StatusInternalServerError)
		return
	}

	req := openai.ImageRequest{}

	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	res, err := bk.ImagesGenerations(context.Background(), req)
	if err != nil {
		s.logger.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(200, res)
}

// audioSpeech TTS
func (s *ApiServer) audioSpeech(c *gin.Context) {
	bk := s.aService.GetBackend(backend.ModelTypeTTS)
	if bk == nil {
		s.logger.Error("未配置TTS")
		c.Status(http.StatusInternalServerError)
		return
	}

	req := openai.CreateSpeechRequest{}

	asfile := false

	if c.Request.Method == http.MethodPost {
		if err := c.ShouldBindBodyWithJSON(&req); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
	} else {
		input := strings.TrimSpace(c.Query("input"))
		if input == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		format := strings.TrimSpace(c.Query("response_format"))
		voice := strings.TrimSpace(c.Query("voice"))
		speedStr := strings.TrimSpace(c.Query("speed"))
		asfile = strings.TrimSpace(c.Query("file")) != ""
		speed := float64(0)
		if speedStr != "" {
			if speedN, err := strconv.ParseFloat(speedStr, 64); err == nil {
				speed = speedN
			}
		}

		req.Input = input
		req.ResponseFormat = openai.SpeechResponseFormat(format)
		req.Speed = speed
		req.Voice = openai.SpeechVoice(voice)
	}

	res, err := bk.AudioSpeech(context.Background(), req)
	if err != nil {
		s.logger.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	length := int64(0)
	lengthStr := strings.TrimSpace(res.Header().Get("Content-Length"))
	if lengthStr == "" && !asfile {
		c.Header("Transfer-Encoding", "chunked")
	} else {
		length, _ = strconv.ParseInt(res.Header().Get("Content-Length"), 10, 64)
	}

	if length > 0 || asfile {
		alldata, _ := io.ReadAll(res)
		reader := bytes.NewReader(alldata)
		c.DataFromReader(http.StatusOK, int64(reader.Len()), res.Header().Get("Content-Type"), reader, nil)
	} else {
		w := c.Writer
		header := w.Header()
		header.Set("Transfer-Encoding", "chunked")
		header.Set("Content-Type", res.Header().Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		var buf [85000]byte
		for {
			n, errn := res.Read(buf[0:])
			if errn != nil {
				if errn != io.EOF {
					s.logger.Warnf("read error: %v", errn)
				}
				break
			}
			w.Write(buf[0:n])
		}

		w.(http.Flusher).Flush()
	}

}

// audioTranscriptions STT
func (s *ApiServer) audioTranscriptions(c *gin.Context) {
	bk := s.aService.GetBackend(backend.ModelTypeSTT)
	if bk == nil {
		s.logger.Error("未配置STT")
		c.Status(http.StatusInternalServerError)
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	if _, hasf := form.File["file"]; !hasf {
		c.Status(http.StatusBadRequest)
		return
	}

	if _, has := form.Value["is_frame"]; has {
		// 非完整音频，需要整合

		if _, hasf := form.Value["audio_id"]; !hasf {
			c.Status(http.StatusBadRequest)
			return
		}
		if _, hasf := form.Value["audio_mime"]; !hasf {
			c.Status(http.StatusBadRequest)
			return
		}

		if mimeok := audio.CheckMimeValid(form.Value["audio_mime"][0]); !mimeok {
			c.Status(http.StatusBadRequest)
			return
		}

		if _, hasf := form.Value["frame_index"]; !hasf {
			c.Status(http.StatusBadRequest)
			return
		}
		if _, hasf := form.Value["is_finish"]; !hasf {
			c.Status(http.StatusBadRequest)
			return
		}
		if _, errf := strconv.Atoi(form.Value["frame_index"][0]); errf != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		req, errf := s.audioTranscriptionsFrame(form)
		if errf != nil {
			s.logger.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}

		if req != nil {
			if req.Language == "" {
				req.Language = "zh"
			}
			res, err := bk.AudioTranscriptions(context.Background(), *req)
			if err != nil {
				s.logger.Error(err)
				c.Status(http.StatusInternalServerError)
				return
			}
			c.JSON(http.StatusOK, res)
		} else {
			c.Status(http.StatusOK)
		}
		return
	}

	req := openai.AudioRequest{}
	if _, hasf := form.File["model"]; hasf {
		req.Model = form.Value["model"][0]
	}

	audio := form.File["file"]
	fs, err := audio[0].Open()
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	req.Reader = fs
	req.FilePath = audio[0].Filename

	if lang, has := form.Value["language"]; has {
		req.Language = lang[0]
	}

	if prompt, has := form.Value["prompt"]; has {
		req.Prompt = prompt[0]
	}

	if temperature, has := form.Value["temperature"]; has {
		if ti, erri := strconv.ParseFloat(temperature[0], 32); erri == nil {
			req.Temperature = float32(ti)
		}
	}

	if format, has := form.Value["response_format"]; has {
		req.Format = openai.AudioResponseFormat(format[0])
	}

	if tgs, has := form.Value["timestamp_granularities[]"]; has {
		req.TimestampGranularities = []openai.TranscriptionTimestampGranularity{}
		for _, tg := range tgs {
			req.TimestampGranularities = append(req.TimestampGranularities, openai.TranscriptionTimestampGranularity(tg))
		}
	}

	res, err := bk.AudioTranscriptions(context.Background(), req)
	if err != nil {
		s.logger.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, res)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type audioFrame struct {
	FrameIndex int    `json:"frame_index"`
	IsFinish   int    `json:"is_finish"`
	Data       string `json:"data"`
}

// wsAudioTranscriptions STT websocket
func (s *ApiServer) wsAudioTranscriptions(c *gin.Context) {
	bk := s.aService.GetBackend(backend.ModelTypeSTT)
	if bk == nil {
		s.logger.Error("未配置STT")
		c.Status(http.StatusInternalServerError)
		return
	}

	file_name := c.Query("file")
	model := c.Query("model")
	audio_id := c.Query("audio_id")
	audio_mime := c.Query("audio_mime")
	language := c.Query("language")
	prompt := c.Query("prompt")
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		s.logger.Debugln(err.Error())
		c.Status(http.StatusBadRequest)
		return
	}
	defer conn.Close()
	for {
		var data audioFrame
		if errc := conn.ReadJSON(&data); errc != nil {
			s.logger.Error(errc)
			break
		}
		s.logger.Tracef("audio frame %s %d", audio_id, data.FrameIndex)

		af := s.audioFrames[audio_id]
		if af == nil {
			af = audio.NewAudio(audio_mime)
			s.audioFrames[audio_id] = af
		}
		bdata, errb := base64.StdEncoding.DecodeString(data.Data)
		if errb != nil {
			break
		}
		af.AddFrame(data.FrameIndex, bdata)

		if data.IsFinish == 1 {
			defer delete(s.audioFrames, audio_id)

			req := openai.AudioRequest{
				Model:    model,
				Reader:   af.ToAudioBytesReader(),
				FilePath: file_name,
				Language: language,
				Prompt:   prompt,
			}
			s.logger.Tracef("audio %s send stt...", audio_id)
			res, errt := bk.AudioTranscriptions(context.Background(), req)
			if errt != nil {
				s.logger.Error(errt)
				return
			}
			s.logger.Tracef("audio %s: %s", audio_id, res.Text)
			err = conn.WriteMessage(websocket.TextMessage, []byte(res.Text))
			if err != nil {
				s.logger.Error(err)
				return
			}
		}
	}
}

func (s *ApiServer) audioTranscriptionsFrame(form *multipart.Form) (*openai.AudioRequest, error) {
	id := form.Value["audio_id"][0]
	mime := form.Value["audio_mime"][0]
	finished := form.Value["is_finish"][0] == "1"
	index, _ := strconv.Atoi(form.Value["frame_index"][0])

	af := s.audioFrames[id]
	if af == nil {
		af = audio.NewAudio(mime)
		s.audioFrames[id] = af
	}

	afile := form.File["file"][0]
	content, _ := afile.Open()

	bs, _ := io.ReadAll(content)

	af.AddFrame(index, bs)

	if finished {
		defer delete(s.audioFrames, id)

		req := openai.AudioRequest{}

		if _, hasf := form.File["model"]; hasf {
			req.Model = form.Value["model"][0]
		}
		req.Reader = af.ToAudioBytesReader()

		req.FilePath = afile.Filename

		if lang, has := form.Value["language"]; has {
			req.Language = lang[0]
		}

		if prompt, has := form.Value["prompt"]; has {
			req.Prompt = prompt[0]
		}

		if temperature, has := form.Value["temperature"]; has {
			if ti, erri := strconv.ParseFloat(temperature[0], 32); erri == nil {
				req.Temperature = float32(ti)
			}
		}

		if format, has := form.Value["response_format"]; has {
			req.Format = openai.AudioResponseFormat(format[0])
		}

		if tgs, has := form.Value["timestamp_granularities[]"]; has {
			req.TimestampGranularities = []openai.TranscriptionTimestampGranularity{}
			for _, tg := range tgs {
				req.TimestampGranularities = append(req.TimestampGranularities, openai.TranscriptionTimestampGranularity(tg))
			}
		}

		return &req, nil
	}
	return nil, nil

}

type Model struct {
	openai.Model
	BackendName string            `json:"backend_name"`
	BackendType backend.ModelType `json:"backend_type"`
}

func (m Model) String() string {
	jstr, _ := json.Marshal(&m)
	return string(jstr)
}

type ModelsList struct {
	Models []Model `json:"data"`
}

// models 列出模型
func (s *ApiServer) models(c *gin.Context) {
	resp := &ModelsList{}
	defer c.JSON(http.StatusOK, resp)

	var allModels []Model

	if bk := s.aService.GetBackend(backend.ModelTypeLLM); bk != nil {
		models, err := bk.Models(context.Background())
		if err != nil {
			s.logger.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}

		for _, m := range models.Models {
			model := Model{
				Model:       m,
				BackendName: bk.Name(),
			}
			allModels = append(allModels, model)
		}

	}

	if bk := s.aService.GetBackend(backend.ModelTypeTTS); bk != nil {
		models, err := bk.Models(context.Background())
		if err != nil {
			s.logger.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}

		for _, m := range models.Models {
			model := Model{
				Model:       m,
				BackendName: bk.Name(),
			}
			allModels = append(allModels, model)
		}
	}

	if bk := s.aService.GetBackend(backend.ModelTypeSTT); bk != nil {
		models, err := bk.Models(context.Background())
		if err != nil {
			s.logger.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}

		for _, m := range models.Models {
			model := Model{
				Model:       m,
				BackendName: bk.Name(),
			}
			allModels = append(allModels, model)
		}
	}

	if bk := s.aService.GetBackend(backend.ModelTypeImage); bk != nil {
		models, err := bk.Models(context.Background())
		if err != nil {
			s.logger.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}

		for _, m := range models.Models {
			model := Model{
				Model:       m,
				BackendName: bk.Name(),
			}
			allModels = append(allModels, model)
		}
	}

	allModels = slices.SortedFunc(slices.Values(allModels), func(m1, m2 Model) int {
		return cmp.Compare(m1.String(), m2.String())
	})

	allModels = slices.CompactFunc(allModels, func(m1, m2 Model) bool {
		return m1.String() == m2.String()
	})

	resp.Models = allModels
}
