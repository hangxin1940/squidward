package api_server

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"io"
	"net"
	"net/http"
	"slices"
	"squidward/backend"
	"strconv"
)

func NewApiServer(logger *logrus.Logger, aService *backend.AdapterService) *ApiServer {
	return &ApiServer{
		logger:   logger,
		apiBase:  "/v1",
		aService: aService,
	}
}

type ApiServer struct {
	logger      *logrus.Logger
	apiBase     string
	netListener net.Listener
	aService    *backend.AdapterService
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
		apiRouter.POST("/audio/transcriptions", s.audioTranscriptions)
		apiRouter.GET("/models", s.models)
	}

	return router
}

func (s *ApiServer) Stop() {
	_ = s.netListener.Close()
}

// chatCompletions 聊天
func (s *ApiServer) chatCompletions(c *gin.Context) {
	bk := s.aService.GetLLMBackend()
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
				stream, rerr := res.RecvRaw()
				if rerr != nil {
					if errors.Is(rerr, io.EOF) {
						return false
					}
					return true
				}
				_, _ = w.Write(stream)
			}
		})
	} else {
		res, err := bk.ChatCompletions(context.Background(), req)
		if err != nil {
			s.logger.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}

		c.JSON(200, res)
	}

}

// imagesGenerations 创建图像
func (s *ApiServer) imagesGenerations(c *gin.Context) {
	bk := s.aService.GetImageBackend()
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
	bk := s.aService.GetTTSBackend()
	if bk == nil {
		s.logger.Error("未配置TTS")
		c.Status(http.StatusInternalServerError)
		return
	}

	req := openai.CreateSpeechRequest{}

	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	res, err := bk.AudioSpeech(context.Background(), req)
	if err != nil {
		s.logger.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	length, _ := strconv.ParseInt(res.Header().Get("Content-Length"), 10, 64)

	c.DataFromReader(http.StatusOK, int64(length), res.Header().Get("Content-Type"), res, nil)

}

// audioTranscriptions STT
func (s *ApiServer) audioTranscriptions(c *gin.Context) {
	bk := s.aService.GetSTTBackend()
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

	req := openai.AudioRequest{}
	if model, has := form.Value["model"]; has {
		req.Model = model[0]
	} else {
		c.Status(http.StatusBadRequest)
		return
	}

	if audio, has := form.File["file"]; has {
		fs, err := audio[0].Open()
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		req.Reader = fs
		req.FilePath = audio[0].Filename
	} else {
		c.Status(http.StatusBadRequest)
		return
	}

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

type Model struct {
	openai.Model
	BackendName string `json:"backend_name"`
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

	if bk := s.aService.GetLLMBackend(); bk != nil {
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

	if bk := s.aService.GetTTSBackend(); bk != nil {
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

	if bk := s.aService.GetSTTBackend(); bk != nil {
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
