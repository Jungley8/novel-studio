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
	if m.Outcome != domain.ChapterOutcomeCommitted {
		t.Errorf("Outcome mismatch: got %q, expected %q", m.Outcome, domain.ChapterOutcomeCommitted)
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

func TestWorkshop_MetricsSavedOnProduced(t *testing.T) {
	ws, recStore := setupMetricsTestWorkshop(nil)

	req := WorkshopProduceRequest{
		ProjectID:       "p-metrics-test",
		ChapterIndex:    2,
		CoreConflict:    "藏经阁盗经风波",
		ReasoningModel:  "mock-r1",
		WriterModel:     "mock-writer",
		ReviewerModel:   "mock-reviewer",
		WordsTarget:     2000,
		AutoCommit:      false,
		MaxRewriteLoops: 1,
		RunID:           "run-metrics-produced",
	}

	res, err := ws.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if res.Committed {
		t.Fatalf("expected chapter NOT to be committed when AutoCommit is false")
	}

	if len(recStore.metrics) != 1 {
		t.Fatalf("expected exactly 1 metrics record, got %d", len(recStore.metrics))
	}

	m := recStore.metrics[0]
	if m.Outcome != domain.ChapterOutcomeProduced {
		t.Errorf("Outcome mismatch: got %q, expected %q", m.Outcome, domain.ChapterOutcomeProduced)
	}
	if m.ProjectID != req.ProjectID {
		t.Errorf("ProjectID mismatch: got %q, expected %q", m.ProjectID, req.ProjectID)
	}
	if m.ChapterIndex != req.ChapterIndex {
		t.Errorf("ChapterIndex mismatch: got %d, expected %d", m.ChapterIndex, req.ChapterIndex)
	}
}

func setupNeedsHumanMetricsWorkshop() (*ChapterWorkshop, *metricsRecordingStore) {
	beatsResp := `{
		"beats": [
			{"phase": "蓄力压迫", "tension": 6, "action": "反派带队堵截主角洞府", "expectation_broken": "反派预料主角会求饶"},
			{"phase": "绝地反转", "tension": 9, "action": "主角引动护山大阵反扣杀", "expectation_broken": "反派全部被镇压"}
		],
		"state_mutation": {
			"inventory_delta": "",
			"power_delta": ""
		}
	}`

	reviewResp := `{
		"verdict": "REVISION",
		"score": 75,
		"issues": ["情节节奏偏慢，亟需压缩过渡语句"],
		"suggestions": "加快节奏"
	}`

	mockClient := &mockWorkshopLLMClient{
		beatsJSON:   beatsResp,
		renderDraft: "夜幕低垂。三名黑衣修士逼近洞府门前。林凡指尖轻扣石壁。阵纹骤亮，雷光咆哮如龙！",
		reviewJSON:  reviewResp,
		settleJSON:  `{"inventory_delta":"","power_delta":"","character_mutations":[],"evidence":[]}`,
	}

	baseStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p-sem-halt",
			Title:          "语义停线测试",
			TargetPlatform: "番茄脑洞",
		},
		chapters: []*domain.Chapter{},
		codexEntries: []*domain.CodexEntry{
			{
				ID:           "c-prot-1",
				ProjectID:    "p-sem-halt",
				Category:     domain.CategoryCharacter,
				Archetype:    "PROTAGONIST",
				TrackingMode: domain.TrackingModeAlwaysInject,
				Name:         "林凡",
				Realm:        "练气圆满",
				Inventory:    "青玉短剑",
			},
		},
		hooks: []*domain.PlotHook{},
	}

	recStore := &metricsRecordingStore{
		mockChronicleStore: baseStore,
	}

	orch := NewOrchestrator(mockClient)
	qGate := NewQualityGate(mockClient, nil)
	chronicle := NewCanonChronicle(recStore)
	ws := NewChapterWorkshop(orch, chronicle, qGate, recStore)
	return ws, recStore
}

func TestWorkshop_MetricsSavedOnNeedsHuman(t *testing.T) {
	ws, recStore := setupNeedsHumanMetricsWorkshop()

	req := WorkshopProduceRequest{
		ProjectID:       "p-sem-halt",
		ChapterIndex:    1,
		CoreConflict:    "宗门反派夜袭抢夺古经",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
		RunID:           "run-metrics-needs-human",
	}

	res, err := ws.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if res.Committed {
		t.Fatalf("expected chapter NOT to be committed on semantic halt")
	}
	if res.Audit == nil || !res.Audit.NeedsHuman {
		t.Fatalf("expected res.Audit.NeedsHuman == true, got %v", res.Audit)
	}

	if len(recStore.metrics) != 1 {
		t.Fatalf("expected exactly 1 metrics record, got %d", len(recStore.metrics))
	}

	m := recStore.metrics[0]
	if m.Outcome != domain.ChapterOutcomeNeedsHuman {
		t.Errorf("Outcome mismatch: got %q, expected %q", m.Outcome, domain.ChapterOutcomeNeedsHuman)
	}
	if m.ProjectID != req.ProjectID {
		t.Errorf("ProjectID mismatch: got %q, expected %q", m.ProjectID, req.ProjectID)
	}
	if m.ChapterIndex != req.ChapterIndex {
		t.Errorf("ChapterIndex mismatch: got %d, expected %d", m.ChapterIndex, req.ChapterIndex)
	}
}

