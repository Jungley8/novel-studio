package coord

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// LeaseStore 是协调租约的持久化面（由 store.SQLiteStore 实现）。
type LeaseStore interface {
	AcquireProductionLease(ctx context.Context, projectID, holder, label string, chapter int, ttl time.Duration) (*domain.CoordinationLease, error)
	HeartbeatProductionLease(ctx context.Context, projectID, holder string, chapter int, ttl time.Duration) (bool, error)
	ReleaseProductionLease(ctx context.Context, projectID, holder string) error
}

// heartbeatInterval 是生产锁续期间隔；测试可覆盖。
var heartbeatInterval = ProductionHeartbeatInterval

// productionTokenSeq 保证同纳秒内多次获取的 holder token 不重复。
var productionTokenSeq atomic.Int64

// Production 持有某作品的生产锁并自动心跳续期。
// 生命周期：AcquireProduction →（SetChapter 跟踪进度）→ Release。
// 进程崩溃兜底：心跳停止后锁按 ProductionLeaseTTL 自动过期。
type Production struct {
	store     LeaseStore
	projectID string
	token     string
	label     string
	chapter   atomic.Int64
	cancel    context.CancelFunc
	done      chan struct{}
}

// AcquireProduction 获取生产锁并启动心跳（holder token 代际隔离）。
// 被占用时返回 (nil, 阻塞租约, ErrBusy)，阻塞租约携带对方标签与当前章节。
func AcquireProduction(ctx context.Context, store LeaseStore, projectID, label string, chapter int) (*Production, *domain.CoordinationLease, error) {
	if store == nil || projectID == "" {
		return nil, nil, errors.New("coord: acquire production requires store and project id")
	}
	token := fmt.Sprintf("prod-%d-%d", time.Now().UnixNano(), productionTokenSeq.Add(1))
	blocking, err := store.AcquireProductionLease(ctx, projectID, token, label, chapter, ProductionLeaseTTL)
	if err != nil {
		return nil, blocking, err
	}
	hbCtx, cancel := context.WithCancel(context.Background())
	p := &Production{
		store:     store,
		projectID: projectID,
		token:     token,
		label:     label,
		cancel:    cancel,
		done:      make(chan struct{}),
	}
	p.chapter.Store(int64(chapter))
	go p.heartbeat(hbCtx)
	return p, nil, nil
}

// BindProduction 组装“流水线创世即登记生产锁”的回调三元组：
//   - hook 传给 PipelineRequest.OnProjectCreated：项目创世入库时点获取生产锁，
//     被占用则返回错误、流水线随之中止（Fail-Closed）；
//   - trackChapter 传给进度回调：跟随当前章节号（随心跳写入，供其它进程展示）；
//   - release 由调用方 defer：释放生产锁（幂等，未成功获取时为空操作）。
func BindProduction(store LeaseStore, label string) (hook func(ctx context.Context, projectID string) error, trackChapter func(chapter int), release func()) {
	var prod *Production
	hook = func(ctx context.Context, projectID string) error {
		p, blocking, err := AcquireProduction(ctx, store, projectID, label, 0)
		if err != nil {
			if errors.Is(err, ErrBusy) && blocking != nil {
				return fmt.Errorf("作品已有在途任务（%s），已拒绝并发写入（Fail-Closed）: %w", blocking.Label, err)
			}
			return fmt.Errorf("登记生产锁失败: %w", err)
		}
		prod = p
		return nil
	}
	trackChapter = func(chapter int) {
		if prod != nil {
			prod.SetChapter(chapter)
		}
	}
	release = func() {
		if prod != nil {
			prod.Release()
		}
	}
	return hook, trackChapter, release
}

// SetChapter 记录当前生产章节（随心跳写入；<=0 忽略）。
func (p *Production) SetChapter(chapter int) {
	if p == nil || chapter <= 0 {
		return
	}
	p.chapter.Store(int64(chapter))
}

// Release 停止心跳并释放生产锁；可安全重复调用。
func (p *Production) Release() {
	if p == nil {
		return
	}
	p.cancel()
	<-p.done
	_ = p.store.ReleaseProductionLease(context.Background(), p.projectID, p.token)
}

func (p *Production) heartbeat(ctx context.Context) {
	defer close(p.done)
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 续期失败（锁被接管 / DB 异常）不中断生产：锁由 TTL 与 holder 代际隔离兜底，
			// 生产本身仍由调用方 ctx 控制；下一轮心跳自动重试。
			_, _ = p.store.HeartbeatProductionLease(ctx, p.projectID, p.token, int(p.chapter.Load()), ProductionLeaseTTL)
		}
	}
}
