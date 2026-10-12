package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/coord"
	"github.com/Jungley8/novel-studio/internal/domain"
)

// newLockTestStores 在同一数据库文件上打开两个 store 实例，
// 模拟“守护服务”与“CLI”两个进程共享数据面的跨进程协调场景。
func newLockTestStores(t *testing.T) (daemon *SQLiteStore, cli *SQLiteStore) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "locks.db")
	daemon, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("open daemon store: %v", err)
	}
	cli, err = NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("open cli store: %v", err)
	}
	t.Cleanup(func() {
		_ = daemon.Close()
		_ = cli.Close()
	})
	if err := daemon.SaveProject(context.Background(), &domain.Project{ID: "p1", Title: "锁测试"}); err != nil {
		t.Fatalf("save project: %v", err)
	}
	return daemon, cli
}

func TestCoordinationLeases_EditLeaseRoundtripAcrossInstances(t *testing.T) {
	daemon, cli := newLockTestStores(t)
	ctx := context.Background()

	expires, err := daemon.UpsertEditLease(ctx, "p1", time.Minute)
	if err != nil {
		t.Fatalf("upsert edit lease: %v", err)
	}
	if !expires.After(time.Now()) {
		t.Fatalf("expiry should be in the future, got %v", expires)
	}

	// 另一方（CLI 视角）可见同一编辑租约
	lease, err := cli.GetLiveLease(ctx, "p1", coord.KindEdit)
	if err != nil || lease == nil {
		t.Fatalf("edit lease should be visible cross-instance, got %+v err=%v", lease, err)
	}

	if err := cli.DeleteEditLease(ctx, "p1"); err != nil {
		t.Fatalf("delete edit lease: %v", err)
	}
	if lease, _ = daemon.GetLiveLease(ctx, "p1", coord.KindEdit); lease != nil {
		t.Fatalf("edit lease should be gone after release, got %+v", lease)
	}
}

func TestCoordinationLeases_ProductionBlockedByEditLease(t *testing.T) {
	daemon, cli := newLockTestStores(t)
	ctx := context.Background()

	if _, err := daemon.UpsertEditLease(ctx, "p1", time.Minute); err != nil {
		t.Fatalf("upsert edit lease: %v", err)
	}

	// 浏览器人工编辑在场 → CLI 取生产锁必须失败
	blocking, err := cli.AcquireProductionLease(ctx, "p1", "cli-holder", "CLI 无头生产", 5, coord.ProductionLeaseTTL)
	if !errors.Is(err, coord.ErrBusy) {
		t.Fatalf("expected ErrBusy while editing, got %v", err)
	}
	if blocking == nil || blocking.Kind != coord.KindEdit {
		t.Fatalf("blocking lease should be the edit lease, got %+v", blocking)
	}

	// 人工收工 → 生产锁可取
	if err := daemon.DeleteEditLease(ctx, "p1"); err != nil {
		t.Fatalf("release edit lease: %v", err)
	}
	blocking, err = cli.AcquireProductionLease(ctx, "p1", "cli-holder", "CLI 无头生产", 5, coord.ProductionLeaseTTL)
	if err != nil || blocking != nil {
		t.Fatalf("acquire after release failed: blocking=%+v err=%v", blocking, err)
	}

	// 守护服务视角：生产锁在位，第二个生产者必须失败并看到生产方标签
	blocking, err = daemon.AcquireProductionLease(ctx, "p1", "daemon-holder", "排程自动续产", 6, coord.ProductionLeaseTTL)
	if !errors.Is(err, coord.ErrBusy) {
		t.Fatalf("expected ErrBusy while produced, got %v", err)
	}
	if blocking == nil || blocking.Kind != coord.KindProduction || blocking.Label != "CLI 无头生产" {
		t.Fatalf("blocking lease should be production by CLI, got %+v", blocking)
	}
}

