package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/store"
)

func runProduceCLI(argv []string, defaultDataDir string) int {
	fs := flag.NewFlagSet("produce", flag.ExitOnError)
	dataDirFlag := fs.String("data-dir", defaultDataDir, "数据持久化目录")
	projectIDFlag := fs.String("project", "", "作品 ID (默认选择首个已有作品)")
	chapterIdxFlag := fs.Int("chapter", 0, "章节序号 (默认自动递增下一章)")
	conflictFlag := fs.String("conflict", "核心对峙与打破常规预期", "本章核心冲突事实")
	autoCommitFlag := fs.Bool("commit", true, "通过主编终审后自动提交归档至数据库")
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

	// Resolve Chapter Index
	chapterIdx := *chapterIdxFlag
	if chapterIdx <= 0 {
		chapters, _ := s.ListChapters(ctx, projectID)
		chapterIdx = len(chapters) + 1
	}

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

	fmt.Println("\n✓ 生产完成！")
	if res.ResumedPhase != "" {
		fmt.Printf("  • 断点恢复: 从 [%s] 阶段无损续跑\n", res.ResumedPhase)
	}
	fmt.Printf("  • 终审裁决: %s (综合评分: %d/100, 突发度: %d)\n",
		res.Audit.Verdict, res.Audit.Score, res.Audit.BurstinessScore)
	fmt.Printf("  • 定向返工轮次: %d 次\n", res.RewriteLoops)
	fmt.Printf("  • Token消耗: %d tokens (预估成本: $%.4f USD)\n",
		res.TotalUsage.TotalTokens, res.EstimatedCostUSD)
	if len(res.Audit.Issues) > 0 {
		fmt.Printf("  • 审校批注: %s\n", strings.Join(res.Audit.Issues, "; "))
	}
	if len(res.Audit.ResolvedHookIDs) > 0 {
		fmt.Printf("  • 伏笔闭环回收: %s (已标记 RESOLVED)\n", strings.Join(res.Audit.ResolvedHookIDs, ", "))
	}
	if res.Committed {
		fmt.Printf("  • 持久化状态: 已原子归档写入 SQLite (实体状态机已更新)\n")
	} else {
		fmt.Printf("  • 持久化状态: 未归档 (可手动审核后归档)\n")
	}

	fmt.Printf("\n--- 章节正文片段 (%d 字) ---\n", len([]rune(res.Content)))
	runes := []rune(res.Content)
	if len(runes) > 300 {
		fmt.Printf("%s...\n", string(runes[:300]))
	} else {
		fmt.Printf("%s\n", res.Content)
	}

	return 0
}
