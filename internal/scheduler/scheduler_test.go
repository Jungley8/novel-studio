package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/coord"
	"github.com/Jungley8/novel-studio/internal/domain"
)

// In-memory fake store for scheduler tests
type fakeStore struct {
	mu           sync.Mutex
	automations  []*domain.ProjectAutomation
	queue        []*domain.BookQueueItem
	state        map[string]string
	staleResetN  int64
	recordedRuns []runRecord
	pausedCalls  []pauseRecord
}

type runRecord struct {
	projectID string
	status    string
	message   string
	at        time.Time
}

type pauseRecord struct {
	projectID string
	paused    bool
	reason    string
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		state: make(map[string]string),
	}
}

func (f *fakeStore) ListProjectAutomations(ctx context.Context) ([]*domain.ProjectAutomation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	res := make([]*domain.ProjectAutomation, len(f.automations))
	for i, a := range f.automations {
		cp := *a
		res[i] = &cp
	}
	return res, nil
}

func (f *fakeStore) SetAutomationPaused(ctx context.Context, projectID string, paused bool, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pausedCalls = append(f.pausedCalls, pauseRecord{projectID: projectID, paused: paused, reason: reason})
	for _, a := range f.automations {
		if a.ProjectID == projectID {
			a.Paused = paused
			a.PausedReason = reason
		}
	}
	return nil
}

func (f *fakeStore) RecordAutomationRun(ctx context.Context, projectID, status, message string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordedRuns = append(f.recordedRuns, runRecord{projectID: projectID, status: status, message: message, at: at})
	return nil
}

func (f *fakeStore) ListBookQueue(ctx context.Context) ([]*domain.BookQueueItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	res := make([]*domain.BookQueueItem, len(f.queue))
	for i, it := range f.queue {
		cp := *it
		res[i] = &cp
	}
	return res, nil
}

func (f *fakeStore) SaveBookQueueItem(ctx context.Context, item *domain.BookQueueItem) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, it := range f.queue {
		if it.ID == item.ID {
			cp := *item
			f.queue[i] = &cp
			return nil
		}
	}
	cp := *item
	f.queue = append(f.queue, &cp)
	return nil
}

func (f *fakeStore) ResetStaleQueueRunning(ctx context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var cnt int64
	for _, it := range f.queue {
		if it.Status == domain.QueueStatusRunning {
			it.Status = domain.QueueStatusPending
			cnt++
		}
	}
	f.staleResetN = cnt
	return cnt, nil
}

func (f *fakeStore) StaleResetCount() int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.staleResetN
}

func (f *fakeStore) GetSchedulerState(ctx context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state[key], nil
}

func (f *fakeStore) SetSchedulerState(ctx context.Context, key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state[key] = value
	return nil
}

// In-memory fake runners
type fakeContinuationRunner struct {
	mu    sync.Mutex
	calls []string
	fn    func(ctx context.Context, projectID string, kind domain.AutomationQuotaKind, count int) (bool, string, error)
}

func (f *fakeContinuationRunner) RunContinuation(ctx context.Context, projectID string, kind domain.AutomationQuotaKind, count int, progress func(string)) (bool, string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, projectID)
	f.mu.Unlock()
	if progress != nil {
		progress("continuing " + projectID)
	}
	if f.fn != nil {
		return f.fn(ctx, projectID, kind, count)
	}
	return false, "ok", nil
}

type fakeBookRunner struct {
	mu    sync.Mutex
	calls []string
	fn    func(ctx context.Context, item *domain.BookQueueItem) (bool, string, string, error)
}

func (f *fakeBookRunner) RunBook(ctx context.Context, item *domain.BookQueueItem, progress func(string)) (bool, string, string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, item.ID)
	f.mu.Unlock()
	if progress != nil {
		progress("running book " + item.ID)
	}
	if f.fn != nil {
		return f.fn(ctx, item)
	}
	return false, "proj_" + item.ID, "ok", nil
}

