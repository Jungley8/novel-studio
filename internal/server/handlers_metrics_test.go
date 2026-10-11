package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestProjectMetrics_Success(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	proj := &domain.Project{
		ID:    "proj-metrics-success",
		Title: "质量指标测试作品",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	m1 := &domain.ChapterQualityMetrics{
		ProjectID:              proj.ID,
		ChapterIndex:           1,
		RunID:                  "run-1",
		ActualWords:            2200,
		WordsTarget:            2000,
		WordQuotaRatio:         1.1,
		UnderTarget:            false,
		Verdict:                "ACCEPTED",
		Score:                  95,
		RewriteLoops:           0,
		UnresolvedObservations: 0,
		HookClaimsTotal:        2,
		HookClaimsValidated:    2,
		HookClaimsRejected:     0,
		CompactionFailed:       false,
		Outcome:                domain.ChapterOutcomeCommitted,
		CreatedAt:              time.Now(),
	}
	if err := s.SaveChapterMetrics(ctx, m1); err != nil {
		t.Fatalf("SaveChapterMetrics m1 failed: %v", err)
	}

	m2 := &domain.ChapterQualityMetrics{
		ProjectID:              proj.ID,
		ChapterIndex:           2,
		RunID:                  "run-2",
		ActualWords:            1500,
		WordsTarget:            2000,
		WordQuotaRatio:         0.75,
		UnderTarget:            true,
		Verdict:                "REJECTED",
		Score:                  60,
		RewriteLoops:           2,
		UnresolvedObservations: 1,
		HookClaimsTotal:        2,
		HookClaimsValidated:    1,
		HookClaimsRejected:     1,
		CompactionFailed:       false,
		Outcome:                domain.ChapterOutcomeNeedsHuman,
		CreatedAt:              time.Now(),
	}
	if err := s.SaveChapterMetrics(ctx, m2); err != nil {
		t.Fatalf("SaveChapterMetrics m2 failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+proj.ID+"/metrics", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Summary struct {
			Chapters                       int     `json:"chapters"`
			CommittedChapters              int     `json:"committed_chapters"`
			NeedsHumanChapters             int     `json:"needs_human_chapters"`
			TechnicalBlockedChapters       int     `json:"technical_blocked_chapters"`
			ProducedChapters               int     `json:"produced_chapters"`
			UnderTargetChapters            int     `json:"under_target_chapters"`
			RewriteChapters                int     `json:"rewrite_chapters"`
			UnclosedReconciliationChapters int     `json:"unclosed_reconciliation_chapters"`
			CompactionFailedChapters       int     `json:"compaction_failed_chapters"`
			WordPassRate                   float64 `json:"word_pass_rate"`
			AvgScore                       float64 `json:"avg_score"`
			RewriteRate                    float64 `json:"rewrite_rate"`
			HookClaimsTotal                int     `json:"hook_claims_total"`
			HookClaimsValidated            int     `json:"hook_claims_validated"`
			HookClaimsRejected             int     `json:"hook_claims_rejected"`
			HookPassRate                   float64 `json:"hook_pass_rate"`
		} `json:"summary"`
		Chapters []domain.ChapterQualityMetrics `json:"chapters"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	if resp.Summary.Chapters != 2 {
		t.Errorf("expected summary.chapters == 2, got %d", resp.Summary.Chapters)
	}
	if resp.Summary.CommittedChapters != 1 {
		t.Errorf("expected committed_chapters == 1, got %d", resp.Summary.CommittedChapters)
	}
	if resp.Summary.NeedsHumanChapters != 1 {
		t.Errorf("expected needs_human_chapters == 1, got %d", resp.Summary.NeedsHumanChapters)
	}
	if len(resp.Chapters) != 2 {
		t.Fatalf("expected len(chapters) == 2, got %d", len(resp.Chapters))
	}
	if resp.Chapters[0].Outcome != domain.ChapterOutcomeCommitted {
		t.Errorf("expected chapters[0].outcome == %q, got %q", domain.ChapterOutcomeCommitted, resp.Chapters[0].Outcome)
	}
	if resp.Chapters[1].Outcome != domain.ChapterOutcomeNeedsHuman {
		t.Errorf("expected chapters[1].outcome == %q, got %q", domain.ChapterOutcomeNeedsHuman, resp.Chapters[1].Outcome)
	}
}

func TestProjectMetrics_NotFound(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/projects/non-existent-proj/metrics", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown project, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProjectMetrics_EmptyChapters(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	proj := &domain.Project{
		ID:    "proj-metrics-empty",
		Title: "空指标项目",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+proj.ID+"/metrics", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal json failed: %v", err)
	}
	if string(raw["chapters"]) != "[]" {
		t.Errorf("expected chapters to be '[]', got %s", string(raw["chapters"]))
	}
}

func TestProjectMetrics_MethodNotAllowed(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	proj := &domain.Project{
		ID:    "proj-metrics-post",
		Title: "测试方法",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+proj.ID+"/metrics", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d: %s", w.Code, w.Body.String())
	}
}
