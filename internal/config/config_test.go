package config_test

import (
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