type technicalBlockedRecordingStore struct {
	*mockCheckpointWorkshopStore
	metrics []*domain.ChapterQualityMetrics
}

func (s *technicalBlockedRecordingStore) SaveChapterMetrics(ctx context.Context, m *domain.ChapterQualityMetrics) error {
	s.metrics = append(s.metrics, m)
	return nil
}

func (s *technicalBlockedRecordingStore) ListChapterMetrics(ctx context.Context, projectID string) ([]*domain.ChapterQualityMetrics, error) {
	return s.metrics, nil
}

func setupTechnicalBlockedMetricsWorkshop() (*ChapterWorkshop, *technicalBlockedRecordingStore) {
	beatsResp := `{
		"beats": [
			{"phase": "蓄力压迫", "tension": 6, "action": "反派带队堵截主角洞府", "expectation_broken": "反派预料主角会求饶"},
			{"phase": "绝地反转", "tension": 9, "action": "主角引动护山大阵反扣杀", "expectation_broken": "反派全部被镇压"}
		],
		"state_mutation": {
			"inventory_delta": "",
			"power_delta": ""
		}
	}`

	reviewResp := `{
		"verdict": "ACCEPTED",
		"score": 90,
		"issues": [],
		"suggestions": "质量达标"
	}`

	mockClient := &mockWorkshopLLMClient{
		beatsJSON:   beatsResp,
		renderDraft: "夜幕低垂。三名黑衣修士逼近洞府门前。林凡指尖轻扣石壁。阵纹骤亮，雷光咆哮如龙！",
		reviewJSON:  reviewResp,
		settleJSON:  "{unparseable-json-response",
	}

	baseStore := &mockCheckpointWorkshopStore{
		mockChronicleStore: &mockChronicleStore{
			project: &domain.Project{
				ID:             "p-tech-halt",
				Title:          "技术阻断停线测试",
				TargetPlatform: "番茄脑洞",
			},
			chapters: []*domain.Chapter{},
			codexEntries: []*domain.CodexEntry{
				{
					ID:           "c-prot-1",
					ProjectID:    "p-tech-halt",
					Category:     domain.CategoryCharacter,
					Archetype:    "PROTAGONIST",
					TrackingMode: domain.TrackingModeAlwaysInject,
					Name:         "林凡",
					Realm:        "练气圆满",
					Inventory:    "青玉短剑",
				},
			},
		},
	}

	recStore := &technicalBlockedRecordingStore{
		mockCheckpointWorkshopStore: baseStore,
	}

	orch := NewOrchestrator(mockClient)
	qGate := NewQualityGate(mockClient, nil)
	chronicle := NewCanonChronicle(recStore)
	ws := NewChapterWorkshop(orch, chronicle, qGate, recStore)
	return ws, recStore
}

func TestWorkshop_MetricsSavedOnTechnicalBlocked(t *testing.T) {
	ws, recStore := setupTechnicalBlockedMetricsWorkshop()

	req := WorkshopProduceRequest{
		ProjectID:       "p-tech-halt",
		ChapterIndex:    1,
		CoreConflict:    "宗门反派夜袭抢夺古经",
		AutoCommit:      true,
		MaxRewriteLoops: 1,
		RunID:           "run-metrics-tech-blocked",
	}

	res, err := ws.ProduceChapter(context.Background(), req)
	if err != nil {
		t.Fatalf("ProduceChapter failed: %v", err)
	}

	if !res.TechnicalBlocked {
		t.Fatalf("expected TechnicalBlocked == true, got false")
	}
	if res.Committed {
		t.Errorf("expected Committed == false, got true")
	}

	if len(recStore.metrics) != 1 {
		t.Fatalf("expected exactly 1 metrics record, got %d", len(recStore.metrics))
	}

	m := recStore.metrics[0]
	if m.Outcome != domain.ChapterOutcomeTechnicalBlocked {
		t.Errorf("Outcome mismatch: got %q, expected %q", m.Outcome, domain.ChapterOutcomeTechnicalBlocked)
	}
	if m.ProjectID != req.ProjectID {
		t.Errorf("ProjectID mismatch: got %q, expected %q", m.ProjectID, req.ProjectID)
	}
	if m.ChapterIndex != req.ChapterIndex {
		t.Errorf("ChapterIndex mismatch: got %d, expected %d", m.ChapterIndex, req.ChapterIndex)
	}
}
