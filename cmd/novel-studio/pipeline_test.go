package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/engine"
)

func TestRunPipelineCLI_MissingRequiredFlags(t *testing.T) {
	tempDir := t.TempDir()

	// 1. 缺 -title
	var stdout1, stderr1 bytes.Buffer
	code1 := runPipeline([]string{"-concept", "测试灵感设定"}, tempDir, &stdout1, &stderr1)
	if code1 != 1 {
		t.Fatalf("expected exit code 1 when -title is missing, got %d", code1)
	}
	if !strings.Contains(stderr1.String(), "请指定作品书名") {
		t.Errorf("expected stderr to contain '请指定作品书名', got: %s", stderr1.String())
	}

	// 2. 缺 -concept
	var stdout2, stderr2 bytes.Buffer
	code2 := runPipeline([]string{"-title", "测试书名"}, tempDir, &stdout2, &stderr2)
	if code2 != 1 {
		t.Fatalf("expected exit code 1 when -concept is missing, got %d", code2)
	}
	if !strings.Contains(stderr2.String(), "请指定核心脑洞") {
		t.Errorf("expected stderr to contain '请指定核心脑洞', got: %s", stderr2.String())
	}

	// 3. 两者均为空
	var stdout3, stderr3 bytes.Buffer
	code3 := runPipeline([]string{}, tempDir, &stdout3, &stderr3)
	if code3 != 1 {
		t.Fatalf("expected exit code 1 when all required flags missing, got %d", code3)
	}
}

func TestRunPipelineCLI_HelpSmoke(t *testing.T) {
	tempDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runPipeline([]string{"-h"}, tempDir, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0 on -h, got %d", code)
	}
	out := stderr.String()
	if !strings.Contains(out, "Usage of pipeline:") {
		t.Errorf("expected usage message in output, got: %s", out)
	}
	if !strings.Contains(out, "退出码:") {
		t.Errorf("expected exit codes in usage message, got: %s", out)
	}
}

func TestRunPipelineCLI_ConfigAndExecutionFailureExit1(t *testing.T) {
	// 1. 非法 config.json 导致加载配置失败
	tempDir1 := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempDir1, "config.json"), []byte("{invalid-json"), 0644)
	var stdout1, stderr1 bytes.Buffer
	code1 := runPipeline([]string{"-title", "测试书名", "-concept", "测试核心脑洞"}, tempDir1, &stdout1, &stderr1)
	if code1 != 1 {
		t.Fatalf("expected exit code 1 on corrupted config, got %d", code1)
	}
	if !strings.Contains(stderr1.String(), "加载配置失败") {
		t.Errorf("expected stderr to note '加载配置失败', got: %s", stderr1.String())
	}

	// 2. 缺失 APIKey 导致流水线推演执行失败
	tempDir2 := t.TempDir()
	var stdout2, stderr2 bytes.Buffer
	code2 := runPipeline([]string{"-title", "测试书名", "-concept", "测试核心脑洞"}, tempDir2, &stdout2, &stderr2)
	if code2 != 1 {
		t.Fatalf("expected exit code 1 on missing API key/execution failure, got %d", code2)
	}
	if !strings.Contains(stderr2.String(), "流水线执行失败") {
		t.Errorf("expected stderr to note '流水线执行失败', got: %s", stderr2.String())
	}
}

