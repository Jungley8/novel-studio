package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMaskKey(t *testing.T) {
	if maskKey("1234567") != "******" {
		t.Errorf("expected masked short key, got %s", maskKey("1234567"))
	}
	longKey := "sk-ant-api03-abcdef1234567890"
	masked := maskKey(longKey)
	if !startsWith(masked, "sk-a") || !endsWith(masked, "7890") {
		t.Errorf("unexpected masked string: %s", masked)
	}
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func TestTailString(t *testing.T) {
	input := "line1\nline2\nline3\nline4\nline5"
	tailed := tailString(input, 2)
	expected := "line4\nline5\n"
	if tailed != expected {
		t.Errorf("tailString mismatch: got %q, want %q", tailed, expected)
	}

	short := "line1\nline2"
	if tailString(short, 5) != "line1\nline2\n" {
		t.Errorf("tailString short mismatch")
	}
}

func TestDoctorAndServiceManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "novel_studio_service_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewServiceManager(tmpDir, 28980)
	if err != nil {
		t.Fatalf("NewServiceManager failed: %v", err)
	}
	if mgr.port != 28980 {
		t.Errorf("expected port 28980, got %d", mgr.port)
	}

	// Status on non-running port
	running, msg := mgr.Status()
	if running {
		t.Errorf("expected running to be false, got true")
	}
	if msg == "" {
		t.Errorf("expected non-empty status message")
	}

	// Test logs reading
	logFile := filepath.Join(tmpDir, "service.log")
	_ = os.WriteFile(logFile, []byte("log1\nlog2\n"), 0644)
	logs, err := mgr.Logs(5)
	if err != nil {
		t.Fatalf("Logs failed: %v", err)
	}
	if logs == "" {
		t.Errorf("expected logs output")
	}

	// Run Doctor in tempDir
	code := RunDoctor(tmpDir, 39999)
	if code != 0 {
		t.Errorf("expected RunDoctor to succeed (return 0), got %d", code)
	}
}
