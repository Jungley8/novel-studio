package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/store"
)

type abSideConfig struct {
	ReasoningModel  string `json:"reasoning_model,omitempty"`
	WriterModel     string `json:"writer_model,omitempty"`
	ReviewerModel   string `json:"reviewer_model,omitempty"`
	WordsTarget     int    `json:"words_target,omitempty"`
	MaxRewriteLoops int    `json:"max_rewrite_loops,omitempty"`
}

type abRunSide struct {
	Label                  string  `json:"label"`
	Committed              bool    `json:"committed"`
	Outcome                string  `json:"outcome"`
	Verdict                string  `json:"verdict"`
	Score                  int     `json:"score"`
	ActualWords            int     `json:"actual_words"`
	WordQuotaRatio         float64 `json:"word_quota_ratio"`
	RewriteLoops           int     `json:"rewrite_loops"`
	UnresolvedObservations int     `json:"unresolved_observations"`
	HookClaimsValidated    int     `json:"hook_claims_validated"`
	HookClaimsTotal        int     `json:"hook_claims_total"`
	CompactionFailed       bool    `json:"compaction_failed"`
	Tokens                 int     `json:"tokens"`
	CostUSD                float64 `json:"cost_usd"`
	Err                    string  `json:"err,omitempty"`
}

type abComparisonRow struct {
	Dimension string
	SideA     string
	SideB     string
	Diff      string
}

// resolveABSideConfig 解析并校验 A/B 侧配置（Fail-Closed 纯函数）。
func resolveABSideConfig(cfg *config.Config, rawA, rawB abSideConfig) (abSideConfig, abSideConfig, error) {
	sideA := rawA
	sideB := rawB

	// A 侧默认值取 cfg
	if cfg != nil {
		if strings.TrimSpace(sideA.WriterModel) == "" {
			sideA.WriterModel = cfg.WriterModel
		}
		if strings.TrimSpace(sideA.ReasoningModel) == "" {
			sideA.ReasoningModel = cfg.ReasoningModel
		}
		if strings.TrimSpace(sideA.ReviewerModel) == "" {
			sideA.ReviewerModel = cfg.ReviewerModel
		}
	}
	if sideA.WordsTarget <= 0 {
		sideA.WordsTarget = 2000
	}
	if sideA.MaxRewriteLoops <= 0 {
		sideA.MaxRewriteLoops = 3
	}

	// B 侧校验与默认值
	if strings.TrimSpace(sideB.WriterModel) == "" {
		return sideA, sideB, errors.New("-writer-b 为空（B 侧必须显式给出 writer 模型）")
	}
	if cfg != nil {
		if strings.TrimSpace(sideB.ReasoningModel) == "" {
			sideB.ReasoningModel = cfg.ReasoningModel
		}
		if strings.TrimSpace(sideB.ReviewerModel) == "" {
			sideB.ReviewerModel = cfg.ReviewerModel
		}
	}
	if sideB.WordsTarget <= 0 {
		sideB.WordsTarget = 2000
	}
	if sideB.MaxRewriteLoops <= 0 {
		sideB.MaxRewriteLoops = 3
	}

	// A/B 五元组相同性校验（Fail-Closed）
	if sideA == sideB {
		return sideA, sideB, errors.New("A/B 配置完全相同，拒绝空转（至少一个 B 侧参数须不同）")
	}

	return sideA, sideB, nil
}

func formatIntDiff(diff int) string {
	if diff > 0 {
		return fmt.Sprintf("+%d", diff)
	}
	return fmt.Sprintf("%d", diff)
}

func formatPercentDiff(diff float64) string {
	if diff > 0 {
		return fmt.Sprintf("+%.1f%%", diff)
	}
	return fmt.Sprintf("%.1f%%", diff)
}

func formatCostDiff(diff float64) string {
	if diff > 0 {
		return fmt.Sprintf("+$%.4f", diff)
	} else if diff < 0 {
		return fmt.Sprintf("-$%.4f", -diff)
	}
	return "$0.0000"
}