func TestBuildPipelineRequest_FlagsMapping(t *testing.T) {
	// 1. 默认值验证
	fs1, opts1 := newPipelineFlagSet("/tmp/dummy", nil)
	err := fs1.Parse([]string{"-title", "我的仙门", "-concept", "凡人修仙"})
	if err != nil {
		t.Fatalf("failed to parse default flags: %v", err)
	}

	req1 := buildPipelineRequest(opts1, nil, "default-reasoner-model", nil)
	if req1.Title != "我的仙门" {
		t.Errorf("expected Title '我的仙门', got %s", req1.Title)
	}
	if req1.Concept != "凡人修仙" {
		t.Errorf("expected Concept '凡人修仙', got %s", req1.Concept)
	}
	if req1.TargetPlatform != "起点仙侠" {
		t.Errorf("expected TargetPlatform '起点仙侠', got %s", req1.TargetPlatform)
	}
	if req1.Volumes != 0 {
		t.Errorf("expected Volumes 0, got %d", req1.Volumes)
	}
	if req1.AutoLockIntents != false {
		t.Errorf("expected AutoLockIntents false, got %v", req1.AutoLockIntents)
	}
	if req1.WordsTarget != 2000 {
		t.Errorf("expected WordsTarget 2000, got %d", req1.WordsTarget)
	}
	if req1.MaxRewriteLoops != 3 {
		t.Errorf("expected MaxRewriteLoops 3, got %d", req1.MaxRewriteLoops)
	}
	if req1.NarrativeStyle != "" {
		t.Errorf("expected empty NarrativeStyle, got %s", req1.NarrativeStyle)
	}
	if req1.ChapterFrom != 0 {
		t.Errorf("expected ChapterFrom 0, got %d", req1.ChapterFrom)
	}
	if req1.ReasoningModel != "default-reasoner-model" {
		t.Errorf("expected ReasoningModel fallback to default, got %s", req1.ReasoningModel)
	}

	// 2. 显式覆盖参数
	fs2, opts2 := newPipelineFlagSet("/tmp/dummy", nil)
	err = fs2.Parse([]string{
		"-title", "长生武道",
		"-concept", "极道流加点升级",
		"-platform", "番茄脑洞",
		"-volumes", "2",
		"-auto-lock",
		"-words", "3000",
		"-max-rewrite", "5",
		"-style", "dark-fantasy",
		"-chapter-from", "10",
		"-model", "custom-reasoner-v1",
	})
	if err != nil {
		t.Fatalf("failed to parse explicit flags: %v", err)
	}
	fs2.Visit(func(f *flag.Flag) {
		if f.Name == "auto-lock" {
			opts2.AutoLockSet = true
		}
	})

	req2 := buildPipelineRequest(opts2, nil, "default-model", nil)
	if req2.Title != "长生武道" {
		t.Errorf("expected Title '长生武道', got %s", req2.Title)
	}
	if req2.Concept != "极道流加点升级" {
		t.Errorf("expected Concept '极道流加点升级', got %s", req2.Concept)
	}
	if req2.TargetPlatform != "番茄脑洞" {
		t.Errorf("expected TargetPlatform '番茄脑洞', got %s", req2.TargetPlatform)
	}
	if req2.Volumes != 2 {
		t.Errorf("expected Volumes 2, got %d", req2.Volumes)
	}
	if req2.AutoLockIntents != true {
		t.Errorf("expected AutoLockIntents true, got %v", req2.AutoLockIntents)
	}
	if req2.WordsTarget != 3000 {
		t.Errorf("expected WordsTarget 3000, got %d", req2.WordsTarget)
	}
	if req2.MaxRewriteLoops != 5 {
		t.Errorf("expected MaxRewriteLoops 5, got %d", req2.MaxRewriteLoops)
	}
	if req2.NarrativeStyle != "dark-fantasy" {
		t.Errorf("expected NarrativeStyle 'dark-fantasy', got %s", req2.NarrativeStyle)
	}
	if req2.ChapterFrom != 10 {
		t.Errorf("expected ChapterFrom 10, got %d", req2.ChapterFrom)
	}
	if req2.ReasoningModel != "custom-reasoner-v1" {
		t.Errorf("expected ReasoningModel 'custom-reasoner-v1', got %s", req2.ReasoningModel)
	}
}

