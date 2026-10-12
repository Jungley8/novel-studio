package engine_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/coord"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/store"
)

// 创世回调钩子（OnProjectCreated）：开书流水线在此登记生产锁；最早可知时点、早于任何项目内写入。

func TestStudioPipeline_OnProjectCreatedHook(t *testing.T) {
	ctx := context.Background()
	llm := &fakePipelineLLM{}
	orch := engine.NewOrchestrator(llm)
	producer := &fakeVolumeProducer{}
	st := newFakePipelineStore()

	pipeline := engine.NewStudioPipeline(orch, producer, st)

	var hookCalls []string
	producerCallsAtHook := -1
	savedAtHook := false

	req := engine.PipelineRequest{
		Title:           "钩子测试",
		Concept:         "验证创世回调时点",
		TargetPlatform:  "起点仙侠",
		ReasoningModel:  "deepseek-reasoner",
		Volumes:         1,
		AutoLockIntents: true,
		OnProjectCreated: func(ctx context.Context, projectID string) error {
			hookCalls = append(hookCalls, projectID)
			producer.mu.Lock()
			producerCallsAtHook = len(producer.calls)
			producer.mu.Unlock()
			if _, err := st.GetProject(ctx, projectID); err == nil {
				savedAtHook = true
			}
			return nil
		},
	}

	res, err := pipeline.Run(ctx, req)
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}
	if len(hookCalls) != 1 {
		t.Fatalf("hook must fire exactly once, got %v", hookCalls)
	}
	if hookCalls[0] != res.ProjectID {
		t.Fatalf("hook project id %q != result project id %q", hookCalls[0], res.ProjectID)
	}
	if !savedAtHook {
		t.Fatal("project must be committed before hook fires")
	}
	if producerCallsAtHook != 0 {
		t.Fatalf("hook must fire before any volume production, producer calls=%d", producerCallsAtHook)
	}
}

func TestStudioPipeline_OnProjectCreatedHookError_AbortsBeforeProduction(t *testing.T) {
	ctx := context.Background()
	llm := &fakePipelineLLM{}
	orch := engine.NewOrchestrator(llm)
	producer := &fakeVolumeProducer{}
	st := newFakePipelineStore()

	pipeline := engine.NewStudioPipeline(orch, producer, st)
	hookErr := errors.New("production lease busy")

	var hookedProjectID string
	var hookProjectSaved bool
	res, err := pipeline.Run(ctx, engine.PipelineRequest{
		Title:           "钩子失败测试",
		Concept:         "回调失败须中止流水线",
		ReasoningModel:  "deepseek-reasoner",
		Volumes:         1,
		AutoLockIntents: true,
		OnProjectCreated: func(ctx context.Context, projectID string) error {
			hookedProjectID = projectID
			if _, err := st.GetProject(ctx, projectID); err == nil {
				hookProjectSaved = true
			}
			return hookErr
		},
	})
	if err == nil || !strings.Contains(err.Error(), hookErr.Error()) {
		t.Fatalf("expected hook error to abort pipeline, got %v", err)
	}
	if res != nil {
		t.Fatalf("result must be nil on hook failure, got %+v", res)
	}
	producer.mu.Lock()
	producerCalls := len(producer.calls)
	producer.mu.Unlock()
	if producerCalls != 0 {
		t.Fatalf("volume production must not start after hook failure, got %d calls", producerCalls)
	}
	// 文档化的半状态：项目已入库（hook 在 SaveProject 之后触发），种子未注入
	if !hookProjectSaved {
		t.Fatalf("hook project %q should already be committed, fail-closed aborts after that point", hookedProjectID)
	}
}

// 端到端：真实 SQLite + coord.BindProduction —— 流水线生产期间生产锁对其它进程可见，
// 结束后由调用方释放。
func TestStudioPipeline_ProjectCreatedHookAcquiresProductionLease(t *testing.T) {
	ctx := context.Background()
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "pipe.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	hook, trackChapter, release := coord.BindProduction(st, "测试开书")
	defer release()

	var hookProjectID string
	leaseVisibleInProduction := false

	llm := &fakePipelineLLM{}
	orch := engine.NewOrchestrator(llm)
	producer := &fakeVolumeProducer{produceFunc: func(ctx context.Context, req engine.VolumeProduceRequest) (*engine.VolumeProduceResult, error) {
		// 生产进行中：其它进程视角必须看到生产锁在位（ErrBusy，携带标签）
		blocking, err := st.AcquireProductionLease(ctx, req.ProjectID, "probe-holder", "探针", 0, time.Minute)
		if !errors.Is(err, coord.ErrBusy) || blocking == nil {
			return nil, fmt.Errorf("expected ErrBusy during production, blocking=%+v err=%v", blocking, err)
		}
		if blocking.Label != "测试开书" {
			return nil, fmt.Errorf("blocking lease should carry production label, got %+v", blocking)
		}
		leaseVisibleInProduction = true
		return &engine.VolumeProduceResult{
			VolumeIndex:  req.VolumeIndex,
			StartChapter: 1,
			EndChapter:   2,
			Chapters: []engine.VolumeChapterSummary{
				{ChapterIndex: 1, Committed: true, Score: 88},
			},
		}, nil
	}}

	pipeline := engine.NewStudioPipeline(orch, producer, st)
	res, err := pipeline.Run(ctx, engine.PipelineRequest{
		Title:           "锁联动测试",
		Concept:         "验证流水线持有生产锁",
		TargetPlatform:  "起点仙侠",
		ReasoningModel:  "deepseek-reasoner",
		Volumes:         1,
		AutoLockIntents: true,
		OnProjectCreated: func(ctx context.Context, projectID string) error {
			hookProjectID = projectID
			return hook(ctx, projectID)
		},
		OnProgress: func(ev engine.PipelineEvent) {
			trackChapter(ev.ChapterIndex)
		},
	})
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}
	if hookProjectID == "" || hookProjectID != res.ProjectID {
		t.Fatalf("hook project id %q != result project id %q", hookProjectID, res.ProjectID)
	}
	if !leaseVisibleInProduction {
		t.Fatal("production lease was not visible to other processes during volume production")
	}

	// 流水线结束后、release 之前：锁仍在（生命周期归调用方）
	if lease, err := st.GetLiveLease(ctx, res.ProjectID, coord.KindProduction); err != nil || lease == nil {
		t.Fatalf("lease must remain held after Run until caller releases, lease=%+v err=%v", lease, err)
	}
	release()
	if lease, _ := st.GetLiveLease(ctx, res.ProjectID, coord.KindProduction); lease != nil {
		t.Fatalf("lease must be released after release(), got %+v", lease)
	}
}
