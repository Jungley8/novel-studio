package main

// 退出码契约:
//   0 = 全部完成
//   1 = 执行失败 (Run 返回 error)
//   2 = 停线待人工 (res.Halted == true)

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
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/store"
)

type pipelineCLIOptions struct {
	DataDir         string
	Title           string
	Concept         string
	Platform        string
	Volumes         int
	AutoLock        bool
	Words           int
	MaxRewriteLoops int
	Style           string
	ChapterFrom     int
	Model           string
}

func newPipelineFlagSet(defaultDataDir string, stderr io.Writer) (*flag.FlagSet, *pipelineCLIOptions) {
	opts := &pipelineCLIOptions{}
	fs := flag.NewFlagSet("pipeline", flag.ContinueOnError)
	if stderr != nil {
		fs.SetOutput(stderr)
	}
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage of pipeline:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\n退出码:\n  0: 全部完成\n  1: 执行失败\n  2: 停线待人工\n")
	}
	fs.StringVar(&opts.DataDir, "data-dir", defaultDataDir, "数据持久化目录")
	fs.StringVar(&opts.Title, "title", "", "作品书名 (必填)")
	fs.StringVar(&opts.Concept, "concept", "", "核心脑洞灵感与题材设定 (必填)")
	fs.StringVar(&opts.Platform, "platform", "起点仙侠", "目标平台 (起点仙侠/番茄脑洞/知乎盐言/通用网文)")
	fs.IntVar(&opts.Volumes, "volumes", 0, "规划执行卷数 (默认 0 表示全部分卷)")
	fs.BoolVar(&opts.AutoLock, "auto-lock", false, "生成意图卡后自动锁定并继续推进 (默认 false: 生成意图后停线待人工锁定，安全默认)")
	fs.IntVar(&opts.Words, "words", 2000, "每章目标字数 (默认 2000)")
	fs.IntVar(&opts.MaxRewriteLoops, "max-rewrite", 3, "单章最大返工轮次 (默认 3)")
	fs.StringVar(&opts.Style, "style", "", "全书叙事风格约束")
	fs.IntVar(&opts.ChapterFrom, "chapter-from", 0, "起始生产章节序号 (默认 0 表示从卷首开始)")
	fs.StringVar(&opts.Model, "model", "", "推演大模型 (默认使用 config.json 中的 reasoning_model)")
	return fs, opts
}

func buildPipelineRequest(opts *pipelineCLIOptions, defaultReasoningModel string, onProgress func(engine.PipelineEvent)) engine.PipelineRequest {
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = defaultReasoningModel
	}
	return engine.PipelineRequest{
		Title:           opts.Title,
		TargetPlatform:  opts.Platform,
		Concept:         opts.Concept,
		ReasoningModel:  model,
		Volumes:         opts.Volumes,
		AutoLockIntents: opts.AutoLock,
		WordsTarget:     opts.Words,
		MaxRewriteLoops: opts.MaxRewriteLoops,
		NarrativeStyle:  opts.Style,
		ChapterFrom:     opts.ChapterFrom,
		OnProgress:      onProgress,
	}
}

func formatPipelineEvent(ev engine.PipelineEvent) string {
	prefix := " "
	if ev.NeedsHuman {
		prefix = "⚠"
	}
	var loc string
	if ev.VolumeIndex > 0 && ev.ChapterIndex > 0 {
		loc = fmt.Sprintf(" [第%d卷/第%d章]", ev.VolumeIndex, ev.ChapterIndex)
	} else if ev.VolumeIndex > 0 {
		loc = fmt.Sprintf(" [第%d卷]", ev.VolumeIndex)
	} else if ev.ChapterIndex > 0 {
		loc = fmt.Sprintf(" [第%d章]", ev.ChapterIndex)
	}
	return fmt.Sprintf("%s [%s]%s %s", prefix, ev.Phase, loc, ev.Message)
}

