package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ProviderConfig struct {
	APIBase string `json:"api_base"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

type Config struct {
	ServerPort          int             `json:"server_port"`
	DataDir             string          `json:"data_dir"`
	APIBase             string          `json:"api_base"`
	APIKey              string          `json:"api_key"`
	ReasoningModel      string          `json:"reasoning_model"`
	WriterModel         string          `json:"writer_model"`
	ReviewerModel       string          `json:"reviewer_model,omitempty"`
	ReviewerProvider    *ProviderConfig `json:"reviewer_provider,omitempty"`
	ReasonerProvider    *ProviderConfig `json:"reasoner_provider,omitempty"`
	WriterProvider      *ProviderConfig `json:"writer_provider,omitempty"`
	AutoOpenBrowser     bool            `json:"auto_open_browser"`
	ContextBudgetTokens int             `json:"context_budget_tokens,omitempty"`
}

func DefaultConfig() *Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return &Config{
		ServerPort:          28980,
		DataDir:             filepath.Join(home, ".novel-studio"),
		APIBase:             "https://api.deepseek.com/v1",
		APIKey:              "",
		ReasoningModel:      "deepseek-reasoner",
		WriterModel:         "deepseek-chat",
		ReviewerModel:       "deepseek-reasoner",
		AutoOpenBrowser:     true,
		ContextBudgetTokens: 12000,
	}
}

func (c *Config) Validate() error {
	if c.ContextBudgetTokens <= 0 {
		return fmt.Errorf("context_budget_tokens must be positive, got %d", c.ContextBudgetTokens)
	}
	return nil
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
	if cfg.ContextBudgetTokens <= 0 {
		cfg.ContextBudgetTokens = 12000
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
	tmp := fmt.Sprintf("%s.%d.tmp", path, time.Now().UnixNano())
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
