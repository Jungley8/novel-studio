package main

// 退出码契约:
//   0 = 已归档入库 (或未开启自动归档时正常完成)
//   1 = 执行失败
//   2 = 停线待人工 (触发质量门禁/拒稿停线)

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/coord"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/store"
)

func runProduceCLI(argv []string, defaultDataDir string) int {
	fs := flag.NewFlagSet("produce", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage of produce:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\n退出码:\n  0: 已归档入库 (或非自动归档正常完成)\n  1: 执行失败\n  2: 停线待人工\n")
	}
	dataDirFlag := fs.String("data-dir", defaultDataDir, "数据持久化目录")
	projectIDFlag := fs.String("project", "", "作品 ID (默认选择首个已有作品)")
	chapterIdxFlag := fs.Int("chapter", 0, "章节序号 (默认自动递增下一章)")
	conflictFlag := fs.String("conflict", "核心对峙与打破常规预期", "本章核心冲突事实")
	autoCommitFlag := fs.Bool("commit", true, "通过主编终审后自动提交归档至数据库")
	volumeIdxFlag := fs.Int("volume", 0, "分卷序号 (>0 时执行整卷批量生产)")
	_ = fs.Parse(argv)

	dataDir := *dataDirFlag
	cfgPath := filepath.Join(dataDir, "config.json")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 加载配置失败: %v\n", err)
		return 1
	}

	dbPath := filepath.Join(dataDir, "novel.db")
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 打开 SQLite 数据库失败: %v\n", err)
		return 1
	}
	defer s.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx = activity.WithSource(ctx, activity.SourceCLIProduce)

	// Resolve Project ID
	projectID := *projectIDFlag
	if projectID == "" {
		projects, err := s.ListProjects(ctx)
		if err != nil || len(projects) == 0 {
			fmt.Fprintf(os.Stderr, "✗ 未找到任何作品，请先在看板或使用 novel-studio 启动创建作品。\n")
			return 1
		}
		projectID = projects[0].ID
	}

	project, err := s.GetProject(ctx, projectID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 获取作品 [%s] 失败: %v\n", projectID, err)
		return 1
	}

	// Fail-Closed：跨进程生产锁。守护服务（浏览器编辑租约 / 排程生产）或其它 CLI
	// 正在写同一作品时拒绝并发生产；进程崩溃由租约 TTL 自动清理。
	production, blocking, err := coord.AcquireProduction(ctx, s, projectID, "CLI 无头生产", 0)
	if err != nil {
		if errors.Is(err, coord.ErrBusy) && blocking != nil {
			if blocking.Kind == coord.KindEdit {
				fmt.Fprintf(os.Stderr, "✗ 作品 [%s] 正在被人工编辑（编辑租约未过期），已拒绝并发生产。\n", projectID)
			} else {
				fmt.Fprintf(os.Stderr, "✗ 作品 [%s] 正在生产（%s·第 %d 章），已拒绝并发生产。\n", projectID, blocking.Label, blocking.Chapter)
			}
		} else {
			fmt.Fprintf(os.Stderr, "✗ 获取生产锁失败: %v\n", err)
		}
		return 1
	}
	defer production.Release()

	if *volumeIdxFlag > 0 {
		llmClient := engine.NewHTTPLLMClient(cfg.APIBase, cfg.APIKey)
		router := engine.NewLLMRouter(llmClient)
		router.UpdateFromConfig(cfg)

		orch := engine.NewOrchestrator(router)
		chronicle := engine.NewCanonChronicle(s)
		chronicle.SetConfig(cfg)
		qualityGate := engine.NewQualityGate(router, nil)
		workshop := engine.NewChapterWorkshop(orch, chronicle, qualityGate, s)

		batch := engine.NewVolumeBatch(workshop, s)
		req := engine.VolumeProduceRequest{
			ProjectID:       projectID,
			VolumeIndex:     *volumeIdxFlag,
			ChapterFrom:     *chapterIdxFlag,
			WordsTarget:     2000,
			MaxRewriteLoops: 3,
			OnProgress: func(ev engine.VolumeEvent) {
				production.SetChapter(ev.ChapterIndex)
				if ev.Phase == "CHAPTER_DONE" {
					fmt.Printf("第 %d 章 → %s/%d/%v\n", ev.ChapterIndex, ev.Verdict, ev.Score, ev.Committed)
				}
			},
		}

		res, err := batch.ProduceVolume(ctx, req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ 卷批产执行失败: %v\n", err)
			return 1
		}

		return printVolumeProduceSummary(os.Stdout, os.Stderr, res)
	}

	// Resolve Chapter Index
	chapterIdx := *chapterIdxFlag
	if chapterIdx <= 0 {
		chapters, _ := s.ListChapters(ctx, projectID)
		chapterIdx = len(chapters) + 1
	}
	production.SetChapter(chapterIdx)

	fmt.Println("==================================================================")
	fmt.Printf("  NovelStudio Autonomous Pipeline (自主创作生产引擎)\n")
	fmt.Printf("  • 作品: 《%s》 (%s)\n", project.Title, project.TargetPlatform)
	fmt.Printf("  • 目标章节: 第 %d 章\n", chapterIdx)
	fmt.Printf("  • 核心冲突: %s\n", *conflictFlag)
	fmt.Printf("  • 推理模型: %s | 渲染模型: %s | 独立审校: %s\n", cfg.ReasoningModel, cfg.WriterModel, cfg.ReviewerModel)
	fmt.Println("==================================================================")

	llmClient := engine.NewHTTPLLMClient(cfg.APIBase, cfg.APIKey)
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
		CoreConflict:     *conflictFlag,
		ReasoningModel:   cfg.ReasoningModel,
		WriterModel:      cfg.WriterModel,
		ReviewerModel:    cfg.ReviewerModel,
		WordsTarget:      2000,
		AutoCommit:       *autoCommitFlag,
		MaxRewriteLoops:  3,
		ResumeCheckpoint: true,
		OnProgress: func(ev engine.WorkshopEvent) {
			fmt.Printf("  [%s] %s\n", ev.Phase, ev.Message)
		},
	}

	res, err := workshop.ProduceChapter(ctx, produceReq)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 自主生产流水线执行失败: %v\n", err)
		return 1
	}

	return printProduceSummary(os.Stdout, os.Stderr, res)
}

