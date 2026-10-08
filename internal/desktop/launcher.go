package desktop

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

// OpenBrowser attempts to open the default system browser to the target URL.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		// linux / freebsd
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start()
}

// WaitForShutdown blocks until an interrupt signal is received and shuts down the server gracefully.
func WaitForShutdown(srv *http.Server) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	sig := <-quit
	fmt.Printf("\n[NovelStudio] 接收到停机信号 (%s)，正在安全退出...\n", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return srv.Shutdown(ctx)
}

// EnsureCLIInPATH checks and automatically creates a symlink to ~/.local/bin/novel-studio
// so that open-source users and GUI desktop users can immediately execute `novel-studio` in terminal without manual setup.
func EnsureCLIInPATH() {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return
	}
	execPath, err := os.Executable()
	if err != nil {
		return
	}

	// Resolve symlinks on the current executable to get true path
	if resolved, rerr := filepath.EvalSymlinks(execPath); rerr == nil && resolved != "" {
		execPath = resolved
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	localBin := filepath.Join(home, ".local", "bin")
	_ = os.MkdirAll(localBin, 0755)
	targetSymlink := filepath.Join(localBin, "novel-studio")

	// On macOS, if the installed /Applications bundle exists, prefer pointing to it
	if runtime.GOOS == "darwin" {
		appBin := "/Applications/novel-studio.app/Contents/MacOS/novel-studio"
		if _, serr := os.Stat(appBin); serr == nil {
			execPath = appBin
		}
	}

	// Never create a circular symlink pointing to itself
	if filepath.Clean(execPath) == filepath.Clean(targetSymlink) {
		return
	}

	// Check existing symlink destination
	if dest, rerr := os.Readlink(targetSymlink); rerr == nil {
		if filepath.Clean(dest) == filepath.Clean(execPath) {
			return
		}
		// If dest points to targetSymlink itself or is circular, remove it
		if filepath.Clean(dest) == filepath.Clean(targetSymlink) {
			_ = os.Remove(targetSymlink)
		}
	}

	_ = os.Remove(targetSymlink)
	_ = os.Symlink(execPath, targetSymlink)
}
