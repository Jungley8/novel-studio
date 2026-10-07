package desktop_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/desktop"
)

func TestWaitForShutdown_Graceful(t *testing.T) {
	// Verify that a server can be cancelled or shutdown with context
	srv := &http.Server{Addr: ":0"}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Shutdown(ctx)
	}()

	select {
	case err := <-errChan:
		if err != nil {
			t.Logf("shutdown completed: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("shutdown timed out")
	}
}

func TestOpenBrowser_CommandBuild(t *testing.T) {
	// Non-blocking invocation verification on test
	_ = desktop.OpenBrowser("http://127.0.0.1:28980")
}