func printProduceSummary(stdout, stderr io.Writer, res *engine.WorkshopProduceResult) int {
	if res == nil {
		return 1
	}

	// 停线待人工: 退出码 2
	if res.Audit != nil && res.Audit.NeedsHuman {
		fmt.Fprintf(stderr, "✗ 终审裁决: %s (综合评分: %d/100)\n", res.Audit.Verdict, res.Audit.Score)
		fmt.Fprintf(stderr, "✗ 返工轮次: %d 次\n", res.RewriteLoops)
		for i := 0; i < len(res.Audit.Issues) && i < 3; i++ {
			fmt.Fprintf(stderr, "✗ 审校问题 %d: %s\n", i+1, res.Audit.Issues[i])
		}
		return 2
	}

	fmt.Fprintln(stdout, "\n✓ 生产完成！")
	if res.ResumedPhase != "" {
		fmt.Fprintf(stdout, "  • 断点恢复: 从 [%s] 阶段无损续跑\n", res.ResumedPhase)
	}
	if res.Audit != nil {
		fmt.Fprintf(stdout, "  • 终审裁决: %s (综合评分: %d/100, 突发度: %d)\n",
			res.Audit.Verdict, res.Audit.Score, res.Audit.BurstinessScore)
	}
	fmt.Fprintf(stdout, "  • 定向返工轮次: %d 次\n", res.RewriteLoops)
	fmt.Fprintf(stdout, "  • Token消耗: %d tokens (预估成本: $%.4f USD)\n",
		res.TotalUsage.TotalTokens, res.EstimatedCostUSD)
	if res.Audit != nil && len(res.Audit.Issues) > 0 {
		fmt.Fprintf(stdout, "  • 审校批注: %s\n", strings.Join(res.Audit.Issues, "; "))
	}
	if res.Audit != nil && len(res.Audit.ResolvedHookIDs) > 0 {
		fmt.Fprintf(stdout, "  • 伏笔闭环回收: %s (已标记 RESOLVED)\n", strings.Join(res.Audit.ResolvedHookIDs, ", "))
	}
	if res.Committed {
		fmt.Fprintf(stdout, "  • 持久化状态: 已原子归档写入 SQLite (实体状态机已更新)\n")
	} else {
		fmt.Fprintf(stdout, "  • 持久化状态: 未归档 (可手动审核后归档)\n")
	}

	fmt.Fprintf(stdout, "\n--- 章节正文片段 (%d 字) ---\n", len([]rune(res.Content)))
	runes := []rune(res.Content)
	if len(runes) > 300 {
		fmt.Fprintf(stdout, "%s...\n", string(runes[:300]))
	} else {
		fmt.Fprintf(stdout, "%s\n", res.Content)
	}

	return 0
}

func printVolumeProduceSummary(stdout, stderr io.Writer, res *engine.VolumeProduceResult) int {
	if res == nil {
		return 1
	}

	fmt.Fprintln(stdout, "\n==================== 卷批产汇总 ====================")
	fmt.Fprintf(stdout, "  • 生产章数: %d 章 (第 %d-%d 章)\n", len(res.Chapters), res.StartChapter, res.EndChapter)
	if res.Stopped {
		fmt.Fprintf(stderr, "  • 停线章: 第 %d 章 (原因: %s)\n", res.StoppedAtChapter, res.StopReason)
	} else {
		fmt.Fprintf(stdout, "  • 停线章: 无 (全卷正常完成)\n")
	}
	fmt.Fprintf(stdout, "  • Token: %d tokens (Prompt: %d, Completion: %d)\n",
		res.TotalUsage.TotalTokens, res.TotalUsage.PromptTokens, res.TotalUsage.CompletionTokens)
	fmt.Fprintf(stdout, "  • 成本: $%.4f USD\n", res.EstimatedCostUSD)
	fmt.Fprintln(stdout, "====================================================")

	if res.StopReason == "HALTED" {
		return 2
	}
	if res.StopReason == "ERROR" || res.Stopped {
		return 1
	}
	return 0
}
