package engine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type PipelineRequest struct {
	Title, TargetPlatform, Concept, ReasoningModel string
	Volumes                                        int // 0 → 全部已规划卷；N>0 → 按 VolumeIndex 升序前 N 卷；N>len(arcs) → 校验错
	AutoLockIntents                                bool
	WordsTarget                                    int // <=0 → 2000
	MaxRewriteLoops                                int // <=0 → 3
	NarrativeStyle                                 string
	ChapterFrom                                    int
	RunID                                          string
	OnProgress                                     func(PipelineEvent)
}

type PipelineEvent struct {
	Phase        string `json:"phase"` // PIPELINE_START|FRAMEWORK_READY|RELATIONS_DONE|INTENTS_READY|INTENTS_LOCKED|VOLUME_DONE|HALTED|PIPELINE_COMPLETE
	VolumeIndex  int    `json:"volume_index,omitempty"`
	ChapterIndex int    `json:"chapter_index,omitempty"`
	Message      string `json:"message"`
	NeedsHuman   bool   `json:"needs_human,omitempty"`
}

type PipelineResult struct {
	ProjectID        string                 `json:"project_id"`
	Title            string                 `json:"title"`
	Volumes          []*VolumeProduceResult `json:"volumes"`
	TotalUsage       TokenUsage             `json:"total_usage"`
	EstimatedCostUSD float64                `json:"estimated_cost_usd"`
	Halted           bool                   `json:"halted"`
	HaltReason       string                 `json:"halt_reason,omitempty"`
	Notes            []string               `json:"notes,omitempty"`
}

type VolumeProducer interface {
	ProduceVolume(ctx context.Context, req VolumeProduceRequest) (*VolumeProduceResult, error)
}

type PipelineStore interface {
	GetProject(ctx context.Context, id string) (*domain.Project, error)
	SaveProject(ctx context.Context, p *domain.Project) error
	SaveCodexEntry(ctx context.Context, e *domain.CodexEntry) error
	SavePlotHook(ctx context.Context, h *domain.PlotHook) error
	ListChapterIntents(ctx context.Context, projectID string, volumeIndex int) ([]*domain.ChapterIntent, error)
	SaveChapterIntent(ctx context.Context, i *domain.ChapterIntent) error
	ListCodexEntries(ctx context.Context, projectID string, category domain.CodexCategory) ([]*domain.CodexEntry, error)
	SaveCodexRelation(ctx context.Context, projectID string, rel *domain.EntityRelation) error
}

type StudioPipeline struct {
	orch     *Orchestrator
	producer VolumeProducer
	store    PipelineStore
}

func NewStudioPipeline(orch *Orchestrator, producer VolumeProducer, store PipelineStore) *StudioPipeline {
	return &StudioPipeline{
		orch:     orch,
		producer: producer,
		store:    store,
	}
}

func addTokenUsage(dest *TokenUsage, src TokenUsage) {
	dest.PromptTokens += src.PromptTokens
	dest.CompletionTokens += src.CompletionTokens
	dest.TotalTokens += src.TotalTokens
}

