package main

import (
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"net"
	"os"
	"os/signal"
	api_server "squidward/api-server"
	"squidward/backend"
	"squidward/lib"
	"syscall"
)

var (
	conf_yaml string
)

func init() {

	flag.StringVar(&conf_yaml, "c", "config.yaml", "config file")

	gin.SetMode(gin.DebugMode)

}

func main() {
	flag.Parse()
	viper.SetConfigFile(conf_yaml)

	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}

	backendCfgs := viper.GetStringMap("ai_backend")

	logger := lib.NewLogger(6, "SQUIDWARD", 8)

	aServcie := &backend.AdapterService{}
	for bktype, cfgI := range backendCfgs {
		cfg := cfgI.(map[string]interface{})
		config := map[string]string{}
		for key, value := range cfg {
			strKey := fmt.Sprintf("%v", key)
			strValue := fmt.Sprintf("%v", value)
			config[strKey] = strValue
		}

		if _, has := config["name"]; !has {
			logger.Fatalf("`name` not found in %s", bktype)
		}
		if _, has := config["type"]; !has {
			logger.Fatalf("`type` not found in %s", bktype)
		}
		if _, has := config["api_base"]; !has {
			logger.Fatalf("`api_base` not found in %s", bktype)
		}

		var bk backend.Adapter

		switch config["type"] {
		case "openai":
			var errb error
			bk, errb = backend.NewOpenAIStyleBackend(bktype, config, nil)
			if errb != nil {
				logger.Fatal(err)
			}
		}

		switch bktype {
		case "llm":
			aServcie.SetLLMBackend(bk)
		case "tts":
			aServcie.SetTTSBackend(bk)
		case "stt":
			aServcie.SetSTTBackend(bk)
		case "image":
			aServcie.SetImageBackend(bk)
		}
	}

	// --------- 初始Manager服务器 ---------
	netListener, err := net.Listen("tcp", "0.0.0.0:12345")
	if err != nil {
		logger.Fatalf("Fatal error config file: %v", err)
	}

	apiServer := api_server.NewApiServer(logger, aServcie)

	go func() {
		if serr := apiServer.Serve(netListener); serr != nil {
			logger.Error(serr)
		}
	}()

	logger.Info("启动完成")
	// --------- 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit
	fmt.Println("正在终止...")
	apiServer.Stop()
}
