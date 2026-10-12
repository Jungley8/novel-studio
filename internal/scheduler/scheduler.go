package scheduler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Jungley8/novel-studio/internal/coord"
	"github.com/Jungley8/novel-studio/internal/domain"
)

type ContinuationRunner interface {
	// 续产：kind=chapters → 连续生产 count 章；kind=volumes → 连续生产 count 卷。
	// halted=true 表示停线待人工（此时 err==nil）；err!=nil 为技术错误。
	RunContinuation(ctx context.Context, projectID string, kind domain.AutomationQuotaKind, count int, progress func(string)) (halted bool, message string, err error)
}

type BookRunner interface {
	// 开书：执行队列项完整 pipeline（创世→意图→自动锁→批产）。
	RunBook(ctx context.Context, item *domain.BookQueueItem, progress func(string)) (halted bool, projectID string, message string, err error)
}

type Store interface {
	ListProjectAutomations(ctx context.Context) ([]*domain.ProjectAutomation, error)
	SetAutomationPaused(ctx context.Context, projectID string, paused bool, reason string) error
	RecordAutomationRun(ctx context.Context, projectID, status, message string, at time.Time) error
	ListBookQueue(ctx context.Context) ([]*domain.BookQueueItem, error)
	SaveBookQueueItem(ctx context.Context, item *domain.BookQueueItem) error
	ResetStaleQueueRunning(ctx context.Context) (int64, error)
	GetSchedulerState(ctx context.Context, key string) (string, error)
	SetSchedulerState(ctx context.Context, key, value string) error
}

type Notifier interface {
	Notify(title, message string) error
}

type Options struct {
	TickInterval time.Duration                    // 默认 30s
	Now          func() time.Time                 // 默认 time.Now
	Logf         func(format string, args ...any) // 默认写 stderr
}

type Scheduler struct {
	store      Store
	cont       ContinuationRunner
	book       BookRunner
	notifier   Notifier
	opts       Options
	mu         sync.Mutex
	queueTimes []string
}

func New(st Store, cont ContinuationRunner, book BookRunner, n Notifier, opts Options) *Scheduler {
	if opts.TickInterval <= 0 {
		opts.TickInterval = 30 * time.Second
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Logf == nil {
		opts.Logf = func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, format+"\n", args...)
		}
	}
	return &Scheduler{
		store:    st,
		cont:     cont,
		book:     book,
		notifier: n,
		opts:     opts,
	}
}

// SetQueueTimes 并发安全地设置新书队列每日触发时刻列表（nil/空 = 队列不自动处理）。
func (s *Scheduler) SetQueueTimes(times []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if times == nil {
		s.queueTimes = nil
		return
	}
	cp := make([]string, len(times))
	copy(cp, times)
	s.queueTimes = cp
}

func (s *Scheduler) getQueueTimes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.queueTimes == nil {
		return nil
	}
	cp := make([]string, len(s.queueTimes))
	copy(cp, s.queueTimes)
	return cp
}

// Start 阻塞循环；先 ResetStaleQueueRunning（有重置则 log），启动即 RunOnce 一趟（catch-up），然后 ticker。
func (s *Scheduler) Start(ctx context.Context) {
	if count, err := s.store.ResetStaleQueueRunning(ctx); err != nil {
		s.opts.Logf("[scheduler] reset stale queue running failed: %v", err)
	} else if count > 0 {
		s.opts.Logf("[scheduler] reset %d stale running queue items to pending", count)
	}

	if _, err := s.RunOnce(ctx); err != nil {
		s.opts.Logf("[scheduler] initial run failed: %v", err)
	}

	ticker := time.NewTicker(s.opts.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.RunOnce(ctx); err != nil {
				s.opts.Logf("[scheduler] run failed: %v", err)
			}
		}
	}
}

