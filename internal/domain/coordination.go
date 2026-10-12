package domain

import "time"

// CoordinationLease 是跨进程作品级写入互斥的租约记录：
//   - kind=edit：前台人工编辑租约（浏览器心跳续期），表达“人工在场”；
//   - kind=production：生产锁（守护服务排程/UI 一键成章/CLI 无头生产持有），
//     单生产者，按 holder token 隔离续期与释放。
//
// 记录存于 SQLite（coordination_leases 表），守护服务与 CLI 共享同一数据面；
// 进程异常退出由 expires_at 过期自动清理。
type CoordinationLease struct {
	ProjectID string    `json:"project_id"`
	Kind      string    `json:"kind"`
	Holder    string    `json:"holder"`
	Label     string    `json:"label"`
	Chapter   int       `json:"chapter"`
	UpdatedAt time.Time `json:"updated_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