type fakeNotifier struct {
	mu     sync.Mutex
	titles []string
	msgs   []string
}

func (f *fakeNotifier) Notify(title, message string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.titles = append(f.titles, title)
	f.msgs = append(f.msgs, message)
	return nil
}

// --------------------- Tests ---------------------

func TestDueSlot(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}

	// now = 2026-10-11 15:30:00 (slot earlier today is 09:00, 6.5 hours ago)
	now := time.Date(2026, 10, 11, 15, 30, 0, 0, loc)

	tests := []struct {
		name     string
		times    []string
		lastRun  time.Time
		wantHit  bool
		wantSlot time.Time
	}{
		{
			name:     "catch-up: lastRun zero, slot earlier today within 24h triggers",
			times:    []string{"09:00"},
			lastRun:  time.Time{},
			wantHit:  true,
			wantSlot: time.Date(2026, 10, 11, 9, 0, 0, 0, loc),
		},
		{
			name:     "catch-up: lastRun zero, slot yesterday within 24h triggers",
			times:    []string{"18:00"}, // yesterday 18:00 is within [now-24h, now]
			lastRun:  time.Time{},
			wantHit:  true,
			wantSlot: time.Date(2026, 10, 10, 18, 0, 0, 0, loc),
		},
		{
			name:    "strictly greater: S == lastRun does NOT trigger",
			times:   []string{"09:00"},
			lastRun: time.Date(2026, 10, 11, 9, 0, 0, 0, loc),
			wantHit: false,
		},
		{
			name:    "strictly greater: S < lastRun does NOT trigger",
			times:   []string{"09:00"},
			lastRun: time.Date(2026, 10, 11, 9, 30, 0, 0, loc),
			wantHit: false,
		},
		{
			name:     "slot yesterday after lastRun within 24h triggers",
			times:    []string{"16:00"}, // today 16:00 is future; yesterday 16:00 is 23.5h ago (> lastRun yesterday 15:00)
			lastRun:  time.Date(2026, 10, 10, 15, 0, 0, 0, loc),
			wantHit:  true,
			wantSlot: time.Date(2026, 10, 10, 16, 0, 0, 0, loc),
		},
		{
			name:    "slot yesterday before lastRun does NOT trigger",
			times:   []string{"16:00"},
			lastRun: time.Date(2026, 10, 10, 17, 0, 0, 0, loc),
			wantHit: false,
		},
		{
			name:     "multiple times picks max S",
			times:    []string{"08:00", "12:00", "14:00", "18:00"},
			lastRun:  time.Time{},
			wantHit:  true,
			wantSlot: time.Date(2026, 10, 11, 14, 0, 0, 0, loc), // today 14:00 is max <= now
		},
		{
			name:     "invalid times skipped and fallback to valid",
			times:    []string{"bad:time", "25:00", "08:00", "12:99"},
			lastRun:  time.Time{},
			wantHit:  true,
			wantSlot: time.Date(2026, 10, 11, 8, 0, 0, 0, loc),
		},
		{
			name:    "all invalid times does not trigger",
			times:   []string{"bad", "99:99", "xx:yy"},
			lastRun: time.Time{},
			wantHit: false,
		},
		{
			name:    "empty times does not trigger",
			times:   []string{},
			lastRun: time.Time{},
			wantHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSlot, gotHit := dueSlot(tt.times, tt.lastRun, now)
			if gotHit != tt.wantHit {
				t.Fatalf("dueSlot hit = %v, want %v", gotHit, tt.wantHit)
			}
			if gotHit && !gotSlot.Equal(tt.wantSlot) {
				t.Errorf("dueSlot slot = %v, want %v", gotSlot, tt.wantSlot)
			}
		})
	}
}

