package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/coord"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

// runProduceCLIWithStderr 运行 CLI 并捕获 stderr 文本。
func runProduceCLIWithStderr(t *testing.T, tempDir string, args []string) (int, string) {
	t.Helper()
	oldStderr := os.Stderr
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = pw
	code := runProduceCLI(append([]string{"-data-dir", tempDir}, args...), tempDir)
	_ = pw.Close()
	os.Stderr = oldStderr
	out, _ := io.ReadAll(pr)
	_ = pr.Close()
	return code, string(out)
}

func setupProduceLeaseFixture(t *testing.T) (tempDir string, projectID string) {
	t.Helper()
	tempDir = t.TempDir()
	s, err := store.NewSQLiteStore(filepath.Join(tempDir, "novel.db"))
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	ctx := context.Background()
	proj := &domain.Project{ID: "proj-busy", Title: "占线作品", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.SaveProject(ctx, proj); err != nil {
		_ = s.Close()
		t.Fatalf("save project: %v", err)
	}
	_ = s.Close()
	return tempDir, proj.ID
}

// 编辑租约未过期（守护服务浏览器正在编辑）→ CLI produce 必须 Fail-Closed：
// 退出码 1、输出忙碌原因、不写入生产锁。
func TestRunProduceCLI_RejectedWhenEditLeaseHeld(t *testing.T) {
	tempDir, projectID := setupProduceLeaseFixture(t)
	ctx := context.Background()

	s, err := store.NewSQLiteStore(filepath.Join(tempDir, "novel.db"))
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	if _, err := s.UpsertEditLease(ctx, projectID, time.Minute); err != nil {
		_ = s.Close()
		t.Fatalf("hold edit lease: %v", err)
	}
	_ = s.Close()

	code, stderr := runProduceCLIWithStderr(t, tempDir, []string{"-project", projectID})
	if code != 1 {
		t.Fatalf("expected exit code 1 when edit lease held, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "正在被人工编辑") {
		t.Fatalf("expected busy message on stderr, got: %s", stderr)
	}

	// 失败路径不得留下生产锁；编辑租约原样保留（仍属浏览器）
	s, err = store.NewSQLiteStore(filepath.Join(tempDir, "novel.db"))
	if err != nil {
		t.Fatalf("reopen store after run: %v", err)
	}
	defer s.Close()
	if lease, _ := s.GetLiveLease(ctx, projectID, coord.KindProduction); lease != nil {
		t.Fatalf("busy CLI must not leave a production lease, got %+v", lease)
	}
	if lease, _ := s.GetLiveLease(ctx, projectID, coord.KindEdit); lease == nil {
		t.Fatal("edit lease must remain after rejected CLI run")
	}
}

// 已有生产锁（守护服务排程/另一 CLI 正在生产）→ CLI produce 拒绝并展示生产方标签。
func TestRunProduceCLI_RejectedWhenProductionLeaseHeld(t *testing.T) {
	tempDir, projectID := setupProduceLeaseFixture(t)
	ctx := context.Background()

	s, err := store.NewSQLiteStore(filepath.Join(tempDir, "novel.db"))
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	if _, err := s.AcquireProductionLease(ctx, projectID, "daemon-holder", "排程自动续产", 5, coord.ProductionLeaseTTL); err != nil {
		_ = s.Close()
		t.Fatalf("hold production lease: %v", err)
	}
	_ = s.Close()

	code, stderr := runProduceCLIWithStderr(t, tempDir, []string{"-project", projectID})
	if code != 1 {
		t.Fatalf("expected exit code 1 when production lease held, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "排程自动续产") {
		t.Fatalf("expected production label on stderr, got: %s", stderr)
	}
}