// compareAB 计算并排对比维度列表（纯函数）。
func compareAB(sideA, sideB abRunSide) []abComparisonRow {
	var rows []abComparisonRow

	rows = append(rows, abComparisonRow{
		Dimension: "终态 (Outcome)",
		SideA:     sideA.Outcome,
		SideB:     sideB.Outcome,
		Diff:      "-",
	})
	rows = append(rows, abComparisonRow{
		Dimension: "终审裁决 (Verdict)",
		SideA:     sideA.Verdict,
		SideB:     sideB.Verdict,
		Diff:      "-",
	})
	rows = append(rows, abComparisonRow{
		Dimension: "是否入库 (Committed)",
		SideA:     fmt.Sprintf("%t", sideA.Committed),
		SideB:     fmt.Sprintf("%t", sideB.Committed),
		Diff:      "-",
	})
	rows = append(rows, abComparisonRow{
		Dimension: "综合评分 (Score)",
		SideA:     fmt.Sprintf("%d", sideA.Score),
		SideB:     fmt.Sprintf("%d", sideB.Score),
		Diff:      formatIntDiff(sideB.Score - sideA.Score),
	})
	rows = append(rows, abComparisonRow{
		Dimension: "实际字数 (ActualWords)",
		SideA:     fmt.Sprintf("%d", sideA.ActualWords),
		SideB:     fmt.Sprintf("%d", sideB.ActualWords),
		Diff:      formatIntDiff(sideB.ActualWords - sideA.ActualWords),
	})
	rows = append(rows, abComparisonRow{
		Dimension: "字数达成率 (WordQuotaRatio)",
		SideA:     fmt.Sprintf("%.1f%%", sideA.WordQuotaRatio*100),
		SideB:     fmt.Sprintf("%.1f%%", sideB.WordQuotaRatio*100),
		Diff:      formatPercentDiff((sideB.WordQuotaRatio - sideA.WordQuotaRatio) * 100),
	})
	rows = append(rows, abComparisonRow{
		Dimension: "返工轮次 (RewriteLoops)",
		SideA:     fmt.Sprintf("%d", sideA.RewriteLoops),
		SideB:     fmt.Sprintf("%d", sideB.RewriteLoops),
		Diff:      formatIntDiff(sideB.RewriteLoops - sideA.RewriteLoops),
	})
	rows = append(rows, abComparisonRow{
		Dimension: "未决观测 (UnresolvedObs)",
		SideA:     fmt.Sprintf("%d", sideA.UnresolvedObservations),
		SideB:     fmt.Sprintf("%d", sideB.UnresolvedObservations),
		Diff:      formatIntDiff(sideB.UnresolvedObservations - sideA.UnresolvedObservations),
	})
	rows = append(rows, abComparisonRow{
		Dimension: "伏笔验证通过 (HookValidated)",
		SideA:     fmt.Sprintf("%d", sideA.HookClaimsValidated),
		SideB:     fmt.Sprintf("%d", sideB.HookClaimsValidated),
		Diff:      formatIntDiff(sideB.HookClaimsValidated - sideA.HookClaimsValidated),
	})
	rows = append(rows, abComparisonRow{
		Dimension: "伏笔声明总数 (HookTotal)",
		SideA:     fmt.Sprintf("%d", sideA.HookClaimsTotal),
		SideB:     fmt.Sprintf("%d", sideB.HookClaimsTotal),
		Diff:      formatIntDiff(sideB.HookClaimsTotal - sideA.HookClaimsTotal),
	})
	rows = append(rows, abComparisonRow{
		Dimension: "状态压缩失败 (CompactionFailed)",
		SideA:     fmt.Sprintf("%t", sideA.CompactionFailed),
		SideB:     fmt.Sprintf("%t", sideB.CompactionFailed),
		Diff:      "-",
	})
	rows = append(rows, abComparisonRow{
		Dimension: "Token 消耗 (Tokens)",
		SideA:     fmt.Sprintf("%d", sideA.Tokens),
		SideB:     fmt.Sprintf("%d", sideB.Tokens),
		Diff:      formatIntDiff(sideB.Tokens - sideA.Tokens),
	})
	rows = append(rows, abComparisonRow{
		Dimension: "预估成本 (CostUSD)",
		SideA:     fmt.Sprintf("$%.4f", sideA.CostUSD),
		SideB:     fmt.Sprintf("$%.4f", sideB.CostUSD),
		Diff:      formatCostDiff(sideB.CostUSD - sideA.CostUSD),
	})
	if sideA.Err != "" || sideB.Err != "" {
		rows = append(rows, abComparisonRow{
			Dimension: "异常信息 (Error)",
			SideA:     sideA.Err,
			SideB:     sideB.Err,
			Diff:      "-",
		})
	}
	return rows
}

