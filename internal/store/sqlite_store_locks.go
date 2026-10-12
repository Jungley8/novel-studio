package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Jungley8/novel-studio/internal/coord"
	"github.com/Jungley8/novel-studio/internal/domain"
)

// -------------------------------------------------------------
// Coordination (跨进程作品级写入互斥：编辑租约 / 生产锁)
// 守护服务与 CLI 共享同一 SQLite，任意一方可看见彼此的编辑/生产状态；
// 进程异常退出由 expires_at 过期自动清理（惰性 + 启动期显式清理）。
// -------------------------------------------------------------

// UpsertEditLease 写入/续期前台编辑租约，返回新的到期时间。
func (s *SQLiteStore) UpsertEditLease(ctx context.Context, projectID string, ttl time.Duration) (time.Time, error) {
	now := time.Now()
	expires := now.Add(ttl)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO coordination_leases (project_id, kind, holder, label, chapter, updated_at, expires_at)
		VALUES (?, ?, '', '', 0, ?, ?)
		ON CONFLICT(project_id, kind) DO UPDATE SET
			updated_at = excluded.updated_at,
			expires_at = excluded.expires_at;
	`, projectID, coord.KindEdit, now, expires)
	if err != nil {
		return time.Time{}, fmt.Errorf("upsert edit lease: %w", err)
	}
	return expires, nil
}

// DeleteEditLease 释放编辑租约（不存在时无操作）。
func (s *SQLiteStore) DeleteEditLease(ctx context.Context, projectID string) error {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM coordination_leases WHERE project_id = ? AND kind = ?`,
		projectID, coord.KindEdit,
	); err != nil {
		return fmt.Errorf("delete edit lease: %w", err)
	}
	return nil
}

// GetLiveLease 返回未过期的指定租约；不存在或已过期时返回 (nil, nil)。
func (s *SQLiteStore) GetLiveLease(ctx context.Context, projectID, kind string) (*domain.CoordinationLease, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT project_id, kind, holder, label, chapter, updated_at, expires_at
		FROM coordination_leases
		WHERE project_id = ? AND kind = ? AND expires_at > ?
	`, projectID, kind, time.Now())

	var l domain.CoordinationLease
	if err := row.Scan(&l.ProjectID, &l.Kind, &l.Holder, &l.Label, &l.Chapter, &l.UpdatedAt, &l.ExpiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get live lease: %w", err)
	}
	return &l, nil
}

// AcquireProductionLease 原子获取生产锁（BEGIN IMMEDIATE，跨进程安全）：
//   - 存在未过期的编辑租约或生产锁 → 返回 (阻塞租约, coord.ErrBusy)；
//   - 过期遗留先清理，再写入新锁（holder token 用于后续续期/释放的代际隔离）。
func (s *SQLiteStore) AcquireProductionLease(ctx context.Context, projectID, holder, label string, chapter int, ttl time.Duration) (*domain.CoordinationLease, error) {
	if projectID == "" || holder == "" {
		return nil, errors.New("production lease requires project_id and holder token")
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire lease connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return nil, fmt.Errorf("begin immediate tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()

	now := time.Now()
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM coordination_leases WHERE project_id = ? AND expires_at <= ?`,
		projectID, now,
	); err != nil {
		return nil, fmt.Errorf("cleanup expired leases: %w", err)
	}

	var blocking domain.CoordinationLease
	err = conn.QueryRowContext(ctx, `
		SELECT project_id, kind, holder, label, chapter, updated_at, expires_at
		FROM coordination_leases WHERE project_id = ? LIMIT 1`,
		projectID,
	).Scan(&blocking.ProjectID, &blocking.Kind, &blocking.Holder, &blocking.Label,
		&blocking.Chapter, &blocking.UpdatedAt, &blocking.ExpiresAt)
	if err == nil {
		return &blocking, coord.ErrBusy
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("scan blocking lease: %w", err)
	}

	if _, err := conn.ExecContext(ctx, `
		INSERT INTO coordination_leases (project_id, kind, holder, label, chapter, updated_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		projectID, coord.KindProduction, holder, label, chapter, now, now.Add(ttl),
	); err != nil {
		return nil, fmt.Errorf("insert production lease: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return nil, fmt.Errorf("commit lease tx: %w", err)
	}
	committed = true
	return nil, nil
}

// HeartbeatProductionLease 续期生产锁并更新当前生产章节。
// 返回 false 表示锁已不属于该 holder（超时被接管或已释放），生产方应视为锁丢失。
func (s *SQLiteStore) HeartbeatProductionLease(ctx context.Context, projectID, holder string, chapter int, ttl time.Duration) (bool, error) {
	now := time.Now()
	res, err := s.db.ExecContext(ctx, `
		UPDATE coordination_leases SET chapter = ?, updated_at = ?, expires_at = ?
		WHERE project_id = ? AND kind = ? AND holder = ?`,
		chapter, now, now.Add(ttl), projectID, coord.KindProduction, holder,
	)
	if err != nil {
		return false, fmt.Errorf("heartbeat production lease: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("heartbeat rows affected: %w", err)
	}
	return affected > 0, nil
}

// ReleaseProductionLease 释放生产锁（仅 holder 匹配时生效，代际隔离）。
func (s *SQLiteStore) ReleaseProductionLease(ctx context.Context, projectID, holder string) error {
	if _, err := s.db.ExecContext(ctx, `
		DELETE FROM coordination_leases
		WHERE project_id = ? AND kind = ? AND holder = ?`,
		projectID, coord.KindProduction, holder,
	); err != nil {
		return fmt.Errorf("release production lease: %w", err)
	}
	return nil
}

// CleanupExpiredLeases 清理全部过期租约（启动期调用；返回清理行数）。
func (s *SQLiteStore) CleanupExpiredLeases(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM coordination_leases WHERE expires_at <= ?`, time.Now())
	if err != nil {
		return 0, fmt.Errorf("cleanup expired leases: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("cleanup rows affected: %w", err)
	}
	return affected, nil
}