// 前台编辑租约生效（coord.ErrBusy）时：跳过该作品、不记录 last_run（下一趟重试）、
// 不告警、不暂停自动化。
func TestRunOnce_AutomationBusySkip(t *testing.T) {
	now := time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)
	st := newFakeStore()
	st.automations = []*domain.ProjectAutomation{
		{
			ProjectID:    "proj_edit",
			ProjectTitle: "正在被编辑的书",
			Enabled:      true,
			Times:        []string{"09:00"},
			QuotaKind:    domain.QuotaKindChapters,
			QuotaCount:   1,
		},
	}
	cont := &fakeContinuationRunner{
		fn: func(ctx context.Context, projectID string, kind domain.AutomationQuotaKind, count int) (bool, string, error) {
			return false, "", coord.ErrBusy
		},
	}
	book := &fakeBookRunner{}
	n := &fakeNotifier{}

	sched := New(st, cont, book, n, Options{
		Now: func() time.Time { return now },
	})

	count, err := sched.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected RunOnce err: %v", err)
	}
	if count != 0 {
		t.Fatalf("busy project must not be counted as executed, got %d", count)
	}
	if len(st.recordedRuns) != 0 {
		t.Fatalf("busy skip must not record a run (due slot retries next tick), got %+v", st.recordedRuns)
	}
	if len(st.pausedCalls) != 0 {
		t.Fatalf("busy skip must not pause automation, got %+v", st.pausedCalls)
	}
	if len(n.titles) != 0 {
		t.Fatalf("busy skip must not notify, got %+v", n.titles)
	}
}

