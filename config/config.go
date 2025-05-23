package config

import (
	"gopkg.in/yaml.v3"
	"os"
	"time"
	"zen/utils/logger"
)

type Config struct {
	Server struct {
		Port string `yaml:"port" envconfig:"SERVER_PORT"`
	} `yaml:"server"`
	Upstream    []string     `yaml:"upstream"`
	HealthCheck *HealthCheck `yaml:"health_check,omitempty"`
}

type HealthCheck struct {
	Enabled            bool          `yaml:"enabled"`
	Interval           time.Duration `yaml:"interval"`
	Timeout            time.Duration `yaml:"timeout"`
	HealthyThreshold   int           `yaml:"healthy_threshold"`
	UnhealthyThreshold int           `yaml:"unhealthy_threshold"`
}

func ParseConfig(cfg *Config, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		logger.Error("Failed to read configuration file: {}", err)
		return err
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(cfg)
	if err != nil {
		logger.Error("Failed to decode configuration file: {}", err)
		return err
	}

	if cfg.HealthCheck == nil {
		cfg.HealthCheck = &HealthCheck{
			Enabled:            true,
			Interval:           30 * time.Second,
			Timeout:            5 * time.Second,
			HealthyThreshold:   2,
			UnhealthyThreshold: 3,
		}
		logger.Info("Using default health check configuration")
	} else if cfg.HealthCheck.Enabled {
		if cfg.HealthCheck.Interval == 0 {
			cfg.HealthCheck.Interval = 30 * time.Second
		}
		if cfg.HealthCheck.Timeout == 0 {
			cfg.HealthCheck.Timeout = 5 * time.Second
		}
		if cfg.HealthCheck.HealthyThreshold == 0 {
			cfg.HealthCheck.HealthyThreshold = 2
		}
		if cfg.HealthCheck.UnhealthyThreshold == 0 {
			cfg.HealthCheck.UnhealthyThreshold = 3
		}
		logger.Info("Health check enabled with interval: {}", cfg.HealthCheck.Interval)
	}

	return nil
}
