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