func TestCoordinationLeases_EditAllowedDuringProduction(t *testing.T) {
	daemon, _ := newLockTestStores(t)
	ctx := context.Background()

	if _, err := daemon.AcquireProductionLease(ctx, "p1", "cli-holder", "CLI 无头生产", 1, coord.ProductionLeaseTTL); err != nil {
		t.Fatalf("acquire production: %v", err)
	}

	// 生产期间允许表达“人工在场”（冲突在具体写入口裁决）
	if _, err := daemon.UpsertEditLease(ctx, "p1", time.Minute); err != nil {
		t.Fatalf("edit lease should be grantable during production: %v", err)
	}
	if lease, _ := daemon.GetLiveLease(ctx, "p1", coord.KindEdit); lease == nil {
		t.Fatal("edit lease should be live")
	}
	if lease, _ := daemon.GetLiveLease(ctx, "p1", coord.KindProduction); lease == nil {
		t.Fatal("production lease should remain live")
	}
}

func TestCoordinationLeases_HeartbeatAndReleaseHolderIsolation(t *testing.T) {
	daemon, cli := newLockTestStores(t)
	ctx := context.Background()

	if _, err := cli.AcquireProductionLease(ctx, "p1", "holder-1", "CLI 无头生产", 0, coord.ProductionLeaseTTL); err != nil {
		t.Fatalf("acquire: %v", err)
	}

	// 错误 holder 的续期/释放均无效（代际隔离）
	if ok, err := daemon.HeartbeatProductionLease(ctx, "p1", "holder-wrong", 9, coord.ProductionLeaseTTL); err != nil || ok {
		t.Fatalf("wrong-holder heartbeat must be a no-op, ok=%v err=%v", ok, err)
	}
	if err := daemon.ReleaseProductionLease(ctx, "p1", "holder-wrong"); err != nil {
		t.Fatalf("wrong-holder release: %v", err)
	}
	if lease, _ := cli.GetLiveLease(ctx, "p1", coord.KindProduction); lease == nil {
		t.Fatal("lock must survive wrong-holder release")
	}

	// 正确 holder 续期更新章节号
	if ok, err := cli.HeartbeatProductionLease(ctx, "p1", "holder-1", 7, coord.ProductionLeaseTTL); err != nil || !ok {
		t.Fatalf("heartbeat failed: ok=%v err=%v", ok, err)
	}
	if lease, _ := daemon.GetLiveLease(ctx, "p1", coord.KindProduction); lease == nil || lease.Chapter != 7 {
		t.Fatalf("heartbeat should update chapter, got %+v", lease)
	}

	// 正确 holder 释放后他人可取锁
	if err := cli.ReleaseProductionLease(ctx, "p1", "holder-1"); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := daemon.AcquireProductionLease(ctx, "p1", "holder-2", "排程自动续产", 8, coord.ProductionLeaseTTL); err != nil {
		t.Fatalf("acquire after release should succeed: %v", err)
	}
}

func TestCoordinationLeases_ExpiredTakeoverAndCleanup(t *testing.T) {
	daemon, cli := newLockTestStores(t)
	ctx := context.Background()

	if _, err := cli.AcquireProductionLease(ctx, "p1", "holder-1", "CLI 无头生产", 1, 40*time.Millisecond); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	time.Sleep(80 * time.Millisecond)

	// 过期锁视为不存在，可直接接管
	if lease, _ := daemon.GetLiveLease(ctx, "p1", coord.KindProduction); lease != nil {
		t.Fatalf("expired lease must not be live, got %+v", lease)
	}
	if _, err := daemon.AcquireProductionLease(ctx, "p1", "holder-2", "排程自动续产", 2, coord.ProductionLeaseTTL); err != nil {
		t.Fatalf("expired lock should be takeable: %v", err)
	}

	// 过期残留清理
	if _, err := cli.UpsertEditLease(ctx, "p1", 30*time.Millisecond); err != nil {
		t.Fatalf("upsert edit lease: %v", err)
	}
	time.Sleep(60 * time.Millisecond)
	removed, err := cli.CleanupExpiredLeases(ctx)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if removed < 1 {
		t.Fatalf("expected at least 1 expired lease cleaned, got %d", removed)
	}
	if lease, _ := daemon.GetLiveLease(ctx, "p1", coord.KindEdit); lease != nil {
		t.Fatalf("expired edit lease must be gone, got %+v", lease)
	}
}
