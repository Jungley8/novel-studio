package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/store"
)

func runGenesisCLI(argv []string, defaultDataDir string) int {
	fs := flag.NewFlagSet("genesis", flag.ExitOnError)
	dataDirFlag := fs.String("data-dir", defaultDataDir, "数据持久化目录")
	titleFlag := fs.String("title", "", "作品书名 (必填)")
	platformFlag := fs.String("platform", "起点仙侠", "目标平台 (起点仙侠/番茄脑洞/知乎盐言/通用网文)")
	conceptFlag := fs.String("concept", "", "核心脑洞灵感与题材设定 (必填)")
	modelFlag := fs.String("model", "", "推演大模型 (默认使用 ReasonerModel)")
	_ = fs.Parse(argv)

	if strings.TrimSpace(*titleFlag) == "" {
		fmt.Fprintf(os.Stderr, "✗ 请指定作品书名: -title \"书名\"\n")
		return 1
	}
	if strings.TrimSpace(*conceptFlag) == "" {
		fmt.Fprintf(os.Stderr, "✗ 请指定核心脑洞与题材灵感: -concept \"核心立意与设定\"\n")
		return 1
	}

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

	reasoningModel := *modelFlag
	if reasoningModel == "" {
		reasoningModel = cfg.ReasoningModel
	}

	fmt.Printf("\n🌌 正在启动宏观创世引擎 (Genesis Architecture)...\n")
	fmt.Printf("   书名: 《%s》 | 目标平台: %s\n", *titleFlag, *platformFlag)
	fmt.Printf("   推演模型: %s\n", reasoningModel)
	fmt.Printf("   灵感输入: %s\n", *conceptFlag)
	fmt.Println("------------------------------------------------------------")

	llmClient := engine.NewHTTPLLMClient(cfg.APIBase, cfg.APIKey)
	orch := engine.NewOrchestrator(llmClient)

	ctx := context.Background()
	fw, err := orch.BootstrapFramework(ctx, reasoningModel, engine.FrameworkBootstrapRequest{
		Title:          *titleFlag,
		TargetPlatform: *platformFlag,
		CoreConcept:    *conceptFlag,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 宏观总纲推演失败: %v\n", err)
		return 1
	}

	projectID := fmt.Sprintf("proj_%d", time.Now().UnixNano())
	var worldRules strings.Builder
	worldRules.WriteString("【天道法则与核心世界公理】\n")
	for i, axiom := range fw.WorldAxioms {
		worldRules.WriteString(fmt.Sprintf("%d. %s\n", i+1, axiom))
	}
	if len(fw.PowerLadder) > 0 {
		worldRules.WriteString("\n【战力阶梯与突破反噬】\n")
		for _, tier := range fw.PowerLadder {
			worldRules.WriteString(fmt.Sprintf("- %s: 关隘[%s] | 代价[%s]\n", tier.Realm, tier.Bottleneck, tier.Drawback))
		}
	}

	initialRealm := "练气一层"
	if len(fw.PowerLadder) > 0 {
		initialRealm = fw.PowerLadder[0].Realm
	}

	proj := &domain.Project{
		ID:             projectID,
		Title:          *titleFlag,
		TargetPlatform: *platformFlag,
		WorldRules:     strings.TrimSpace(worldRules.String()),
		Protagonist: domain.Protagonist{
			NameAndLevel: fmt.Sprintf("主角 (%s)", initialRealm),
			Inventory:    "残破黑铁, 粗布短衫",
			CoreGoal:     fw.ThemePremise,
			HealthStatus: "良好",
		},
		Framework: fw,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.SaveProject(ctx, proj); err != nil {
		fmt.Fprintf(os.Stderr, "✗ 保存作品失败: %v\n", err)
		return 1
	}

	// Save seed hooks
	for i, sh := range fw.SeedHooks {
		hook := &domain.PlotHook{
			ID:             fmt.Sprintf("hook_%s_%d", projectID, i+1),
			ProjectID:      projectID,
			Title:          sh.Title,
			Details:        sh.Details,
			CreatedChapter: sh.CreatedChapter,
			TargetChapter:  sh.TargetChapter,
			Status:         domain.HookStatusOpen,
			CreatedAt:      time.Now(),
		}
		_ = s.SavePlotHook(ctx, hook)
	}

	// Print Summary
	fmt.Printf("\n✨ 宏观总纲创世推演完成！作品已原子落库 (ID: %s)\n", projectID)
	fmt.Printf("📖 核心立意: %s\n", fw.ThemePremise)

	fmt.Println("\n⚖️  天道法则与世界公理:")
	for i, a := range fw.WorldAxioms {
		fmt.Printf("   %d. %s\n", i+1, a)
	}

	fmt.Println("\n⚡ 战力境界与晋升代价阶梯:")
	for _, t := range fw.PowerLadder {
		fmt.Printf("   • [%s] %s (关卡: %s | 代价: %s)\n", t.Realm, t.Description, t.Bottleneck, t.Drawback)
	}

	fmt.Println("\n🚩 核心势力与宗门阵营:")
	for _, f := range fw.Factions {
		fmt.Printf("   • %s [%s] 主张: %s (威胁: %s)\n", f.Name, f.Alignment, f.Doctrine, f.ThreatLevel)
	}

	fmt.Println("\n📜 分卷主线大纲:")
	for _, v := range fw.VolumeArcs {
		fmt.Printf("   • 第 %d 卷:《%s》(约%d章) - 目标: %s | 高潮: %s\n", v.VolumeIndex, v.Title, v.EstimatedChapters, v.CoreGoal, v.Climax)
	}

	fmt.Printf("\n🌱 初始种子伏笔 (%d 个已注入伏笔账本):\n", len(fw.SeedHooks))
	for _, h := range fw.SeedHooks {
		fmt.Printf("   • 《%s》(预定第%d章回收): %s\n", h.Title, h.TargetChapter, h.Details)
	}

	fmt.Println("\n------------------------------------------------------------")
	fmt.Printf("🚀 下一步：可直接使用工坊生产第一章：\n")
	fmt.Printf("   novel-studio produce -project %s -conflict \"开篇核心冲突事件\"\n\n", projectID)

	return 0
}
