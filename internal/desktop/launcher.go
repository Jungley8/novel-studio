package desktop

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
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
