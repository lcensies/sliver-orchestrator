package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Infra     map[string]InfraConfig `yaml:"infra"`
	Payloads  []PayloadConfig        `yaml:"payloads"`
	OutputDir string                 `yaml:"outputdir"`
}

type InfraConfig struct {
	C2Host     string `yaml:"c2_host"`
	ImplantURL string `yaml:"implant_url"`
}

type PayloadConfig struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	InfraRef string `yaml:"infra_ref"`
	Filename string `yaml:"filename"`
	OS       string `yaml:"os"`
	Arch     string `yaml:"arch"`
}

// LoadConfig reads and parses YAML
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
