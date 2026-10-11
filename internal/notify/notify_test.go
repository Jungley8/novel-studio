package notify

import (
	"errors"
	"runtime"
	"testing"
)

func TestEscapeAppleScript(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "backslashes",
			input:    `C:\path\to\file`,
			expected: `C:\\path\\to\\file`,
		},
		{
			name:     "double quotes",
			input:    `He said "hello"`,
			expected: `He said \"hello\"`,
		},
		{
			name:     "newlines carriage returns and tabs",
			input:    "Line 1\nLine 2\rLine 3\tTabbed",
			expected: "Line 1 Line 2 Line 3 Tabbed",
		},
		{
			name:     "combined complex characters",
			input:    "Error: \"failed\"\nPath: \\tmp\\test\tDetails: 'none'",
			expected: "Error: \\\"failed\\\" Path: \\\\tmp\\\\test Details: 'none'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := EscapeAppleScript(tt.input)
			if actual != tt.expected {
				t.Errorf("EscapeAppleScript(%q) = %q, want %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestDarwinNotifier(t *testing.T) {
	orig := runOsaScript
	defer func() { runOsaScript = orig }()

	var lastScript string
	runOsaScript = func(script string) error {
		lastScript = script
		return nil
	}

	dn := &darwinNotifier{}
	err := dn.Notify("Title \"Test\"", "Message with \n newline and \\ backslash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedScript := `display notification "Message with   newline and \\ backslash" with title "Title \"Test\""`
	if lastScript != expectedScript {
		t.Errorf("runOsaScript got %q, want %q", lastScript, expectedScript)
	}

	// Test error propagation
	expectedErr := errors.New("command failed")
	runOsaScript = func(script string) error {
		return expectedErr
	}
	err = dn.Notify("Title", "Message")
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestNoopNotifier(t *testing.T) {
	nn := &noopNotifier{}
	if err := nn.Notify("Title", "Message"); err != nil {
		t.Errorf("noopNotifier should return nil, got %v", err)
	}
}

func TestNew(t *testing.T) {
	n := New()
	if n == nil {
		t.Fatal("New() returned nil Notifier")
	}
	if runtime.GOOS == "darwin" {
		if _, ok := n.(*darwinNotifier); !ok {
			t.Errorf("expected *darwinNotifier on darwin, got %T", n)
		}
	} else {
		if _, ok := n.(*noopNotifier); !ok {
			t.Errorf("expected *noopNotifier on non-darwin, got %T", n)
		}
	}
}
