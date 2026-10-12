// Package coord 提供进程内的作品级写入互斥原语：前台人工编辑租约 + 后台生产锁。
//
// 单二进制内，无人值守调度器与桌面 UI 共享同一进程，两者对同一作品
// 的并发写入必须显式互斥（Fail-Closed）：
//   - 前台在途编辑期间持有「编辑租约」（带 TTL 的心跳续期）；
//   - 调度器/一键成章开始生产前必须获取「生产锁」，租约未过期则拒绝启动；
//   - 章节定稿/撤回归档等正史写入口在生产锁生效期间一律拒绝（409）。
package coord

import (
	"errors"
	"sync"
	"time"
)

// ErrBusy 表示目标作品正被人工编辑或已有生产任务在跑；
// 调用方应将其视为“本轮跳过/稍后重试”，而不是技术失败。
var ErrBusy = errors.New("project busy: production or edit lease active")

// ProductionInfo 描述一条在途生产任务。
type ProductionInfo struct {
	Label   string    // 任务描述，如「排程自动续产」「一键成章」
	Chapter int       // 当前生产章节号（0 表示未知/整卷）
	Started time.Time // 开始时间
}

type productionState struct {
	info  ProductionInfo
	token int64
}

// Coordinator 是并发安全的作品级协调器；零值不可用，请用 New。
type Coordinator struct {
	mu          sync.Mutex
	now         func() time.Time
	nextToken   int64
	editLeases  map[string]time.Time
	productions map[string]*productionState
}

// New 创建协调器（真实时钟）。
func New() *Coordinator {
	return NewWithClock(time.Now)
}

// NewWithClock 创建协调器并注入时钟（测试用）。
func NewWithClock(now func() time.Time) *Coordinator {
	if now == nil {
		now = time.Now
	}
	return &Coordinator{
		now:         now,
		editLeases:  make(map[string]time.Time),
		productions: make(map[string]*productionState),
	}
}

// AcquireEdit 获取/续期指定作品的编辑租约，返回新的到期时间。
// 允许在生产锁生效期间获取：租约表达“人工在场”，冲突在具体写入口裁决。
func (c *Coordinator) AcquireEdit(projectID string, ttl time.Duration) time.Time {
	if ttl <= 0 {
		ttl = 120 * time.Second
	}
	expiry := c.now().Add(ttl)
	c.mu.Lock()
	c.editLeases[projectID] = expiry
	c.mu.Unlock()
	return expiry
}

// ReleaseEdit 释放编辑租约（不存在时无操作）。
func (c *Coordinator) ReleaseEdit(projectID string) {
	c.mu.Lock()
	delete(c.editLeases, projectID)
	c.mu.Unlock()
}

// EditActive 判断编辑租约是否仍在有效期内；过期即惰性清理。
func (c *Coordinator) EditActive(projectID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	expiry, ok := c.editLeases[projectID]
	if !ok {
		return false
	}
	if !c.now().Before(expiry) {
		delete(c.editLeases, projectID)
		return false
	}
	return true
}

// BeginProduction 尝试为指定作品开启生产任务（调度器或一键成章）。
// 若该作品存在未过期的编辑租约或已有生产任务，返回 ErrBusy。
// 成功时必须 defer 调用返回的 release 释放；release 具备幂等与代际隔离
// （旧任务的 release 不会误删后续新任务）。
func (c *Coordinator) BeginProduction(projectID, label string) (release func(), err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if expiry, ok := c.editLeases[projectID]; ok {
		if c.now().Before(expiry) {
			return nil, ErrBusy
		}
		delete(c.editLeases, projectID)
	}
	if _, ok := c.productions[projectID]; ok {
		return nil, ErrBusy
	}

	c.nextToken++
	token := c.nextToken
	c.productions[projectID] = &productionState{
		info:  ProductionInfo{Label: label, Started: c.now()},
		token: token,
	}

	release = func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if st, ok := c.productions[projectID]; ok && st.token == token {
			delete(c.productions, projectID)
		}
	}
	return release, nil
}

// SetProductionChapter 更新当前生产章节号（0 = 未知/整卷）。
func (c *Coordinator) SetProductionChapter(projectID string, chapter int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if st, ok := c.productions[projectID]; ok {
		st.info.Chapter = chapter
	}
}

// Production 返回指定作品的在途生产任务；无任务时 active=false。
func (c *Coordinator) Production(projectID string) (ProductionInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	st, ok := c.productions[projectID]
	if !ok {
		return ProductionInfo{}, false
	}
	return st.info, true
}
