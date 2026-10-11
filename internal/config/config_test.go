package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Jungley8/novel-studio/internal/config"
)

func TestConfig_LoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load non-existent config failed: %v", err)
	}

	if cfg.ServerPort != 28980 {
		t.Errorf("expected default port 28980, got %d", cfg.ServerPort)
	}

	cfg.APIKey = "sk-test-123456"
	cfg.ServerPort = 9999

	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("Save config failed: %v", err)
	}

	reloaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Reload config failed: %v", err)
	}

	if reloaded.APIKey != "sk-test-123456" {
		t.Errorf("expected APIKey sk-test-123456, got %s", reloaded.APIKey)
	}
	if reloaded.ServerPort != 9999 {
		t.Errorf("expected ServerPort 9999, got %d", reloaded.ServerPort)
	}
}

func TestConfig_ContextBudgetTokensValidation(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Explicit context_budget_tokens: 0 -> error (Fail-Closed)
	zeroPath := filepath.Join(tmpDir, "zero.json")
	if err := os.WriteFile(zeroPath, []byte(`{"context_budget_tokens": 0}`), 0644); err != nil {
		t.Fatalf("failed to write zero config: %v", err)
	}
	if _, err := config.Load(zeroPath); err == nil {
		t.Fatalf("expected error for explicit context_budget_tokens: 0, got nil")
	}

	// 1b. Explicit negative context_budget_tokens: -100 -> error
	negPath := filepath.Join(tmpDir, "neg.json")
	if err := os.WriteFile(negPath, []byte(`{"context_budget_tokens": -100}`), 0644); err != nil {
		t.Fatalf("failed to write neg config: %v", err)
	}
	if _, err := config.Load(negPath); err == nil {
		t.Fatalf("expected error for explicit negative context_budget_tokens, got nil")
	}

	// 2. Explicit context_budget_tokens: 5000 -> ok and value is 5000
	customPath := filepath.Join(tmpDir, "custom.json")
	if err := os.WriteFile(customPath, []byte(`{"context_budget_tokens": 5000}`), 0644); err != nil {
		t.Fatalf("failed to write custom config: %v", err)
	}
	cfg, err := config.Load(customPath)
	if err != nil {
		t.Fatalf("unexpected error for context_budget_tokens: 5000: %v", err)
	}
	if cfg.ContextBudgetTokens != 5000 {
		t.Errorf("expected ContextBudgetTokens 5000, got %d", cfg.ContextBudgetTokens)
	}

	// 3. Missing key -> ok and defaults to 12000
	defaultPath := filepath.Join(tmpDir, "default.json")
	if err := os.WriteFile(defaultPath, []byte(`{"api_base": "https://api.example.com"}`), 0644); err != nil {
		t.Fatalf("failed to write default config: %v", err)
	}
	cfgDef, err := config.Load(defaultPath)
	if err != nil {
		t.Fatalf("unexpected error for missing context_budget_tokens: %v", err)
	}
	if cfgDef.ContextBudgetTokens != 12000 {
		t.Errorf("expected default ContextBudgetTokens 12000, got %d", cfgDef.ContextBudgetTokens)
	}
}

func TestConfig_DefaultPipelineSettings(t *testing.T) {
	// 1. Assert default values
	cfg := config.DefaultConfig()
	if cfg.DefaultWordsTarget != 2000 {
		t.Errorf("expected DefaultWordsTarget 2000, got %d", cfg.DefaultWordsTarget)
	}
	if cfg.DefaultMaxRewriteLoops != 3 {
		t.Errorf("expected DefaultMaxRewriteLoops 3, got %d", cfg.DefaultMaxRewriteLoops)
	}
	if cfg.DefaultNarrativeStyle != "" {
		t.Errorf("expected DefaultNarrativeStyle empty, got %q", cfg.DefaultNarrativeStyle)
	}
	if cfg.DefaultAutoLockIntents != false {
		t.Errorf("expected DefaultAutoLockIntents false, got %v", cfg.DefaultAutoLockIntents)
	}

	// 2. Negative validation (Fail-Closed)
	cfgNegWords := config.DefaultConfig()
	cfgNegWords.DefaultWordsTarget = -1
	if err := cfgNegWords.Validate(); err == nil {
		t.Errorf("expected error for negative DefaultWordsTarget, got nil")
	}

	cfgNegLoops := config.DefaultConfig()
	cfgNegLoops.DefaultMaxRewriteLoops = -1
	if err := cfgNegLoops.Validate(); err == nil {
		t.Errorf("expected error for negative DefaultMaxRewriteLoops, got nil")
	}

	// 0 is treated as fallback sentinel and allowed
	cfgZero := config.DefaultConfig()
	cfgZero.DefaultWordsTarget = 0
	cfgZero.DefaultMaxRewriteLoops = 0
	if err := cfgZero.Validate(); err != nil {
		t.Errorf("expected zero to be valid, got: %v", err)
	}

	// 3. Load rejection for negative JSON values
	tmpDir := t.TempDir()
	negWordsPath := filepath.Join(tmpDir, "neg_words.json")
	if err := os.WriteFile(negWordsPath, []byte(`{"default_words_target": -100}`), 0644); err != nil {
		t.Fatalf("failed to write neg_words config: %v", err)
	}
	if _, err := config.Load(negWordsPath); err == nil {
		t.Errorf("expected Load error for negative default_words_target, got nil")
	}

	negLoopsPath := filepath.Join(tmpDir, "neg_loops.json")
	if err := os.WriteFile(negLoopsPath, []byte(`{"default_max_rewrite_loops": -2}`), 0644); err != nil {
		t.Fatalf("failed to write neg_loops config: %v", err)
	}
	if _, err := config.Load(negLoopsPath); err == nil {
		t.Errorf("expected Load error for negative default_max_rewrite_loops, got nil")
	}

	// 4. JSON roundtrip & Load/Save persistence
	savePath := filepath.Join(tmpDir, "custom_pipeline.json")
	cfgCustom := config.DefaultConfig()
	cfgCustom.DefaultWordsTarget = 3500
	cfgCustom.DefaultNarrativeStyle = "硬派武侠"
	cfgCustom.DefaultMaxRewriteLoops = 5
	cfgCustom.DefaultAutoLockIntents = true

	if err := cfgCustom.Save(savePath); err != nil {
		t.Fatalf("failed to save custom pipeline config: %v", err)
	}

	reloaded, err := config.Load(savePath)
	if err != nil {
		t.Fatalf("failed to reload custom pipeline config: %v", err)
	}
	if reloaded.DefaultWordsTarget != 3500 {
		t.Errorf("expected DefaultWordsTarget 3500, got %d", reloaded.DefaultWordsTarget)
	}
	if reloaded.DefaultNarrativeStyle != "硬派武侠" {
		t.Errorf("expected DefaultNarrativeStyle '硬派武侠', got %q", reloaded.DefaultNarrativeStyle)
	}
	if reloaded.DefaultMaxRewriteLoops != 5 {
		t.Errorf("expected DefaultMaxRewriteLoops 5, got %d", reloaded.DefaultMaxRewriteLoops)
	}
	if !reloaded.DefaultAutoLockIntents {
		t.Errorf("expected DefaultAutoLockIntents true, got false")
	}
}