// formatABComparison 格式化 A/B 对比结果为并排表格或 JSON（纯函数）。
func formatABComparison(sideA, sideB abRunSide, asJSON bool) string {
	if asJSON {
		report := map[string]abRunSide{
			"a": sideA,
			"b": sideB,
		}
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "{}"
		}
		return string(data)
	}

	rows := compareAB(sideA, sideB)
	var sb strings.Builder
	sb.WriteString("================================================================================\n")
	sb.WriteString("  NovelStudio A/B 双跑对比评测报告\n")
	sb.WriteString(fmt.Sprintf("  侧 A: %s | 侧 B: %s\n", sideA.Label, sideB.Label))
	sb.WriteString("================================================================================\n")
	sb.WriteString(fmt.Sprintf("  %-32s | %-16s | %-16s | %-14s\n", "维度", sideA.Label, sideB.Label, "差值 (B - A)"))
	sb.WriteString("--------------------------------------------------------------------------------\n")
	for _, r := range rows {
		sb.WriteString(fmt.Sprintf("  %-32s | %-16s | %-16s | %-14s\n", r.Dimension, r.SideA, r.SideB, r.Diff))
	}
	sb.WriteString("================================================================================\n")
	sb.WriteString(fmt.Sprintf("  ● 终态标注: A=[%s] vs B=[%s]\n", sideA.Outcome, sideB.Outcome))
	sb.WriteString("================================================================================\n")
	return sb.String()
}

func formatAB(sideA, sideB abRunSide, asJSON bool) string {
	return formatABComparison(sideA, sideB, asJSON)
}