func TestRunOnce_AutomationOutcomes(t *testing.T) {
	now := time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)

	// 1. Test halted outcome
	t.Run("halted branch sets paused, records halted, and notifies", func(t *testing.T) {
		st := newFakeStore()
		st.automations = []*domain.ProjectAutomation{
			{
				ProjectID:    "proj_halt",
				ProjectTitle: "停线书",
				Enabled:      true,
				Times:        []string{"09:00"},
				QuotaKind:    domain.QuotaKindChapters,
				QuotaCount:   1,
			},
		}

		cont := &fakeContinuationRunner{
			fn: func(ctx context.Context, projectID string, kind domain.AutomationQuotaKind, count int) (bool, string, error) {
				return true, "第 3 章停线待人工（D/55 分）", nil
			},
		}
		book := &fakeBookRunner{}
		n := &fakeNotifier{}

		sched := New(st, cont, book, n, Options{
			Now: func() time.Time { return now },
		})

		count, err := sched.RunOnce(context.Background())
		if err != nil {
			t.Fatalf("unexpected RunOnce err: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 task executed, got %d", count)
		}

		// Check pause call
		if len(st.pausedCalls) != 1 {
			t.Fatalf("expected 1 pause call, got %d", len(st.pausedCalls))
		}
		if !st.pausedCalls[0].paused || st.pausedCalls[0].projectID != "proj_halt" {
			t.Errorf("unexpected pause call: %+v", st.pausedCalls[0])
		}

		// Check run record
		if len(st.recordedRuns) != 1 || st.recordedRuns[0].status != "halted" {
			t.Fatalf("expected recorded status halted, got %+v", st.recordedRuns)
		}

		// Check notification
		if len(n.titles) != 1 || n.titles[0] != "NovelStudio 停线待人工" {
			t.Fatalf("expected notification 'NovelStudio 停线待人工', got %+v", n.titles)
		}
	})

	// 2. Test error outcome
	t.Run("error branch records error, notifies, but does NOT set paused", func(t *testing.T) {
		st := newFakeStore()
		st.automations = []*domain.ProjectAutomation{
			{
				ProjectID:    "proj_err",
				ProjectTitle: "出错书",
				Enabled:      true,
				Times:        []string{"09:00"},
				QuotaKind:    domain.QuotaKindChapters,
				QuotaCount:   1,
			},
		}

		cont := &fakeContinuationRunner{
			fn: func(ctx context.Context, projectID string, kind domain.AutomationQuotaKind, count int) (bool, string, error) {
				return false, "", errors.New("network timeout")
			},
		}
		book := &fakeBookRunner{}
		n := &fakeNotifier{}

		sched := New(st, cont, book, n, Options{
			Now: func() time.Time { return now },
		})

		count, err := sched.RunOnce(context.Background())
		if err != nil {
			t.Fatalf("unexpected RunOnce err: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 task executed, got %d", count)
		}

		if len(st.pausedCalls) != 0 {
			t.Errorf("expected NO pause calls on technical error, got %d", len(st.pausedCalls))
		}
		if len(st.recordedRuns) != 1 || st.recordedRuns[0].status != "error" {
			t.Fatalf("expected recorded status error, got %+v", st.recordedRuns)
		}
		if len(n.titles) != 1 || n.titles[0] != "NovelStudio 自动批产失败" {
			t.Fatalf("expected notification 'NovelStudio 自动批产失败', got %+v", n.titles)
		}
	})

	// 3. Test ok outcome
	t.Run("ok branch records ok and does NOT notify", func(t *testing.T) {
		st := newFakeStore()
		st.automations = []*domain.ProjectAutomation{
			{
				ProjectID:    "proj_ok",
				ProjectTitle: "成功书",
				Enabled:      true,
				Times:        []string{"09:00"},
				QuotaKind:    domain.QuotaKindChapters,
				QuotaCount:   1,
			},
		}

		cont := &fakeContinuationRunner{
			fn: func(ctx context.Context, projectID string, kind domain.AutomationQuotaKind, count int) (bool, string, error) {
				return false, "已续产 1 章", nil
			},
		}
		book := &fakeBookRunner{}
		n := &fakeNotifier{}

		sched := New(st, cont, book, n, Options{
			Now: func() time.Time { return now },
		})

		count, err := sched.RunOnce(context.Background())
		if err != nil {
			t.Fatalf("unexpected RunOnce err: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 task executed, got %d", count)
		}

		if len(st.pausedCalls) != 0 {
			t.Errorf("expected NO pause calls on success, got %d", len(st.pausedCalls))
		}
		if len(st.recordedRuns) != 1 || st.recordedRuns[0].status != "ok" {
			t.Fatalf("expected recorded status ok, got %+v", st.recordedRuns)
		}
		if len(n.titles) != 0 {
			t.Fatalf("expected NO notification on success, got %+v", n.titles)
		}
	})

	// 4. Test paused / disabled skipped
	t.Run("paused and disabled automations are skipped", func(t *testing.T) {
		st := newFakeStore()
		st.automations = []*domain.ProjectAutomation{
			{
				ProjectID: "proj_paused",
				Enabled:   true,
				Paused:    true,
				Times:     []string{"09:00"},
			},
			{
				ProjectID: "proj_disabled",
				Enabled:   false,
				Paused:    false,
				Times:     []string{"09:00"},
			},
		}

		cont := &fakeContinuationRunner{}
		book := &fakeBookRunner{}
		n := &fakeNotifier{}

		sched := New(st, cont, book, n, Options{
			Now: func() time.Time { return now },
		})

		count, err := sched.RunOnce(context.Background())
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 tasks, got %d", count)
		}
		if len(cont.calls) != 0 {
			t.Errorf("expected no runner calls, got %v", cont.calls)
		}
	})
}

