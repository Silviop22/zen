package config

import (
	"gopkg.in/yaml.v3"
	"os"
	"zen/utils/logger"
)

type Config struct {
	Server struct {
		Port string `yaml:"port", envconfig:"SERVER_PORT"`
	} `yaml:"server"`
	Upstream []string `yaml:"upstream"`
}

func ParseConfig(cfg *Config, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		logger.Error("Failed to read configuration file:", err)
		return err
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(cfg)
	if err != nil {
		logger.Error("Failed to decode configuration file:", err)
		return err
	}
	return nil
}
