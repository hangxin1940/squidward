package lib

import (
	"github.com/sirupsen/logrus"
	"path/filepath"
	"runtime"
)

func NewLogger(level int, prefix string, stack int) *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.Level(level))
	logger.SetFormatter(&TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		Prefix:          prefix,
		ForceFormatting: true,
		StackLayer:      stack,
	})
	return logger
}

func RuntimeDir() string {
	_, b, _, _ := runtime.Caller(1)
	return filepath.Dir(b)
}
