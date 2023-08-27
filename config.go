package main

import (
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func NewConfig(configPath string, configName string) {
	if configPath == "" {
		configPath = "."
	}

	if configName == "" {
		configName = "config"
	}

	viper.SetConfigType("yaml")
	viper.SetConfigName(configName)
	viper.AddConfigPath(configPath)

	err := viper.ReadInConfig()
	if err != nil {
		panic(errors.Wrap(err, "Config file read error"))
	}
}