func TestRunOnce_Queue(t *testing.T) {
	now := time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)

	t.Run("two pending items only runs first item FIFO and updates running/done status", func(t *testing.T) {
		st := newFakeStore()
		st.queue = []*domain.BookQueueItem{
			{
				ID:       "queue_1",
				Title:    "第一本书",
				Status:   domain.QueueStatusPending,
				Volumes:  1,
				AutoLock: true,
			},
			{
				ID:       "queue_2",
				Title:    "第二本书",
				Status:   domain.QueueStatusPending,
				Volumes:  1,
				AutoLock: true,
			},
		}

		cont := &fakeContinuationRunner{}
		book := &fakeBookRunner{}
		n := &fakeNotifier{}

		sched := New(st, cont, book, n, Options{
			Now: func() time.Time { return now },
		})
		sched.SetQueueTimes([]string{"09:00"})

		count, err := sched.RunOnce(context.Background())
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 task executed, got %d", count)
		}

		// Only the first item should have run
		if len(book.calls) != 1 || book.calls[0] != "queue_1" {
			t.Fatalf("expected queue_1 to run, got calls: %v", book.calls)
		}

		// State queue_last_run must be recorded
		if st.state["queue_last_run"] != now.Format(time.RFC3339) {
			t.Errorf("expected queue_last_run state %s, got %s", now.Format(time.RFC3339), st.state["queue_last_run"])
		}

		// queue_1 should be done, queue_2 still pending
		if st.queue[0].Status != domain.QueueStatusDone {
			t.Errorf("expected queue_1 done, got %s", st.queue[0].Status)
		}
		if st.queue[1].Status != domain.QueueStatusPending {
			t.Errorf("expected queue_2 pending, got %s", st.queue[1].Status)
		}
	})

	t.Run("queue slot consumed even if queue is empty", func(t *testing.T) {
		st := newFakeStore()
		cont := &fakeContinuationRunner{}
		book := &fakeBookRunner{}
		n := &fakeNotifier{}

		sched := New(st, cont, book, n, Options{
			Now: func() time.Time { return now },
		})
		sched.SetQueueTimes([]string{"09:00"})

		count, err := sched.RunOnce(context.Background())
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 task, got %d", count)
		}

		// queue_last_run state must still be written
		if st.state["queue_last_run"] != now.Format(time.RFC3339) {
			t.Errorf("expected queue_last_run state %s, got %s", now.Format(time.RFC3339), st.state["queue_last_run"])
		}
	})

	t.Run("queue error sets status error and notifies", func(t *testing.T) {
		st := newFakeStore()
		st.queue = []*domain.BookQueueItem{
			{
				ID:     "queue_err",
				Title:  "错书",
				Status: domain.QueueStatusPending,
			},
		}

		cont := &fakeContinuationRunner{}
		book := &fakeBookRunner{
			fn: func(ctx context.Context, item *domain.BookQueueItem) (bool, string, string, error) {
				return false, "", "", errors.New("pipeline crashed")
			},
		}
		n := &fakeNotifier{}

		sched := New(st, cont, book, n, Options{
			Now: func() time.Time { return now },
		})
		sched.SetQueueTimes([]string{"09:00"})

		count, err := sched.RunOnce(context.Background())
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 task, got %d", count)
		}

		if st.queue[0].Status != domain.QueueStatusError {
			t.Errorf("expected queue item status error, got %s", st.queue[0].Status)
		}
		if len(n.titles) != 1 || n.titles[0] != "NovelStudio 开书失败" {
			t.Errorf("expected notification 'NovelStudio 开书失败', got %+v", n.titles)
		}
	})

	t.Run("queue halted sets status halted and notifies", func(t *testing.T) {
		st := newFakeStore()
		st.queue = []*domain.BookQueueItem{
			{
				ID:     "queue_halt",
				Title:  "停书",
				Status: domain.QueueStatusPending,
			},
		}

		cont := &fakeContinuationRunner{}
		book := &fakeBookRunner{
			fn: func(ctx context.Context, item *domain.BookQueueItem) (bool, string, string, error) {
				return true, "proj_halted", "第 2 卷停线", nil
			},
		}
		n := &fakeNotifier{}

		sched := New(st, cont, book, n, Options{
			Now: func() time.Time { return now },
		})
		sched.SetQueueTimes([]string{"09:00"})

		count, err := sched.RunOnce(context.Background())
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 task, got %d", count)
		}

		if st.queue[0].Status != domain.QueueStatusHalted {
			t.Errorf("expected queue item status halted, got %s", st.queue[0].Status)
		}
		if st.queue[0].ProjectID != "proj_halted" {
			t.Errorf("expected project ID proj_halted, got %s", st.queue[0].ProjectID)
		}
		if len(n.titles) != 1 || n.titles[0] != "NovelStudio 开书停线" {
			t.Errorf("expected notification 'NovelStudio 开书停线', got %+v", n.titles)
		}
	})
}

