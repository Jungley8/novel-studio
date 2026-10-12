// Package coord 定义跨进程（守护服务 / CLI）作品级写入互斥的共享契约：
// 租约类型、错误哨兵与 TTL 常量。
//
// 单二进制可能以三种角色同时写同一作品：
//   - 守护服务内的无人值守调度器（自动续产）；
//   - 桌面 UI 的「一键成章」与人工定稿；
//   - CLI 无头生产（produce / 整卷批产）。
//
// 三者共享同一个 SQLite 数据库，协调记录落在 coordination_leases 表
// （见 store 的租约方法），确保任意进程都能看见彼此的编辑/生产状态：
//   - 前台在途编辑持有 edit 租约；
//   - 生产者必须先取得 production 锁（存在有效 edit 租约或已被占用则拒绝）。
package coord

import (
	"errors"
	"time"
)

// 租约类型。
const (
	KindEdit       = "edit"
	KindProduction = "production"
)

// ErrBusy 表示目标作品正被人工编辑或已有生产任务在跑；
// 调用方应将其视为“本轮跳过/稍后重试”，而不是技术失败。
var ErrBusy = errors.New("project busy: production or edit lease active")

const (
	// EditLeaseTTL 前台编辑租约的默认有效期（浏览器心跳续期）。
	EditLeaseTTL = 120 * time.Second
	// ProductionLeaseTTL 生产锁有效期；生产方须按 ProductionHeartbeatInterval 续期。
	ProductionLeaseTTL = 120 * time.Second
	// ProductionHeartbeatInterval 生产锁续期间隔（守护服务 / CLI / SSE 生产共用）。
	ProductionHeartbeatInterval = 30 * time.Second
)