func (p *StudioPipeline) Run(ctx context.Context, req PipelineRequest) (*PipelineResult, error) {
	// 1. 校验：Title/Concept 非空、Volumes>=0（错误直接 return nil, error）
	if strings.TrimSpace(req.Title) == "" {
		return nil, errors.New("pipeline: title cannot be empty")
	}
	if strings.TrimSpace(req.Concept) == "" {
		return nil, errors.New("pipeline: concept cannot be empty")
	}
	if req.Volumes < 0 {
		return nil, errors.New("pipeline: volumes must be >= 0")
	}

	result := &PipelineResult{
		Title:   req.Title,
		Volumes: make([]*VolumeProduceResult, 0),
		Notes:   make([]string, 0),
	}

	emit := func(ev PipelineEvent) {
		if req.OnProgress != nil {
			req.OnProgress(ev)
		}
	}

	// 2. emit PIPELINE_START；BootstrapFramework（用 req.ReasoningModel）
	emit(PipelineEvent{
		Phase:   "PIPELINE_START",
		Message: fmt.Sprintf("开始执行流水线: 《%s》", req.Title),
	})

	fw, fwUsage, err := p.orch.BootstrapFramework(ctx, req.ReasoningModel, FrameworkBootstrapRequest{
		Title:          req.Title,
		TargetPlatform: req.TargetPlatform,
		CoreConcept:    req.Concept,
	})
	if err != nil {
		return nil, fmt.Errorf("pipeline bootstrap framework: %w", err)
	}
	addTokenUsage(&result.TotalUsage, fwUsage)

	// 3. projectID := fmt.Sprintf("proj_%d", time.Now().UnixNano())；proj 构造；SaveProject；SeedGenesisArtifacts；emit FRAMEWORK_READY
	projectID := fmt.Sprintf("proj_%d", time.Now().UnixNano())
	result.ProjectID = projectID

	platform := req.TargetPlatform
	if strings.TrimSpace(platform) == "" {
		platform = "通用网文"
	}

	proj := &domain.Project{
		ID:             projectID,
		Title:          req.Title,
		TargetPlatform: platform,
		WorldRules:     BuildWorldRules(fw),
		Framework:      fw,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := p.store.SaveProject(ctx, proj); err != nil {
		return nil, fmt.Errorf("pipeline save project: %w", err)
	}

	if err := SeedGenesisArtifacts(ctx, p.store, proj, fw); err != nil {
		return nil, fmt.Errorf("pipeline seed genesis artifacts: %w", err)
	}

	emit(PipelineEvent{
		Phase:   "FRAMEWORK_READY",
		Message: fmt.Sprintf("创世总纲已生成并入库，共规划 %d 卷", len(fw.VolumeArcs)),
	})

	// 4. 关系图谱抽取（best-effort）
	entries, err := p.store.ListCodexEntries(ctx, projectID, "")
	if err != nil || len(entries) < 2 {
		var note string
		if err != nil {
			note = fmt.Sprintf("读取实体档案失败 (%v)，跳过关系图谱抽取", err)
		} else {
			note = fmt.Sprintf("实体档案不足 2 条 (当前 %d 条)，跳过关系图谱抽取", len(entries))
		}
		result.Notes = append(result.Notes, note)
		emit(PipelineEvent{
			Phase:   "RELATIONS_DONE",
			Message: note,
		})
	} else {
		recentContext := proj.WorldRules + "\n\n全书核心立意：" + fw.ThemePremise
		relations, relUsage, relErr := p.orch.ExtractCodexRelations(ctx, req.ReasoningModel, proj, entries, recentContext)
		addTokenUsage(&result.TotalUsage, relUsage)
		if relErr != nil {
			warning := fmt.Sprintf("关系图谱推演失败: %v", relErr)
			result.Notes = append(result.Notes, warning)
			emit(PipelineEvent{
				Phase:   "RELATIONS_DONE",
				Message: warning,
			})
		} else {
			for i := range relations {
				_ = p.store.SaveCodexRelation(ctx, projectID, &relations[i])
			}
			emit(PipelineEvent{
				Phase:   "RELATIONS_DONE",
				Message: fmt.Sprintf("关系图谱推演完成，写入 %d 条关系", len(relations)),
			})
		}
	}

	// 5. targets 卷 = BackfillVolumeArcs(fw.VolumeArcs) 按 VolumeIndex 升序；Volumes>0 取前 N；Volumes>len(arcs) → 校验错
	arcs := BackfillVolumeArcs(fw.VolumeArcs)
	sort.Slice(arcs, func(i, j int) bool {
		return arcs[i].VolumeIndex < arcs[j].VolumeIndex
	})
	if req.Volumes > len(arcs) {
		return nil, fmt.Errorf("pipeline: requested volumes (%d) exceeds available volumes (%d)", req.Volumes, len(arcs))
	}
	targets := arcs
	if req.Volumes > 0 {
		targets = arcs[:req.Volumes]
	}

	// 6. 逐卷
	wordsTarget := req.WordsTarget
	if wordsTarget <= 0 {
		wordsTarget = 2000
	}
	maxRewriteLoops := req.MaxRewriteLoops
	if maxRewriteLoops <= 0 {
		maxRewriteLoops = 3
	}
	runID := req.RunID
	if runID == "" {
		runID = fmt.Sprintf("run_%d", time.Now().UnixNano())
	}

	for _, arc := range targets {
		// a. 意图补齐
		existingIntents, err := p.store.ListChapterIntents(ctx, projectID, arc.VolumeIndex)
		if err != nil {
			return nil, fmt.Errorf("list chapter intents for volume %d: %w", arc.VolumeIndex, err)
		}
		existingMap := make(map[int]bool)
		for _, it := range existingIntents {
			if it != nil {
				existingMap[it.ChapterIndex] = true
			}
		}

		newIntents, intUsage, err := p.orch.GenerateVolumeIntents(ctx, req.ReasoningModel, proj, arc, existingMap)
		if err != nil {
			return nil, fmt.Errorf("generate volume intents for volume %d: %w", arc.VolumeIndex, err)
		}
		addTokenUsage(&result.TotalUsage, intUsage)

		for i := range newIntents {
			intent := newIntents[i]
			if err := p.store.SaveChapterIntent(ctx, &intent); err != nil {
				return nil, fmt.Errorf("save generated chapter intent ch%d: %w", intent.ChapterIndex, err)
			}
		}
		emit(PipelineEvent{
			Phase:       "INTENTS_READY",
			VolumeIndex: arc.VolumeIndex,
			Message:     fmt.Sprintf("第 %d 卷意图卡已就绪 (新生成 %d 条)", arc.VolumeIndex, len(newIntents)),
		})

		// b. AutoLockIntents 判定与锁定
		if req.AutoLockIntents {
			allIntents, err := p.store.ListChapterIntents(ctx, projectID, arc.VolumeIndex)
			if err != nil {
				return nil, fmt.Errorf("list intents for locking volume %d: %w", arc.VolumeIndex, err)
			}
			for _, it := range allIntents {
				if it != nil && it.Status == domain.IntentStatusDraft {
					if err := it.TransitionTo(domain.IntentStatusLocked); err != nil {
						return nil, fmt.Errorf("lock intent ch%d: %w", it.ChapterIndex, err)
					}
					if err := p.store.SaveChapterIntent(ctx, it); err != nil {
						return nil, fmt.Errorf("save locked intent ch%d: %w", it.ChapterIndex, err)
					}
				}
			}
			emit(PipelineEvent{
				Phase:       "INTENTS_LOCKED",
				VolumeIndex: arc.VolumeIndex,
				Message:     fmt.Sprintf("第 %d 卷意图卡已锁定", arc.VolumeIndex),
			})
		} else {
			allIntents, err := p.store.ListChapterIntents(ctx, projectID, arc.VolumeIndex)
			if err != nil {
				return nil, fmt.Errorf("list intents for check volume %d: %w", arc.VolumeIndex, err)
			}
			lockedMap := make(map[int]bool)
			for _, it := range allIntents {
				if it != nil && it.Status == domain.IntentStatusLocked {
					lockedMap[it.ChapterIndex] = true
				}
			}

			prodStart := arc.StartChapter
			if req.ChapterFrom > prodStart {
				prodStart = req.ChapterFrom
			}
			prodEnd := arc.EndChapter

			missingChapter := 0
			for ch := prodStart; ch <= prodEnd; ch++ {
				if !lockedMap[ch] {
					missingChapter = ch
					break
				}
			}

			if missingChapter > 0 {
				result.Halted = true
				result.HaltReason = fmt.Sprintf("意图卡未锁定（第 %d 章…），已停线待人工锁定", missingChapter)
				emit(PipelineEvent{
					Phase:        "HALTED",
					VolumeIndex:  arc.VolumeIndex,
					ChapterIndex: missingChapter,
					Message:      result.HaltReason,
					NeedsHuman:   true,
				})
				result.EstimatedCostUSD = EstimateCostUSD(result.TotalUsage)
				return result, nil
			}
		}

		// c. 卷生产
		volReq := VolumeProduceRequest{
			ProjectID:       projectID,
			VolumeIndex:     arc.VolumeIndex,
			ChapterFrom:     req.ChapterFrom,
			WordsTarget:     wordsTarget,
			MaxRewriteLoops: maxRewriteLoops,
			NarrativeStyle:  req.NarrativeStyle,
			RunID:           runID,
			OnProgress: func(ve VolumeEvent) {
				emit(PipelineEvent{
					Phase:        ve.Phase,
					VolumeIndex:  arc.VolumeIndex,
					ChapterIndex: ve.ChapterIndex,
					Message:      ve.Message,
					NeedsHuman:   ve.NeedsHuman,
				})
			},
		}

		volRes, err := p.producer.ProduceVolume(ctx, volReq)
		if err != nil {
			return nil, err
		}
		if volRes != nil {
			result.Volumes = append(result.Volumes, volRes)
			addTokenUsage(&result.TotalUsage, volRes.TotalUsage)
			if volRes.Stopped {
				result.Halted = true
				result.HaltReason = volRes.StopReason
				emit(PipelineEvent{
					Phase:        "HALTED",
					VolumeIndex:  arc.VolumeIndex,
					ChapterIndex: volRes.StoppedAtChapter,
					Message:      volRes.StopReason,
					NeedsHuman:   true,
				})
				result.EstimatedCostUSD = EstimateCostUSD(result.TotalUsage)
				return result, nil
			}
		}
		emit(PipelineEvent{
			Phase:       "VOLUME_DONE",
			VolumeIndex: arc.VolumeIndex,
			Message:     fmt.Sprintf("第 %d 卷生产完成", arc.VolumeIndex),
		})
	}

	// 7. 汇总
	result.EstimatedCostUSD = EstimateCostUSD(result.TotalUsage)
	emit(PipelineEvent{
		Phase:   "PIPELINE_COMPLETE",
		Message: fmt.Sprintf("流水线全部完成，共生产 %d 卷", len(result.Volumes)),
	})
	return result, nil
}