func printPipelineSummary(stdout, stderr io.Writer, res *engine.PipelineResult) int {
	if res == nil {
		return 1
	}

	fmt.Fprintln(stdout, "\n==================== 全流程流水线汇总 ====================")
	fmt.Fprintf(stdout, "  • 作品 ID:   %s\n", res.ProjectID)
	fmt.Fprintf(stdout, "  • 作品书名: 《%s》\n", res.Title)
	if len(res.Volumes) == 0 {
		fmt.Fprintln(stdout, "  • 卷生产:    无已生产卷")
	} else {
		fmt.Fprintf(stdout, "  • 卷生产 (%d 卷):\n", len(res.Volumes))
		for _, vol := range res.Volumes {
			if vol == nil {
				continue
			}
			committedCount := 0
			for _, ch := range vol.Chapters {
				if ch.Committed {
					committedCount++
				}
			}
			if vol.Stopped {
				fmt.Fprintf(stdout, "    - 第 %d 卷: Committed %d 章 / 生产 %d 章 (第 %d-%d 章) | 停线章: 第 %d 章 (原因: %s)\n",
					vol.VolumeIndex, committedCount, len(vol.Chapters), vol.StartChapter, vol.EndChapter, vol.StoppedAtChapter, vol.StopReason)
			} else {
				fmt.Fprintf(stdout, "    - 第 %d 卷: Committed %d 章 / 生产 %d 章 (第 %d-%d 章) | 停线章: 无\n",
					vol.VolumeIndex, committedCount, len(vol.Chapters), vol.StartChapter, vol.EndChapter)
			}
		}
	}
	fmt.Fprintf(stdout, "  • Token:     %d tokens (Prompt: %d, Completion: %d)\n",
		res.TotalUsage.TotalTokens, res.TotalUsage.PromptTokens, res.TotalUsage.CompletionTokens)
	fmt.Fprintf(stdout, "  • 成本:      $%.4f USD\n", res.EstimatedCostUSD)
	if res.Halted {
		fmt.Fprintf(stdout, "  • 停线状态:  已停线待人工 (原因: %s)\n", res.HaltReason)
		if stderr != nil {
			fmt.Fprintf(stderr, "⚠ 流水线已停线待人工: %s\n", res.HaltReason)
		}
	} else {
		fmt.Fprintln(stdout, "  • 停线状态:  无 (全部完成)")
	}
	if len(res.Notes) > 0 {
		fmt.Fprintf(stdout, "  • 备注 (%d 条):\n", len(res.Notes))
		for _, note := range res.Notes {
			fmt.Fprintf(stdout, "    - %s\n", note)
		}
	}
	fmt.Fprintln(stdout, "==========================================================")

	if res.Halted {
		return 2
	}
	return 0
}

func runPipelineCLI(argv []string, defaultDataDir string) int {
	return runPipeline(argv, defaultDataDir, os.Stdout, os.Stderr)
}

func runPipeline(argv []string, defaultDataDir string, stdout, stderr io.Writer) int {
	fs, opts := newPipelineFlagSet(defaultDataDir, stderr)
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if strings.TrimSpace(opts.Title) == "" {
		fmt.Fprintf(stderr, "✗ 请指定作品书名: -title \"书名\"\n")
		return 1
	}
	if strings.TrimSpace(opts.Concept) == "" {
		fmt.Fprintf(stderr, "✗ 请指定核心脑洞与题材设定: -concept \"核心立意与设定\"\n")
		return 1
	}

	dataDir := opts.DataDir
	cfgPath := filepath.Join(dataDir, "config.json")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(stderr, "✗ 加载配置失败: %v\n", err)
		return 1
	}

	dbPath := filepath.Join(dataDir, "novel.db")
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "✗ 打开 SQLite 数据库失败: %v\n", err)
		return 1
	}
	defer s.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx = activity.WithSource(ctx, activity.SourceCLIProduce)

	llmClient := engine.NewHTTPLLMClient(cfg.APIBase, cfg.APIKey)
	router := engine.NewLLMRouter(llmClient)
	router.UpdateFromConfig(cfg)

	orch := engine.NewOrchestrator(router)
	chronicle := engine.NewCanonChronicle(s)
	chronicle.SetConfig(cfg)
	qualityGate := engine.NewQualityGate(router, nil)
	workshop := engine.NewChapterWorkshop(orch, chronicle, qualityGate, s)

	batch := engine.NewVolumeBatch(workshop, s)
	pipeline := engine.NewStudioPipeline(orch, batch, s)

	req := buildPipelineRequest(opts, cfg.ReasoningModel, func(ev engine.PipelineEvent) {
		fmt.Fprintln(stdout, formatPipelineEvent(ev))
	})

	fmt.Fprintln(stdout, "==================================================================")
	fmt.Fprintln(stdout, "  NovelStudio Autonomous Pipeline (全自动全流程创作管道)")
	fmt.Fprintf(stdout, "  • 作品书名: 《%s》\n", opts.Title)
	fmt.Fprintf(stdout, "  • 目标平台: %s\n", opts.Platform)
	fmt.Fprintf(stdout, "  • 核心脑洞: %s\n", opts.Concept)
	fmt.Fprintf(stdout, "  • 推理模型: %s\n", req.ReasoningModel)
	fmt.Fprintf(stdout, "  • 规划卷数: %d 卷 (0=全部)\n", opts.Volumes)
	fmt.Fprintf(stdout, "  • 自动锁定: %v\n", opts.AutoLock)
	fmt.Fprintln(stdout, "==================================================================")

	res, err := pipeline.Run(ctx, req)
	if err != nil {
		fmt.Fprintf(stderr, "✗ 流水线执行失败: %v\n", err)
		return 1
	}

	return printPipelineSummary(stdout, stderr, res)
}