// collectRunSide 整合生产产出与落库指标数据（纯函数）。
func collectRunSide(
	label string,
	chapterIdx int,
	res *engine.WorkshopProduceResult,
	produceErr error,
	metricsList []*domain.ChapterQualityMetrics,
) abRunSide {
	runSide := abRunSide{
		Label: label,
	}

	if produceErr != nil {
		runSide.Err = produceErr.Error()
	}

	if res != nil {
		runSide.Committed = res.Committed
		runSide.Tokens = res.TotalUsage.TotalTokens
		runSide.CostUSD = res.EstimatedCostUSD
	}

	// 优先从 ListChapterMetrics 对应行读
	var matchedMetric *domain.ChapterQualityMetrics
	for _, m := range metricsList {
		if m != nil && m.ChapterIndex == chapterIdx {
			matchedMetric = m
		}
	}

	if matchedMetric != nil {
		runSide.Outcome = matchedMetric.Outcome
		runSide.Verdict = matchedMetric.Verdict
		runSide.Score = matchedMetric.Score
		runSide.ActualWords = matchedMetric.ActualWords
		runSide.WordQuotaRatio = matchedMetric.WordQuotaRatio
		runSide.RewriteLoops = matchedMetric.RewriteLoops
		runSide.UnresolvedObservations = matchedMetric.UnresolvedObservations
		runSide.HookClaimsValidated = matchedMetric.HookClaimsValidated
		runSide.HookClaimsTotal = matchedMetric.HookClaimsTotal
		runSide.CompactionFailed = matchedMetric.CompactionFailed
		if matchedMetric.Outcome == domain.ChapterOutcomeCommitted {
			runSide.Committed = true
		}
	} else {
		// 行缺失时回退用 res 字段（注意：本次 commit 章必落库；行缺失按失败记账）
		if res != nil {
			if res.Audit != nil {
				runSide.Verdict = string(res.Audit.Verdict)
				runSide.Score = res.Audit.Score
				runSide.HookClaimsTotal = len(res.Audit.ResolvedHookClaims)
				runSide.HookClaimsValidated = len(res.Audit.ResolvedHookIDs)
				if res.Audit.NeedsHuman {
					runSide.Outcome = domain.ChapterOutcomeNeedsHuman
				}
			}
			if res.TechnicalBlocked {
				runSide.Outcome = domain.ChapterOutcomeTechnicalBlocked
			}
			if runSide.Outcome == "" {
				if res.Committed {
					runSide.Outcome = domain.ChapterOutcomeCommitted
				} else {
					runSide.Outcome = "unknown"
				}
			}
			runSide.ActualWords = res.ActualWords
			runSide.WordQuotaRatio = res.WordQuotaRatio
			runSide.RewriteLoops = res.RewriteLoops
			runSide.UnresolvedObservations = len(res.StateValidationObservations)
		}
		if runSide.Outcome == "" {
			runSide.Outcome = "failed"
		}
		if runSide.Err == "" && produceErr == nil {
			runSide.Err = "质量指标行缺失（ListChapterMetrics 未找到记录）"
		}
	}

	return runSide
}

func runOneSide(
	ctx context.Context,
	label string,
	prefix string,
	dbPath string,
	cfg *config.Config,
	llmClient *engine.HTTPLLMClient,
	projectID string,
	chapterIdx int,
	conflict string,
	sideCfg abSideConfig,
	isJSON bool,
) (abRunSide, error) {
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		return abRunSide{
			Label: label,
			Err:   fmt.Sprintf("打开 SQLite 失败: %v", err),
		}, err
	}
	defer s.Close()

	router := engine.NewLLMRouter(llmClient)
	router.UpdateFromConfig(cfg)
	orch := engine.NewOrchestrator(router)
	chronicle := engine.NewCanonChronicle(s)
	chronicle.SetConfig(cfg)
	qualityGate := engine.NewQualityGate(router, nil)
	workshop := engine.NewChapterWorkshop(orch, chronicle, qualityGate, s)

	produceReq := engine.WorkshopProduceRequest{
		ProjectID:        projectID,
		ChapterIndex:     chapterIdx,
		CoreConflict:     conflict,
		ReasoningModel:   sideCfg.ReasoningModel,
		WriterModel:      sideCfg.WriterModel,
		ReviewerModel:    sideCfg.ReviewerModel,
		WordsTarget:      sideCfg.WordsTarget,
		AutoCommit:       true,
		MaxRewriteLoops:  sideCfg.MaxRewriteLoops,
		ResumeCheckpoint: false,
		OnProgress: func(ev engine.WorkshopEvent) {
			if isJSON {
				fmt.Fprintf(os.Stderr, "  [%s] [%s] %s\n", prefix, ev.Phase, ev.Message)
			} else {
				fmt.Printf("  [%s] [%s] %s\n", prefix, ev.Phase, ev.Message)
			}
		},
	}

	res, produceErr := workshop.ProduceChapter(ctx, produceReq)

	metricsList, mErr := s.ListChapterMetrics(ctx, projectID)
	if mErr != nil {
		metricsList = nil
	}

	runSide := collectRunSide(label, chapterIdx, res, produceErr, metricsList)

	if produceErr != nil {
		return runSide, produceErr
	}
	if runSide.Err != "" {
		return runSide, errors.New(runSide.Err)
	}
	return runSide, nil
}

