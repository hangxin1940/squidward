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

	logger := lib.NewLogger(6, "SQUIDWARD", 8)

	var backendCfgs []*backend.AdapterConfig

	err = viper.UnmarshalKey("ai_backend", &backendCfgs)
	if err != nil {
		panic(err)
	}

	if backendCfgs == nil {
		logger.Fatalf("ai_backend not found in %s", conf_yaml)
	}

	aServcie := &backend.AdapterService{}
	for _, cfg := range backendCfgs {
		if cfg.Name == "" {
			logger.Fatal("`name` not found in config")
		}
		if cfg.Type == "" {
			logger.Fatalf("`type` not found in %s", cfg.Type)
		}
		if cfg.ApiBase == "" {
			logger.Fatalf("`api_base` not found in %s", cfg.Name)
		}
		if cfg.ApiStyle == "" {
			logger.Fatalf("`api_style` not found in %s", cfg.Name)
		}

		switch cfg.ApiStyle {
		case "openai":
			if bk, errb := backend.NewOpenAIStyleBackend(cfg); errb != nil {
				logger.Fatal(err)
			} else {
				aServcie.SetBackend(bk)
			}
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
