package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

func TestResolveABSideConfig(t *testing.T) {
	t.Run("默认值来自 cfg", func(t *testing.T) {
		cfg := &config.Config{
			WriterModel:    "cfg-writer",
			ReasoningModel: "cfg-reasoning",
			ReviewerModel:  "cfg-reviewer",
		}
		rawA := abSideConfig{}
		rawB := abSideConfig{
			WriterModel: "custom-b-writer",
		}

		sideA, sideB, err := resolveABSideConfig(cfg, rawA, rawB)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if sideA.WriterModel != "cfg-writer" || sideA.ReasoningModel != "cfg-reasoning" || sideA.ReviewerModel != "cfg-reviewer" {
			t.Errorf("sideA models not defaulted from cfg: %+v", sideA)
		}
		if sideA.WordsTarget != 2000 || sideA.MaxRewriteLoops != 3 {
			t.Errorf("sideA defaults words/loops expected 2000/3, got %d/%d", sideA.WordsTarget, sideA.MaxRewriteLoops)
		}

		if sideB.WriterModel != "custom-b-writer" {
			t.Errorf("sideB WriterModel expected custom-b-writer, got %s", sideB.WriterModel)
		}
		if sideB.ReasoningModel != "cfg-reasoning" || sideB.ReviewerModel != "cfg-reviewer" {
			t.Errorf("sideB reasoning/reviewer models not defaulted from cfg: %+v", sideB)
		}
		if sideB.WordsTarget != 2000 || sideB.MaxRewriteLoops != 3 {
			t.Errorf("sideB defaults words/loops expected 2000/3, got %d/%d", sideB.WordsTarget, sideB.MaxRewriteLoops)
		}
	})

	t.Run("-writer-b 空 → 报错", func(t *testing.T) {
		cfg := &config.Config{
			WriterModel: "cfg-writer",
		}
		rawA := abSideConfig{WriterModel: "model-a"}
		rawB := abSideConfig{WriterModel: ""}

		_, _, err := resolveABSideConfig(cfg, rawA, rawB)
		if err == nil {
			t.Fatal("expected error when -writer-b is empty, got nil")
		}
		if !strings.Contains(err.Error(), "-writer-b") {
			t.Errorf("expected error message mentioning -writer-b, got %v", err)
		}
	})

	t.Run("五元组全同 → 报错", func(t *testing.T) {
		cfg := &config.Config{}
		rawA := abSideConfig{
			ReasoningModel:  "r1",
			WriterModel:     "w1",
			ReviewerModel:   "rev1",
			WordsTarget:     2000,
			MaxRewriteLoops: 3,
		}
		rawB := abSideConfig{
			ReasoningModel:  "r1",
			WriterModel:     "w1",
			ReviewerModel:   "rev1",
			WordsTarget:     2000,
			MaxRewriteLoops: 3,
		}

		_, _, err := resolveABSideConfig(cfg, rawA, rawB)
		if err == nil {
			t.Fatal("expected error when sideA and sideB are identical, got nil")
		}
		expectedErr := "A/B 配置完全相同，拒绝空转（至少一个 B 侧参数须不同）"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("部分覆盖生效", func(t *testing.T) {
		cfg := &config.Config{
			WriterModel:    "cfg-w",
			ReasoningModel: "cfg-r",
			ReviewerModel:  "cfg-rev",
		}
		rawA := abSideConfig{
			WriterModel: "custom-a",
			WordsTarget: 3500,
		}
		rawB := abSideConfig{
			WriterModel:     "custom-b",
			ReviewerModel:   "custom-rev-b",
			MaxRewriteLoops: 5,
		}

		sideA, sideB, err := resolveABSideConfig(cfg, rawA, rawB)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if sideA.WriterModel != "custom-a" || sideA.WordsTarget != 3500 || sideA.ReasoningModel != "cfg-r" || sideA.ReviewerModel != "cfg-rev" || sideA.MaxRewriteLoops != 3 {
			t.Errorf("sideA unexpected: %+v", sideA)
		}
		if sideB.WriterModel != "custom-b" || sideB.ReviewerModel != "custom-rev-b" || sideB.ReasoningModel != "cfg-r" || sideB.WordsTarget != 2000 || sideB.MaxRewriteLoops != 5 {
			t.Errorf("sideB unexpected: %+v", sideB)
		}
	})

	t.Run("words/loops 归一", func(t *testing.T) {
		cfg := &config.Config{}
		rawA := abSideConfig{
			WriterModel:     "model-a",
			WordsTarget:     -100,
			MaxRewriteLoops: 0,
		}
		rawB := abSideConfig{
			WriterModel:     "model-b",
			WordsTarget:     0,
			MaxRewriteLoops: -5,
		}

		sideA, sideB, err := resolveABSideConfig(cfg, rawA, rawB)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if sideA.WordsTarget != 2000 || sideA.MaxRewriteLoops != 3 {
			t.Errorf("sideA words/loops normalization failed: %d/%d", sideA.WordsTarget, sideA.MaxRewriteLoops)
		}
		if sideB.WordsTarget != 2000 || sideB.MaxRewriteLoops != 3 {
			t.Errorf("sideB words/loops normalization failed: %d/%d", sideB.WordsTarget, sideB.MaxRewriteLoops)
		}
	})
}

func TestFormatABComparison(t *testing.T) {
	sideA := abRunSide{
		Label:                  "Model-Alpha",
		Committed:              true,
		Outcome:                domain.ChapterOutcomeCommitted,
		Verdict:                "ACCEPTED",
		Score:                  85,
		ActualWords:            2100,
		WordQuotaRatio:         1.05,
		RewriteLoops:           1,
		UnresolvedObservations: 0,
		HookClaimsValidated:    2,
		HookClaimsTotal:        2,
		CompactionFailed:       false,
		Tokens:                 4000,
		CostUSD:                0.0150,
	}

	sideB := abRunSide{
		Label:                  "Model-Beta",
		Committed:              false,
		Outcome:                domain.ChapterOutcomeNeedsHuman,
		Verdict:                "REJECT",
		Score:                  70,
		ActualWords:            1900,
		WordQuotaRatio:         0.95,
		RewriteLoops:           3,
		UnresolvedObservations: 2,
		HookClaimsValidated:    1,
		HookClaimsTotal:        2,
		CompactionFailed:       true,
		Tokens:                 5500,
		CostUSD:                0.0210,
	}

	t.Run("文本表格格式含两侧标签、维度行与 B-A 差值及 outcome 标注", func(t *testing.T) {
		out := formatABComparison(sideA, sideB, false)

		if !strings.Contains(out, "Model-Alpha") || !strings.Contains(out, "Model-Beta") {
			t.Errorf("output should contain both labels, got:\n%s", out)
		}

		if !strings.Contains(out, "committed") || !strings.Contains(out, "needs_human") {
			t.Errorf("output should correctly label outcomes, got:\n%s", out)
		}

		// 数值差值检查
		// Score: 70 - 85 = -15
		if !strings.Contains(out, "-15") {
			t.Errorf("output should contain score diff -15, got:\n%s", out)
		}
		// Words: 1900 - 2100 = -200
		if !strings.Contains(out, "-200") {
			t.Errorf("output should contain words diff -200, got:\n%s", out)
		}
		// Loops: 3 - 1 = +2
		if !strings.Contains(out, "+2") {
			t.Errorf("output should contain loops diff +2, got:\n%s", out)
		}
		// Tokens: 5500 - 4000 = +1500
		if !strings.Contains(out, "+1500") {
			t.Errorf("output should contain tokens diff +1500, got:\n%s", out)
		}
		// Cost: 0.0210 - 0.0150 = +$0.0060
		if !strings.Contains(out, "+$0.0060") {
			t.Errorf("output should contain cost diff +$0.0060, got:\n%s", out)
		}
	})

	t.Run("JSON 路径含 a 和 b 键且可反序列化", func(t *testing.T) {
		jsonOut := formatABComparison(sideA, sideB, true)

		var parsed map[string]abRunSide
		if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
			t.Fatalf("failed to unmarshal JSON output: %v, raw:\n%s", err, jsonOut)
		}

		a, okA := parsed["a"]
		b, okB := parsed["b"]
		if !okA || !okB {
			t.Fatalf("expected keys 'a' and 'b' in JSON, got keys: %+v", parsed)
		}

		if a.Label != "Model-Alpha" || b.Label != "Model-Beta" {
			t.Errorf("labels in parsed JSON mismatch: a=%s, b=%s", a.Label, b.Label)
		}
		if a.Score != 85 || b.Score != 70 {
			t.Errorf("scores in parsed JSON mismatch: a=%d, b=%d", a.Score, b.Score)
		}
		if a.Outcome != domain.ChapterOutcomeCommitted || b.Outcome != domain.ChapterOutcomeNeedsHuman {
			t.Errorf("outcomes in parsed JSON mismatch: a=%s, b=%s", a.Outcome, b.Outcome)
		}
	})
}

func TestCollectRunSide(t *testing.T) {
	t.Run("优先从 metrics 列表匹配读取", func(t *testing.T) {
		res := &engine.WorkshopProduceResult{
			ChapterIndex: 5,
			Committed:    true,
			TotalUsage: engine.TokenUsage{
				TotalTokens: 3200,
			},
			EstimatedCostUSD: 0.012,
		}
		metrics := []*domain.ChapterQualityMetrics{
			{
				ChapterIndex:           5,
				Outcome:                domain.ChapterOutcomeCommitted,
				Verdict:                "ACCEPTED",
				Score:                  92,
				ActualWords:            2250,
				WordQuotaRatio:         1.125,
				RewriteLoops:           0,
				UnresolvedObservations: 0,
				HookClaimsValidated:    3,
				HookClaimsTotal:        3,
				CompactionFailed:       false,
				CreatedAt:              time.Now(),
			},
		}

		run := collectRunSide("A", 5, res, nil, metrics)
		if run.Outcome != "committed" || run.Score != 92 || run.ActualWords != 2250 {
			t.Errorf("unexpected run side from metric: %+v", run)
		}
		if run.Tokens != 3200 || run.CostUSD != 0.012 {
			t.Errorf("unexpected usage: tokens=%d, cost=%f", run.Tokens, run.CostUSD)
		}
		if run.Err != "" {
			t.Errorf("unexpected Err: %s", run.Err)
		}
	})

	t.Run("metrics 缺失时回退到 res 字段并记录失败", func(t *testing.T) {
		res := &engine.WorkshopProduceResult{
			ChapterIndex: 5,
			Committed:    false,
			Audit: &domain.AuditReport{
				Verdict:    domain.ReviewVerdictRevision,
				Score:      60,
				NeedsHuman: true,
			},
			ActualWords:    1800,
			WordQuotaRatio: 0.9,
			RewriteLoops:   3,
			TotalUsage: engine.TokenUsage{
				TotalTokens: 4100,
			},
			EstimatedCostUSD: 0.015,
		}

		run := collectRunSide("B", 5, res, nil, nil)
		if run.Outcome != domain.ChapterOutcomeNeedsHuman {
			t.Errorf("expected outcome %s, got %s", domain.ChapterOutcomeNeedsHuman, run.Outcome)
		}
		if run.Score != 60 {
			t.Errorf("expected score 60, got %d", run.Score)
		}
		if run.Err == "" {
			t.Errorf("expected Err to be recorded for missing metrics")
		}
	})

	t.Run("生产发生 error 时正确记录", func(t *testing.T) {
		prodErr := errors.New("network timeout")
		run := collectRunSide("A", 1, nil, prodErr, nil)
		if run.Err != "network timeout" {
			t.Errorf("expected Err to be %q, got %q", "network timeout", run.Err)
		}
		if run.Outcome != "failed" {
			t.Errorf("expected Outcome to be failed, got %s", run.Outcome)
		}
	})
}