// runABTestCLI 运行 novel-studio abtest 命令行指令。
func runABTestCLI(argv []string, defaultDataDir string) int {
	fs := flag.NewFlagSet("abtest", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "NovelStudio abtest: A/B 双跑对比评测（快照隔离，只读真实库）。\n\n")
		fmt.Fprintf(fs.Output(), "Usage of abtest:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\n退出码:\n  0: 两侧均成功完成生产\n  1: 任一基础设施或生产过程失败\n")
	}

	dataDirFlag := fs.String("data-dir", defaultDataDir, "数据持久化目录")
	projectIDFlag := fs.String("project", "", "作品 ID (默认选择首个已有作品)")
	chapterIdxFlag := fs.Int("chapter", 0, "章节序号 (≤0 自动下一章；若已存在则报错拒绝)")
	conflictFlag := fs.String("conflict", "核心对峙与打破常规预期", "本章核心冲突事实")
	jsonFlag := fs.Bool("json", false, "输出机器可读 JSON 格式对比报告")

	writerA := fs.String("writer-a", "", "A 侧生成/渲染模型 (默认取配置)")
	reasoningA := fs.String("reasoning-a", "", "A 侧推理/思考模型 (默认取配置)")
	reviewerA := fs.String("reviewer-a", "", "A 侧审校模型 (默认取配置)")
	wordsA := fs.Int("words-a", 0, "A 侧目标字数 (默认 2000)")
	loopsA := fs.Int("loops-a", 0, "A 侧最大返工循环轮次 (默认 3)")

	writerB := fs.String("writer-b", "", "B 侧生成/渲染模型 (必填)")
	reasoningB := fs.String("reasoning-b", "", "B 侧推理/思考模型 (默认取配置)")
	reviewerB := fs.String("reviewer-b", "", "B 侧审校模型 (默认取配置)")
	wordsB := fs.Int("words-b", 0, "B 侧目标字数 (默认 2000)")
	loopsB := fs.Int("loops-b", 0, "B 侧最大返工循环轮次 (默认 3)")

	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	dataDir := *dataDirFlag
	cfgPath := filepath.Join(dataDir, "config.json")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 加载配置失败: %v\n", err)
		return 1
	}

	rawA := abSideConfig{
		ReasoningModel:  *reasoningA,
		WriterModel:     *writerA,
		ReviewerModel:   *reviewerA,
		WordsTarget:     *wordsA,
		MaxRewriteLoops: *loopsA,
	}
	rawB := abSideConfig{
		ReasoningModel:  *reasoningB,
		WriterModel:     *writerB,
		ReviewerModel:   *reviewerB,
		WordsTarget:     *wordsB,
		MaxRewriteLoops: *loopsB,
	}

	sideA, sideB, err := resolveABSideConfig(cfg, rawA, rawB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ A/B 配置校验失败: %v\n", err)
		return 1
	}

	realDBPath := filepath.Join(dataDir, "novel.db")
	snapPath, snapCleanup, err := openBenchSnapshot(realDBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 创建数据库快照失败: %v\n", err)
		return 1
	}
	defer snapCleanup()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx = activity.WithSource(ctx, activity.SourceCLIProduce)

	snapStore, err := store.NewSQLiteStore(snapPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 打开快照数据库失败: %v\n", err)
		return 1
	}

	projectID := *projectIDFlag
	if projectID == "" {
		projects, err := snapStore.ListProjects(ctx)
		if err != nil || len(projects) == 0 {
			_ = snapStore.Close()
			fmt.Fprintf(os.Stderr, "✗ 未找到任何作品，请先创建作品\n")
			return 1
		}
		projectID = projects[0].ID
	}

	project, err := snapStore.GetProject(ctx, projectID)
	if err != nil || project == nil {
		_ = snapStore.Close()
		fmt.Fprintf(os.Stderr, "✗ 获取作品 [%s] 失败: %v\n", projectID, err)
		return 1
	}

	chapters, err := snapStore.ListChapters(ctx, projectID)
	if err != nil {
		_ = snapStore.Close()
		fmt.Fprintf(os.Stderr, "✗ 获取作品章节失败: %v\n", err)
		return 1
	}

	chapterIdx := *chapterIdxFlag
	if chapterIdx <= 0 {
		chapterIdx = len(chapters) + 1
	} else {
		for _, ch := range chapters {
			if ch.ChapterIndex == chapterIdx {
				_ = snapStore.Close()
				fmt.Fprintf(os.Stderr, "✗ 章节 %d 已存在，拒绝重复生产（A/B 对比要求新章节）\n", chapterIdx)
				return 1
			}
		}
	}

	if err := snapStore.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "✗ 关闭快照数据库失败: %v\n", err)
		return 1
	}

	snapData, err := os.ReadFile(snapPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 读取快照数据失败: %v\n", err)
		return 1
	}

	tmpDirA, err := os.MkdirTemp("", "novel-abtest-a-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 创建 A 侧临时目录失败: %v\n", err)
		return 1
	}
	defer os.RemoveAll(tmpDirA)
	sideAPath := filepath.Join(tmpDirA, "novel.db")
	if err := os.WriteFile(sideAPath, snapData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "✗ 写入 A 侧快照失败: %v\n", err)
		return 1
	}

	tmpDirB, err := os.MkdirTemp("", "novel-abtest-b-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 创建 B 侧临时目录失败: %v\n", err)
		return 1
	}
	defer os.RemoveAll(tmpDirB)
	sideBPath := filepath.Join(tmpDirB, "novel.db")
	if err := os.WriteFile(sideBPath, snapData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "✗ 写入 B 侧快照失败: %v\n", err)
		return 1
	}

	noticeMsg := "将执行两次完整生产（含重写循环），约消耗 2× 单章成本；全程在临时快照上，真实库零写入。"
	if *jsonFlag {
		fmt.Fprintln(os.Stderr, noticeMsg)
	} else {
		fmt.Println("==================================================================")
		fmt.Printf("  NovelStudio A/B 双跑对比评测引擎 (快照隔离)\n")
		fmt.Printf("  • 作品: 《%s》 (ID: %s)\n", project.Title, projectID)
		fmt.Printf("  • 目标章节: 第 %d 章\n", chapterIdx)
		fmt.Printf("  • 核心冲突: %s\n", *conflictFlag)
		fmt.Printf("  • A 侧模型: 推理=%s | 渲染=%s | 审校=%s (目标字数: %d, 最大返工: %d)\n", sideA.ReasoningModel, sideA.WriterModel, sideA.ReviewerModel, sideA.WordsTarget, sideA.MaxRewriteLoops)
		fmt.Printf("  • B 侧模型: 推理=%s | 渲染=%s | 审校=%s (目标字数: %d, 最大返工: %d)\n", sideB.ReasoningModel, sideB.WriterModel, sideB.ReviewerModel, sideB.WordsTarget, sideB.MaxRewriteLoops)
		fmt.Printf("  ※ 提示: %s\n", noticeMsg)
		fmt.Println("==================================================================")
	}

	llmClient := engine.NewHTTPLLMClient(cfg.APIBase, cfg.APIKey)

	runA, errA := runOneSide(ctx, "A", "A", sideAPath, cfg, llmClient, projectID, chapterIdx, *conflictFlag, sideA, *jsonFlag)
	runB, errB := runOneSide(ctx, "B", "B", sideBPath, cfg, llmClient, projectID, chapterIdx, *conflictFlag, sideB, *jsonFlag)

	output := formatABComparison(runA, runB, *jsonFlag)
	fmt.Println(output)

	if errA != nil || errB != nil || runA.Err != "" || runB.Err != "" {
		return 1
	}
	return 0
}