func TestBuildPipelineRequest_ConfigPriority(t *testing.T) {
	cfg := &config.Config{
		ReasoningModel:         "cfg-reasoner",
		DefaultWordsTarget:     3500,
		DefaultMaxRewriteLoops: 6,
		DefaultNarrativeStyle:  "cfg-hardboiled",
		DefaultAutoLockIntents: true,
	}

	// 1. Flag overrides Config and Fallback (Flag > Config > Fallback)
	fs1, opts1 := newPipelineFlagSet("/tmp/dummy", nil)
	_ = fs1.Parse([]string{
		"-title", "Flag书名",
		"-concept", "Flag设定",
		"-words", "2500",
		"-max-rewrite", "4",
		"-style", "flag-wuxia",
		"-auto-lock=false",
	})
	fs1.Visit(func(f *flag.Flag) {
		if f.Name == "auto-lock" {
			opts1.AutoLockSet = true
		}
	})
	req1 := buildPipelineRequest(opts1, cfg, "default-model", nil)
	if req1.WordsTarget != 2500 {
		t.Errorf("expected flag words 2500, got %d", req1.WordsTarget)
	}
	if req1.MaxRewriteLoops != 4 {
		t.Errorf("expected flag max-rewrite 4, got %d", req1.MaxRewriteLoops)
	}
	if req1.NarrativeStyle != "flag-wuxia" {
		t.Errorf("expected flag style 'flag-wuxia', got %s", req1.NarrativeStyle)
	}
	if req1.AutoLockIntents != false {
		t.Errorf("expected explicit flag auto-lock false, got %v", req1.AutoLockIntents)
	}

	// 2. Config overrides Fallback when Flag is unset (Config > Fallback)
	fs2, opts2 := newPipelineFlagSet("/tmp/dummy", nil)
	_ = fs2.Parse([]string{
		"-title", "Config书名",
		"-concept", "Config设定",
	})
	fs2.Visit(func(f *flag.Flag) {
		if f.Name == "auto-lock" {
			opts2.AutoLockSet = true
		}
	})
	req2 := buildPipelineRequest(opts2, cfg, "default-model", nil)
	if req2.WordsTarget != 3500 {
		t.Errorf("expected cfg words 3500, got %d", req2.WordsTarget)
	}
	if req2.MaxRewriteLoops != 6 {
		t.Errorf("expected cfg max-rewrite 6, got %d", req2.MaxRewriteLoops)
	}
	if req2.NarrativeStyle != "cfg-hardboiled" {
		t.Errorf("expected cfg style 'cfg-hardboiled', got %s", req2.NarrativeStyle)
	}
	if req2.AutoLockIntents != true {
		t.Errorf("expected cfg auto-lock true, got %v", req2.AutoLockIntents)
	}
	if req2.ReasoningModel != "cfg-reasoner" {
		t.Errorf("expected cfg model 'cfg-reasoner', got %s", req2.ReasoningModel)
	}

	// 3. Fallback when both Flag and Config are unset or zero
	emptyCfg := &config.Config{}
	fs3, opts3 := newPipelineFlagSet("/tmp/dummy", nil)
	_ = fs3.Parse([]string{
		"-title", "Fallback书名",
		"-concept", "Fallback设定",
	})
	req3 := buildPipelineRequest(opts3, emptyCfg, "default-model", nil)
	if req3.WordsTarget != 2000 {
		t.Errorf("expected fallback words 2000, got %d", req3.WordsTarget)
	}
	if req3.MaxRewriteLoops != 3 {
		t.Errorf("expected fallback max-rewrite 3, got %d", req3.MaxRewriteLoops)
	}
	if req3.NarrativeStyle != "" {
		t.Errorf("expected fallback style empty, got %s", req3.NarrativeStyle)
	}
	if req3.AutoLockIntents != false {
		t.Errorf("expected fallback auto-lock false, got %v", req3.AutoLockIntents)
	}
	if req3.ReasoningModel != "default-model" {
		t.Errorf("expected fallback model 'default-model', got %s", req3.ReasoningModel)
	}
}

func TestPrintPipelineSummary_HaltedExit2(t *testing.T) {
	var stdout, stderr bytes.Buffer
	res := &engine.PipelineResult{
		ProjectID:        "proj_halted_001",
		Title:            "剑临诸天",
		Halted:           true,
		HaltReason:       "意图卡未锁定（第 1 章…），已停线待人工锁定",
		EstimatedCostUSD: 0.0521,
		Notes:            []string{"关系图谱推演完成，写入 5 条关系"},
		TotalUsage: engine.TokenUsage{
			PromptTokens:     1000,
			CompletionTokens: 500,
			TotalTokens:      1500,
		},
		Volumes: []*engine.VolumeProduceResult{
			{
				VolumeIndex:      1,
				StartChapter:     1,
				EndChapter:       10,
				Stopped:          true,
				StoppedAtChapter: 1,
				StopReason:       "HALTED",
				Chapters: []engine.VolumeChapterSummary{
					{ChapterIndex: 1, Committed: false, NeedsHuman: true},
				},
			},
		},
	}

	code := printPipelineSummary(&stdout, &stderr, res)
	if code != 2 {
		t.Fatalf("expected exit code 2 on halted pipeline, got %d", code)
	}

	errStr := stderr.String()
	if !strings.Contains(errStr, "流水线已停线待人工") {
		t.Errorf("expected stderr to contain halt warning, got: %s", errStr)
	}

	outStr := stdout.String()
	if !strings.Contains(outStr, "proj_halted_001") {
		t.Errorf("expected stdout to contain ProjectID, got: %s", outStr)
	}
	if !strings.Contains(outStr, "已停线待人工") {
		t.Errorf("expected stdout to note halted status, got: %s", outStr)
	}
	if !strings.Contains(outStr, "停线章: 第 1 章 (原因: HALTED)") {
		t.Errorf("expected stdout to note stopped chapter info, got: %s", outStr)
	}
	if !strings.Contains(outStr, "关系图谱推演完成") {
		t.Errorf("expected stdout to note notes, got: %s", outStr)
	}
}

