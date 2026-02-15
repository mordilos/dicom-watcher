package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	DirectoryPath string `yaml:"directory_path"`
	ApiUrl        string `yaml:"api_url"`
	Timeout       int    `yaml:"timeout"`
	PollInterval  int    `yaml:"poll_interval"`
	BatchSize     int    `yaml:"batch_size"`
}

func ReadConfig(filePath string) (*Config, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