// RunOnce 单趟执行，返回执行的任务数；可测试。
func (s *Scheduler) RunOnce(ctx context.Context) (int, error) {
	if s.store == nil || s.cont == nil || s.book == nil {
		return 0, errors.New("scheduler unconfigured: missing store or runner")
	}

	now := s.opts.Now()

	// 1. 遍历 ListProjectAutomations：跳过 !Enabled || Paused；lastRun 解析自 LastRunAt；dueSlot 命中收集
	automations, err := s.store.ListProjectAutomations(ctx)
	if err != nil {
		return 0, fmt.Errorf("list project automations: %w", err)
	}

	type autoTask struct {
		auto *domain.ProjectAutomation
		due  time.Time
	}
	var autoTasks []autoTask

	for _, a := range automations {
		if a == nil || !a.Enabled || a.Paused {
			continue
		}
		var lastRun time.Time
		if a.LastRunAt != "" {
			if t, err := time.Parse(time.RFC3339, a.LastRunAt); err == nil {
				lastRun = t
			}
		}
		if due, ok := dueSlot(a.Times, lastRun, now); ok {
			autoTasks = append(autoTasks, autoTask{auto: a, due: due})
		}
	}

	// 2. 队列：GetSchedulerState("queue_last_run") 解析 lastRun；dueSlot 命中
	type queueTask struct {
		item *domain.BookQueueItem
	}
	var qTask *queueTask

	qTimes := s.getQueueTimes()
	if len(qTimes) > 0 {
		var queueLastRun time.Time
		stateVal, err := s.store.GetSchedulerState(ctx, "queue_last_run")
		if err == nil && stateVal != "" {
			if t, err := time.Parse(time.RFC3339, stateVal); err == nil {
				queueLastRun = t
			}
		}
		if _, ok := dueSlot(qTimes, queueLastRun, now); ok {
			// 时刻即消费，无论队列是否为空——错误则跳过队列本趟
			if err := s.store.SetSchedulerState(ctx, "queue_last_run", now.Format(time.RFC3339)); err != nil {
				s.opts.Logf("[scheduler] set queue_last_run state failed: %v", err)
			} else {
				items, err := s.store.ListBookQueue(ctx)
				if err != nil {
					s.opts.Logf("[scheduler] list book queue failed: %v", err)
				} else {
					for _, it := range items {
						if it != nil && it.Status == domain.QueueStatusPending {
							qTask = &queueTask{item: it}
							break
						}
					}
				}
			}
		}
	}

	// 3. 串行执行任务（顺序：automation 任务在前，queue 在后）
	executedCount := 0

	for _, at := range autoTasks {
		if err := ctx.Err(); err != nil {
			return executedCount, err
		}

		a := at.auto
		pid := a.ProjectID
		title := a.ProjectTitle
		if title == "" {
			title = pid
		}

		progress := func(line string) {
			s.opts.Logf("[scheduler] %s", line)
		}

		halted, msg, err := s.cont.RunContinuation(ctx, pid, a.QuotaKind, a.QuotaCount, progress)
		if errors.Is(err, coord.ErrBusy) {
			// 作品正被人工编辑或已有生产任务在跑：本轮跳过，不记录 last_run，
			// 下一趟 tick 自动重试（用户收工 / 生产结束后自然衔接）。
			s.opts.Logf("[scheduler] skip project %s: %v", pid, err)
			continue
		}
		if err != nil {
			_ = s.store.RecordAutomationRun(ctx, pid, "error", err.Error(), now)
			s.notify("NovelStudio 自动批产失败", fmt.Sprintf("《%s》运行出错：%v", title, err))
		} else if halted {
			_ = s.store.RecordAutomationRun(ctx, pid, "halted", msg, now)
			_ = s.store.SetAutomationPaused(ctx, pid, true, msg)
			s.notify("NovelStudio 停线待人工", fmt.Sprintf("《%s》停线：%s", title, msg))
		} else {
			_ = s.store.RecordAutomationRun(ctx, pid, "ok", msg, now)
		}
		executedCount++
	}

	if qTask != nil {
		if err := ctx.Err(); err != nil {
			return executedCount, err
		}

		item := qTask.item
		item.Status = domain.QueueStatusRunning
		if err := s.store.SaveBookQueueItem(ctx, item); err != nil {
			s.opts.Logf("[scheduler] mark queue item running failed: %v", err)
		} else {
			progress := func(line string) {
				s.opts.Logf("[scheduler] %s", line)
			}
			halted, projectID, msg, err := s.book.RunBook(ctx, item, progress)
			if err != nil {
				item.Status = domain.QueueStatusError
				item.LastMessage = err.Error()
				_ = s.store.SaveBookQueueItem(ctx, item)
				s.notify("NovelStudio 开书失败", fmt.Sprintf("《%s》开书出错：%v", item.Title, err))
			} else if halted {
				item.Status = domain.QueueStatusHalted
				item.ProjectID = projectID
				item.LastMessage = msg
				_ = s.store.SaveBookQueueItem(ctx, item)
				s.notify("NovelStudio 开书停线", fmt.Sprintf("《%s》开书停线：%s", item.Title, msg))
			} else {
				item.Status = domain.QueueStatusDone
				item.ProjectID = projectID
				item.LastMessage = msg
				_ = s.store.SaveBookQueueItem(ctx, item)
			}
			executedCount++
		}
	}

	return executedCount, nil
}

func (s *Scheduler) notify(title, message string) {
	if s.notifier == nil {
		return
	}
	if err := s.notifier.Notify(title, message); err != nil {
		s.opts.Logf("[scheduler] notify error (%s: %s): %v", title, message, err)
	}
}

// dueSlot 纯函数：
// - 解析 "HH:MM" 为 now 所在时区的当日时刻；候选 = 今日与昨日的所有时刻。
// - due = 满足 S <= now && S > lastRun && S >= now-24h 的最大 S；返回 (S, true)；无 → (零值, false)。
// - lastRun 为零值（空/不可解析 RFC3339）→ 同样按上式（即最近 24h 内最近一个时刻会触发一次，catch-up）。
// - 非法时刻条目跳过；全非法 → 不触发。
// - 边界：lastRun == S → 不触发（严格大于）；S 恰好 24h 前 → 触发（含边界）；多条 times → 取最大（不重复触发多个）。
func dueSlot(times []string, lastRun, now time.Time) (time.Time, bool) {
	windowStart := now.Add(-24 * time.Hour)
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)

	var (
		found bool
		maxS  time.Time
	)

	for _, t := range times {
		for _, day := range []time.Time{today, yesterday} {
			slot, ok := parseDailySlot(t, day)
			if !ok {
				continue
			}
			// S <= now && S > lastRun && S >= now-24h
			if !slot.After(now) && slot.After(lastRun) && !slot.Before(windowStart) {
				if !found || slot.After(maxS) {
					found = true
					maxS = slot
				}
			}
		}
	}

	if !found {
		return time.Time{}, false
	}
	return maxS, true
}

func parseDailySlot(t string, date time.Time) (time.Time, bool) {
	if len(t) != 5 || t[2] != ':' {
		return time.Time{}, false
	}
	if t[0] < '0' || t[0] > '9' || t[1] < '0' || t[1] > '9' {
		return time.Time{}, false
	}
	hour := int(t[0]-'0')*10 + int(t[1]-'0')
	if hour < 0 || hour > 23 {
		return time.Time{}, false
	}
	if t[3] < '0' || t[3] > '9' || t[4] < '0' || t[4] > '9' {
		return time.Time{}, false
	}
	minute := int(t[3]-'0')*10 + int(t[4]-'0')
	if minute < 0 || minute > 59 {
		return time.Time{}, false
	}
	return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, date.Location()), true
}