func TestPrintPipelineSummary_SuccessExit0(t *testing.T) {
	var stdout, stderr bytes.Buffer
	res := &engine.PipelineResult{
		ProjectID:        "proj_success_001",
		Title:            "长生道主",
		Halted:           false,
		EstimatedCostUSD: 0.1234,
		TotalUsage: engine.TokenUsage{
			PromptTokens:     5000,
			CompletionTokens: 2000,
			TotalTokens:      7000,
		},
		Volumes: []*engine.VolumeProduceResult{
			{
				VolumeIndex:  1,
				StartChapter: 1,
				EndChapter:   3,
				Stopped:      false,
				Chapters: []engine.VolumeChapterSummary{
					{ChapterIndex: 1, Committed: true},
					{ChapterIndex: 2, Committed: true},
					{ChapterIndex: 3, Committed: true},
				},
			},
		},
	}

	code := printPipelineSummary(&stdout, &stderr, res)
	if code != 0 {
		t.Fatalf("expected exit code 0 on successful pipeline, got %d", code)
	}

	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr on success, got: %s", stderr.String())
	}

	outStr := stdout.String()
	if !strings.Contains(outStr, "全流程流水线汇总") {
		t.Errorf("expected summary header, got: %s", outStr)
	}
	if !strings.Contains(outStr, "Committed 3 章 / 生产 3 章") {
		t.Errorf("expected committed volume info, got: %s", outStr)
	}
	if !strings.Contains(outStr, "停线章: 无") {
		t.Errorf("expected no stop notice, got: %s", outStr)
	}
	if !strings.Contains(outStr, "停线状态:  无 (全部完成)") {
		t.Errorf("expected completed halt status, got: %s", outStr)
	}
}

func TestPrintPipelineSummary_NilDefensive(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := printPipelineSummary(&stdout, &stderr, nil)
	if code != 1 {
		t.Fatalf("expected exit code 1 on nil result, got %d", code)
	}
}

func TestFormatPipelineEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    engine.PipelineEvent
		contains []string
	}{
		{
			name: "Normal start event",
			event: engine.PipelineEvent{
				Phase:   "PIPELINE_START",
				Message: "开始执行流水线: 《星空彼岸》",
			},
			contains: []string{"[PIPELINE_START]", "开始执行流水线: 《星空彼岸》"},
		},
		{
			name: "Needs human halt event",
			event: engine.PipelineEvent{
				Phase:        "HALTED",
				VolumeIndex:  1,
				ChapterIndex: 5,
				Message:      "意图卡未锁定",
				NeedsHuman:   true,
			},
			contains: []string{"⚠", "[HALTED]", "[第1卷/第5章]", "意图卡未锁定"},
		},
		{
			name: "Volume ready event with only volume index",
			event: engine.PipelineEvent{
				Phase:       "INTENTS_READY",
				VolumeIndex: 2,
				Message:     "第 2 卷意图卡已就绪",
			},
			contains: []string{"[INTENTS_READY]", "[第2卷]", "第 2 卷意图卡已就绪"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted := formatPipelineEvent(tt.event)
			for _, exp := range tt.contains {
				if !strings.Contains(formatted, exp) {
					t.Errorf("expected formatted string %q to contain %q", formatted, exp)
				}
			}
		})
	}
}
