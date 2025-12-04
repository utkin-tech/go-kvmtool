package config

import (
	"os"

	"github.com/caarlos0/env/v11"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Kernel string `yaml:"kernel" env:"GKVM_KERNEL"`
	Agent  string `yaml:"agent" env:"GKVM_AGENT"`
}

func LoadConfig() (*Config, error) {
	configPath := os.Getenv("GKVM_CONFIG_PATH")
	if configPath == "" {
		paths := []string{
			"/etc/gkvm/config.yaml",
			"/usr/local/etc/gkvm/config.yaml",
			os.ExpandEnv("$HOME/.config/gkvm/config.yaml"),
			os.ExpandEnv("$HOME/.gkvm/config.yaml"),
		}

		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}
	}

	var cfg Config
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	}

	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
