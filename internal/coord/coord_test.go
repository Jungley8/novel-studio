package coord

import (
	"errors"
	"testing"
	"time"
)

func TestEditLease_AcquireExpireRelease(t *testing.T) {
	now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)
	c := NewWithClock(func() time.Time { return now })

	if c.EditActive("p1") {
		t.Fatal("no lease should be inactive")
	}

	expiry := c.AcquireEdit("p1", 2*time.Minute)
	if !expiry.Equal(now.Add(2 * time.Minute)) {
		t.Fatalf("unexpected expiry: %v", expiry)
	}
	if !c.EditActive("p1") {
		t.Fatal("lease should be active")
	}

	// 未过期前续期
	now = now.Add(90 * time.Second)
	c.AcquireEdit("p1", 2*time.Minute)
	if !c.EditActive("p1") {
		t.Fatal("renewed lease should stay active")
	}

	// 到期即失效（惰性清理）
	now = now.Add(2*time.Minute + time.Second)
	if c.EditActive("p1") {
		t.Fatal("expired lease must be inactive")
	}

	// 重新获取后可显式释放
	c.AcquireEdit("p1", time.Minute)
	c.ReleaseEdit("p1")
	if c.EditActive("p1") {
		t.Fatal("released lease must be inactive")
	}
}

func TestBeginProduction_BlocksOnActiveEditLease(t *testing.T) {
	now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)
	c := NewWithClock(func() time.Time { return now })

	c.AcquireEdit("p1", time.Minute)
	if _, err := c.BeginProduction("p1", "排程续产"); !errors.Is(err, ErrBusy) {
		t.Fatalf("expected ErrBusy while editing, got %v", err)
	}

	// 租约过期后放行
	now = now.Add(time.Minute + time.Second)
	release, err := c.BeginProduction("p1", "排程续产")
	if err != nil {
		t.Fatalf("expected production after lease expiry, got %v", err)
	}
	release()
}

func TestBeginProduction_SingleProducerAndTokenIsolation(t *testing.T) {
	c := New()

	release1, err := c.BeginProduction("p1", "排程续产")
	if err != nil {
		t.Fatalf("first begin failed: %v", err)
	}
	if _, err := c.BeginProduction("p1", "一键成章"); !errors.Is(err, ErrBusy) {
		t.Fatalf("second producer must be rejected, got %v", err)
	}

	info, active := c.Production("p1")
	if !active || info.Label != "排程续产" {
		t.Fatalf("unexpected production info: %+v active=%v", info, active)
	}
	c.SetProductionChapter("p1", 7)
	if info, _ = c.Production("p1"); info.Chapter != 7 {
		t.Fatalf("expected chapter 7, got %+v", info)
	}

	// 释放后旧 release 不可误删新任务（代际隔离）
	release1()
	release2, err := c.BeginProduction("p1", "一键成章")
	if err != nil {
		t.Fatalf("re-begin failed: %v", err)
	}
	release1() // 旧代际的重复调用必须无副作用
	if info, active = c.Production("p1"); !active || info.Label != "一键成章" {
		t.Fatalf("stale release must not clear new production, got %+v active=%v", info, active)
	}
	release2()
	if _, active = c.Production("p1"); active {
		t.Fatal("released production must be inactive")
	}
}

func TestAcquireEdit_AllowedDuringProduction(t *testing.T) {
	c := New()

	release, err := c.BeginProduction("p1", "排程续产")
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	defer release()

	// 生产期间允许表达“人工在场”（具体写入口再裁决冲突）
	c.AcquireEdit("p1", time.Minute)
	if !c.EditActive("p1") {
		t.Fatal("edit lease should be granted during production")
	}
	if _, active := c.Production("p1"); !active {
		t.Fatal("production should remain active")
	}
}

func TestCoordinator_ScopesByProject(t *testing.T) {
	c := New()

	c.AcquireEdit("p1", time.Minute)
	release, err := c.BeginProduction("p2", "排程续产")
	if err != nil {
		t.Fatalf("different project must not be blocked: %v", err)
	}
	release()

	if !c.EditActive("p1") || c.EditActive("p2") {
		t.Fatal("leases must be scoped per project")
	}
}
