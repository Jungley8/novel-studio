package notify

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Notifier 发送系统通知（best-effort 旁路通道）。
type Notifier interface {
	Notify(title, message string) error
}

// runOsaScript is package-level variable for running osascript (can be swapped in tests).
var runOsaScript = func(script string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "osascript", "-e", script).Run()
}

// EscapeAppleScript escapes special characters for AppleScript string literals:
// `\` -> `\\`, `"` -> `\"`, and `\n`/`\r`/`\t` -> space.
func EscapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return s
}

type darwinNotifier struct{}

func (d *darwinNotifier) Notify(title, message string) error {
	script := fmt.Sprintf(`display notification "%s" with title "%s"`, EscapeAppleScript(message), EscapeAppleScript(title))
	return runOsaScript(script)
}

type noopNotifier struct{}

func (n *noopNotifier) Notify(title, message string) error {
	return nil
}

// New: darwin → darwinNotifier；其它平台 → noopNotifier（返回 nil）。
func New() Notifier {
	if runtime.GOOS == "darwin" {
		return &darwinNotifier{}
	}
	return &noopNotifier{}
}
