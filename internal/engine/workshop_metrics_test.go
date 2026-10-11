package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
)

type metricsRecordingStore struct {
	*mockChronicleStore
	metrics []*domain.ChapterQualityMetrics
	saveErr error
}

func (s *metricsRecordingStore) SaveChapterMetrics(ctx context.Context, m *domain.ChapterQualityMetrics) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.metrics = append(s.metrics, m)
	return nil
}

func (s *metricsRecordingStore) ListChapterMetrics(ctx context.Context, projectID string) ([]*domain.ChapterQualityMetrics, error) {
	return s.metrics, nil
}

func setupMetricsTestWorkshop(saveErr error) (*ChapterWorkshop, *metricsRecordingStore) {
	beatsResp := `{
		"beats": [
			{"description": "林凡夜探藏经阁，察觉守卫异常"},
			{"description": "暗影袭来，林凡祭出雷灵符破阵"}
		],
		"state_mutation": {
			"character_mutations": [],
			"inventory_delta": "+古经残页",
			"power_delta": "神魂凝练"
		}
	}`
	reviewResp := `{
		"verdict": "ACCEPTED",
		"score": 92,
		"issues": [],
		"suggestions": "行文流畅"
	}`

	mockClient := &mockWorkshopLLMClient{
		beatsJSON:   beatsResp,
		renderDraft: "夜幕降临，风声呼啸。林凡潜入藏经阁中。暗处忽然掠过一道黑影。符光激射，雷霆震响！",
		reviewJSON:  reviewResp,
	}

	orch := NewOrchestrator(mockClient)
	qGate := NewQualityGate(mockClient, nil)

	baseStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p-metrics-test",
			Title:          "质量指标测试小说",
			TargetPlatform: "番茄脑洞",
			WorldRules:     "道法自然",
		},
		codexEntries: []*domain.CodexEntry{
			{
				ID:        "codex_p_pro",
				ProjectID: "p-metrics-test",
				Category:  domain.CategoryCharacter,
				Archetype: "PROTAGONIST",
				Name:      "林凡",
				Realm:     "练气九层",
			},
		},
		chapters: []*domain.Chapter{
			{ChapterIndex: 1, Title: "第1章", Content: "开宗明义。"},
		},
		hooks: []*domain.PlotHook{},
	}

	recStore := &metricsRecordingStore{
		mockChronicleStore: baseStore,
		saveErr:            saveErr,
	}

	chronicle := NewCanonChronicle(recStore)
	chronicle.SetConfig(&config.Config{
		ContextBudgetTokens: 12000,
	})

	ws := NewChapterWorkshop(orch, chronicle, qGate, recStore)
	return ws, recStore
}

func TestWorkshop_MetricsSavedOnCommit(t *testing.T) {
	ws, recStore := setupMetricsTestWorkshop(nil)

	req := WorkshopProduceRequest{
		ProjectID:       "p-metrics-test",
		ChapterIndex:    2,
		CoreConflict:    "藏经阁盗经风波",
		ReasoningModel:  "mock-r1",
		WriterModel:     "mock-writer",
		ReviewerModel:   "mock-reviewer",
		WordsTarget:     2000,
		AutoCommit:      true,
		MaxRewriteLoops: 2,
		RunID:           "run-metrics-42",
	}

	res, err := ws.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if !res.Committed {
		t.Fatalf("expected chapter to be committed")
	}

	if len(recStore.metrics) != 1 {
		t.Fatalf("expected exactly 1 metrics record, got %d", len(recStore.metrics))
	}

	m := recStore.metrics[0]
	if m.ProjectID != req.ProjectID {
		t.Errorf("ProjectID mismatch: got %q, expected %q", m.ProjectID, req.ProjectID)
	}
	if m.ChapterIndex != req.ChapterIndex {
		t.Errorf("ChapterIndex mismatch: got %d, expected %d", m.ChapterIndex, req.ChapterIndex)
	}
	if m.RunID != req.RunID {
		t.Errorf("RunID mismatch: got %q, expected %q", m.RunID, req.RunID)
	}
	if m.ActualWords != len([]rune(res.Content)) {
		t.Errorf("ActualWords mismatch: got %d, expected %d", m.ActualWords, len([]rune(res.Content)))
	}
	if m.WordsTarget != req.WordsTarget {
		t.Errorf("WordsTarget mismatch: got %d, expected %d", m.WordsTarget, req.WordsTarget)
	}
	if m.Verdict != string(res.Audit.Verdict) {
		t.Errorf("Verdict mismatch: got %q, expected %q", m.Verdict, res.Audit.Verdict)
	}
	if m.Score != res.Audit.Score {
		t.Errorf("Score mismatch: got %d, expected %d", m.Score, res.Audit.Score)
	}
	if m.RewriteLoops != res.RewriteLoops {
		t.Errorf("RewriteLoops mismatch: got %d, expected %d", m.RewriteLoops, res.RewriteLoops)
	}
	if m.UnderTarget != res.UnderTarget {
		t.Errorf("UnderTarget mismatch: got %v, expected %v", m.UnderTarget, res.UnderTarget)
	}
}

func TestWorkshop_MetricsSaveErrorRecordsIssueAndDoesNotBlockCommit(t *testing.T) {
	ws, _ := setupMetricsTestWorkshop(errors.New("simulated metrics store I/O error"))

	req := WorkshopProduceRequest{
		ProjectID:       "p-metrics-test",
		ChapterIndex:    2,
		CoreConflict:    "藏经阁盗经风波",
		ReasoningModel:  "mock-r1",
		WriterModel:     "mock-writer",
		ReviewerModel:   "mock-reviewer",
		WordsTarget:     2000,
		AutoCommit:      true,
		MaxRewriteLoops: 2,
		RunID:           "run-metrics-error-test",
	}

	res, err := ws.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter should succeed despite metrics store error, got: %v", err)
	}

	if !res.Committed {
		t.Fatalf("expected chapter to remain committed despite metrics store error")
	}

	if res.Audit == nil {
		t.Fatalf("expected non-nil Audit report")
	}

	foundWarning := false
	for _, issue := range res.Audit.Issues {
		if strings.Contains(issue, "【指标警告】") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected Audit.Issues to contain '【指标警告】', got: %v", res.Audit.Issues)
	}
}
