package coord

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type fakeLeaseStore struct {
	mu         sync.Mutex
	acquireErr error
	blocking   *domain.CoordinationLease

	acquireCalls int
	heartbeats   []int
	heartbeatOK  bool
	releases     []string
}

func (f *fakeLeaseStore) AcquireProductionLease(ctx context.Context, projectID, holder, label string, chapter int, ttl time.Duration) (*domain.CoordinationLease, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acquireCalls++
	if f.acquireErr != nil {
		return f.blocking, f.acquireErr
	}
	return nil, nil
}

func (f *fakeLeaseStore) HeartbeatProductionLease(ctx context.Context, projectID, holder string, chapter int, ttl time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.heartbeats = append(f.heartbeats, chapter)
	return f.heartbeatOK, nil
}

func (f *fakeLeaseStore) ReleaseProductionLease(ctx context.Context, projectID, holder string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releases = append(f.releases, holder)
	return nil
}

func (f *fakeLeaseStore) heartbeatCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.heartbeats)
}

func (f *fakeLeaseStore) lastHeartbeat() (int, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.heartbeats) == 0 {
		return 0, false
	}
	return f.heartbeats[len(f.heartbeats)-1], true
}

func (f *fakeLeaseStore) releaseCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.releases)
}

// 快速心跳间隔仅用于测试。
func withFastHeartbeat(t *testing.T) {
	t.Helper()
	old := heartbeatInterval
	heartbeatInterval = 10 * time.Millisecond
	t.Cleanup(func() { heartbeatInterval = old })
}

func TestAcquireProduction_BusyPassthrough(t *testing.T) {
	blocking := &domain.CoordinationLease{ProjectID: "p1", Kind: KindProduction, Label: "排程自动续产", Chapter: 3}
	fs := &fakeLeaseStore{acquireErr: ErrBusy, blocking: blocking}

	prod, gotBlocking, err := AcquireProduction(context.Background(), fs, "p1", "CLI 无头生产", 0)
	if prod != nil {
		t.Fatalf("busy acquire must not return a holder, got %+v", prod)
	}
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("expected ErrBusy, got %v", err)
	}
	if gotBlocking != blocking {
		t.Fatalf("blocking lease must be passed through, got %+v", gotBlocking)
	}
}

func TestAcquireProduction_RejectsInvalidArgs(t *testing.T) {
	if _, _, err := AcquireProduction(context.Background(), nil, "p1", "x", 0); err == nil {
		t.Fatal("nil store must be rejected")
	}
	fs := &fakeLeaseStore{}
	if _, _, err := AcquireProduction(context.Background(), fs, "", "x", 0); err == nil {
		t.Fatal("empty project id must be rejected")
	}
}

func TestProduction_HeartbeatTracksChapterAndReleaseStops(t *testing.T) {
	withFastHeartbeat(t)
	fs := &fakeLeaseStore{heartbeatOK: true}

	prod, _, err := AcquireProduction(context.Background(), fs, "p1", "排程自动续产", 1)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	prod.SetChapter(7)

	deadline := time.After(2 * time.Second)
	for {
		if ch, ok := fs.lastHeartbeat(); ok && ch == 7 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("chapter 7 never reached heartbeats, got %+v", fs.heartbeats)
		case <-time.After(10 * time.Millisecond):
		}
	}

	prod.Release()
	if fs.releaseCount() != 1 {
		t.Fatalf("expected exactly 1 release, got %d", fs.releaseCount())
	}

	// Release 返回后心跳必须已停止
	after := fs.heartbeatCount()
	time.Sleep(50 * time.Millisecond)
	if got := fs.heartbeatCount(); got != after {
		t.Fatalf("heartbeat must stop after Release, count grew %d → %d", after, got)
	}

	// 重复 Release 幂等（不 panic、再次释放无副作用语义）
	prod.Release()
}

func TestBindProduction_HookAndRelease(t *testing.T) {
	withFastHeartbeat(t)
	fs := &fakeLeaseStore{heartbeatOK: true}
	hook, trackChapter, release := BindProduction(fs, "排程开书")

	// 未获取锁前 trackChapter / release 为空操作
	trackChapter(9)
	release()

	if err := hook(context.Background(), "new-book"); err != nil {
		t.Fatalf("hook should succeed: %v", err)
	}
	trackChapter(5)
	time.Sleep(30 * time.Millisecond)
	if ch, ok := fs.lastHeartbeat(); !ok || ch != 5 {
		t.Fatalf("trackChapter should feed heartbeats, got %+v", fs.heartbeats)
	}

	release()
	if fs.releaseCount() != 1 {
		t.Fatalf("release must release exactly once, got %d", fs.releaseCount())
	}
}

func TestBindProduction_HookBusyFailsClosed(t *testing.T) {
	fs := &fakeLeaseStore{
		acquireErr: ErrBusy,
		blocking:   &domain.CoordinationLease{ProjectID: "p1", Kind: KindEdit, Label: "浏览器编辑"},
	}
	hook, _, release := BindProduction(fs, "排程开书")

	err := hook(context.Background(), "p1")
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("expected wrapped ErrBusy, got %v", err)
	}
	if !strings.Contains(err.Error(), "浏览器编辑") {
		t.Fatalf("error should carry blocking label, got %v", err)
	}
	release() // 未获取锁时释放为空操作
}
