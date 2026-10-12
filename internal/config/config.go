package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type ProviderConfig struct {
	APIBase string `json:"api_base"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

type Config struct {
	ServerPort             int             `json:"server_port"`
	DataDir                string          `json:"data_dir"`
	APIBase                string          `json:"api_base"`
	APIKey                 string          `json:"api_key"`
	ReasoningModel         string          `json:"reasoning_model"`
	WriterModel            string          `json:"writer_model"`
	ReviewerModel          string          `json:"reviewer_model,omitempty"`
	ReviewerProvider       *ProviderConfig `json:"reviewer_provider,omitempty"`
	ReasonerProvider       *ProviderConfig `json:"reasoner_provider,omitempty"`
	WriterProvider         *ProviderConfig `json:"writer_provider,omitempty"`
	ContextBudgetTokens    int             `json:"context_budget_tokens"`
	DefaultWordsTarget     int             `json:"default_words_target"`
	DefaultNarrativeStyle  string          `json:"default_narrative_style"`
	DefaultMaxRewriteLoops int             `json:"default_max_rewrite_loops"`
	DefaultAutoLockIntents bool            `json:"default_auto_lock_intents"`
	QueueTimes             []string        `json:"queue_times"`
}

func DefaultConfig() *Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return &Config{
		ServerPort:             28980,
		DataDir:                filepath.Join(home, ".novel-studio"),
		APIBase:                "https://api.deepseek.com/v1",
		APIKey:                 "",
		ReasoningModel:         "deepseek-reasoner",
		WriterModel:            "deepseek-chat",
		ReviewerModel:          "deepseek-reasoner",
		ContextBudgetTokens:    12000,
		DefaultWordsTarget:     2000,
		DefaultNarrativeStyle:  "",
		DefaultMaxRewriteLoops: 3,
		DefaultAutoLockIntents: false,
		QueueTimes:             []string{},
	}
}

func (c *Config) Validate() error {
	if c.ContextBudgetTokens <= 0 {
		return fmt.Errorf("context_budget_tokens must be positive, got %d", c.ContextBudgetTokens)
	}
	if c.DefaultWordsTarget < 0 {
		return fmt.Errorf("default_words_target cannot be negative, got %d", c.DefaultWordsTarget)
	}
	if c.DefaultMaxRewriteLoops < 0 {
		return fmt.Errorf("default_max_rewrite_loops cannot be negative, got %d", c.DefaultMaxRewriteLoops)
	}
	if err := domain.ValidateDailyTimes(c.QueueTimes); err != nil {
		return fmt.Errorf("queue_times: %w", err)
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
	if err := cfg.Validate(); err != nil {
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
	tmp := fmt.Sprintf("%s.%d.tmp", path, time.Now().UnixNano())
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
