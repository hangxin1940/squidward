package main

import (
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"squidward/backend"
	"testing"
)

func Test_configLoad(t *testing.T) {
	viper.SetConfigFile("config.yaml")

	err := viper.ReadInConfig()
	assert.Empty(t, err)

	var cfg []*backend.AdapterConfig

	err = viper.UnmarshalKey("ai_backend", &cfg)
	assert.Empty(t, err)

}
