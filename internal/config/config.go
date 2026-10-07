package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	ServerPort      int    `json:"server_port"`
	DataDir         string `json:"data_dir"`
	APIBase         string `json:"api_base"`
	APIKey          string `json:"api_key"`
	ReasoningModel  string `json:"reasoning_model"`
	WriterModel     string `json:"writer_model"`
	AutoOpenBrowser bool   `json:"auto_open_browser"`
}

func DefaultConfig() *Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return &Config{
		ServerPort:      28980,
		DataDir:         filepath.Join(home, ".novel-studio"),
		APIBase:         "https://api.deepseek.com/v1",
		APIKey:          "",
		ReasoningModel:  "deepseek-reasoner",
		WriterModel:     "deepseek-chat",
		AutoOpenBrowser: true,
	}
}

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