func TestRunOnce_SerialExecutionOrder(t *testing.T) {
	now := time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)
	st := newFakeStore()
	st.automations = []*domain.ProjectAutomation{
		{
			ProjectID:    "proj_1",
			ProjectTitle: "作品一",
			Enabled:      true,
			Times:        []string{"09:00"},
			QuotaKind:    domain.QuotaKindChapters,
			QuotaCount:   1,
		},
		{
			ProjectID:    "proj_2",
			ProjectTitle: "作品二",
			Enabled:      true,
			Times:        []string{"09:00"},
			QuotaKind:    domain.QuotaKindChapters,
			QuotaCount:   1,
		},
	}
	st.queue = []*domain.BookQueueItem{
		{
			ID:     "queue_item_1",
			Title:  "队列新书",
			Status: domain.QueueStatusPending,
		},
	}

	var executionLog []string
	var mu sync.Mutex

	cont := &fakeContinuationRunner{
		fn: func(ctx context.Context, projectID string, kind domain.AutomationQuotaKind, count int) (bool, string, error) {
			mu.Lock()
			executionLog = append(executionLog, "auto:"+projectID)
			mu.Unlock()
			return false, "ok", nil
		},
	}
	book := &fakeBookRunner{
		fn: func(ctx context.Context, item *domain.BookQueueItem) (bool, string, string, error) {
			mu.Lock()
			executionLog = append(executionLog, "queue:"+item.ID)
			mu.Unlock()
			return false, "proj_new", "ok", nil
		},
	}

	sched := New(st, cont, book, nil, Options{
		Now: func() time.Time { return now },
	})
	sched.SetQueueTimes([]string{"09:00"})

	count, err := sched.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 tasks executed, got %d", count)
	}

	// Execution order: auto:proj_1, auto:proj_2, queue:queue_item_1
	expectedLog := []string{"auto:proj_1", "auto:proj_2", "queue:queue_item_1"}
	if len(executionLog) != 3 {
		t.Fatalf("expected 3 executed calls, got %v", executionLog)
	}
	for i := range expectedLog {
		if executionLog[i] != expectedLog[i] {
			t.Errorf("step %d got %s, want %s", i, executionLog[i], expectedLog[i])
		}
	}
}

func TestStart_Lifecycle(t *testing.T) {
	st := newFakeStore()
	st.queue = []*domain.BookQueueItem{
		{
			ID:     "stale_item",
			Title:  "卡住的条目",
			Status: domain.QueueStatusRunning,
		},
	}

	cont := &fakeContinuationRunner{}
	book := &fakeBookRunner{}

	sched := New(st, cont, book, nil, Options{
		TickInterval: 10 * time.Millisecond,
		Now: func() time.Time {
			return time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)
		},
	})
	sched.SetQueueTimes([]string{"09:00"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		sched.Start(ctx)
		close(done)
	}()

	// Wait briefly for reset and catch-up to finish
	time.Sleep(50 * time.Millisecond)

	// Verify stale running item was reset to pending (and then potentially executed by catch-up)
	if st.StaleResetCount() != 1 {
		t.Errorf("expected 1 stale running item reset, got %d", st.StaleResetCount())
	}

	cancel()
	select {
	case <-done:
		// Clean exit
	case <-time.After(1 * time.Second):
		t.Fatal("Start did not exit after context cancellation")
	}
}
