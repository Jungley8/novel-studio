package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestStructureAudit_ListEmpty200(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	proj := &domain.Project{
		ID:             "proj-audit-empty",
		Title:          "空列表测试",
		TargetPlatform: "起点",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects/proj-audit-empty/structure-audit", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Findings []domain.StructureFinding `json:"findings"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if len(resp.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(resp.Findings))
	}
}

func TestStructureAudit_RunNoSummaries200(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	proj := &domain.Project{
		ID:             "proj-audit-nosum",
		Title:          "无摘要运行测试",
		TargetPlatform: "起点",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/projects/proj-audit-nosum/structure-audit", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Findings []domain.StructureFinding `json:"findings"`
		Note     string                    `json:"note"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if len(resp.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(resp.Findings))
	}
	if resp.Note != "暂无章摘要可审读" {
		t.Fatalf("expected note '暂无章摘要可审读', got %q", resp.Note)
	}
}

func TestStructureAudit_UpdateStatus(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	proj := &domain.Project{
		ID:             "proj-audit-status",
		Title:          "状态更新测试",
		TargetPlatform: "起点",
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	type findingSaver interface {
		SaveStructureFinding(ctx context.Context, f *domain.StructureFinding) error
	}
	saver, ok := s.(findingSaver)
	if !ok {
		t.Fatal("store does not implement findingSaver")
	}

	finding := &domain.StructureFinding{
		ID:           "find_status_1",
		ProjectID:    "proj-audit-status",
		Type:         domain.StructureFindingTypePlotHole,
		Severity:     domain.StructureFindingSeverityHigh,
		ChapterStart: 1,
		ChapterEnd:   2,
		Summary:      "漏洞摘要",
		Evidence:     "证据引用",
		Suggestion:   "修复建议",
		Status:       domain.StructureFindingStatusOpen,
		CreatedAt:    time.Now(),
	}
	if err := saver.SaveStructureFinding(ctx, finding); err != nil {
		t.Fatalf("save finding failed: %v", err)
	}

	t.Run("valid status update to RESOLVED", func(t *testing.T) {
		body := `{"finding_id": "find_status_1", "status": "RESOLVED"}`
		req := httptest.NewRequest(http.MethodPost, "/api/projects/proj-audit-status/structure-audit/status", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("invalid status rejected with 400", func(t *testing.T) {
		body := `{"finding_id": "find_status_1", "status": "INVALID_STATUS"}`
		req := httptest.NewRequest(http.MethodPost, "/api/projects/proj-audit-status/structure-audit/status", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid status, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("empty finding_id rejected with 400", func(t *testing.T) {
		body := `{"finding_id": "", "status": "OPEN"}`
		req := httptest.NewRequest(http.MethodPost, "/api/projects/proj-audit-status/structure-audit/status", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for empty finding_id, got %d: %s", w.Code, w.Body.String())
		}
	})
}
