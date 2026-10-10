package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestActivityAPI_ListAndDetail(t *testing.T) {
	srv, s, _ := setupTestServer(t)

	// Seed via HTTP so the ui_manual source attribution path is exercised.
	projPayload := domain.Project{
		ID:             "proj-act",
		Title:          "时间线验证录",
		TargetPlatform: "番茄脑洞",
		WorldRules:     "灵气复苏",
	}
	body, _ := json.Marshal(projPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create project: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Commit a chapter with a large body to exercise the gzip path end to end.
	bigContent := strings.Repeat("城门大开，旌旗蔽空。", 150)
	_ = s // store handle kept for parity; writes below go through HTTP
	commitPayload := domain.Chapter{
		ChapterIndex: 1,
		Title:        "开城",
		CoreConflict: "守将倒戈",
		Content:      bigContent,
	}
	cbody, _ := json.Marshal(commitPayload)
	creq := httptest.NewRequest(http.MethodPost, "/api/projects/proj-act/chapters", strings.NewReader(string(cbody)))
	cw := httptest.NewRecorder()
	srv.ServeHTTP(cw, creq)
	if cw.Code != http.StatusCreated {
		t.Fatalf("commit chapter: expected 201, got %d: %s", cw.Code, cw.Body.String())
	}

	// 1. List timeline (checkpoints hidden by default).
	lreq := httptest.NewRequest(http.MethodGet, "/api/projects/proj-act/activity", nil)
	lw := httptest.NewRecorder()
	srv.ServeHTTP(lw, lreq)
	if lw.Code != http.StatusOK {
		t.Fatalf("list activity: expected 200, got %d: %s", lw.Code, lw.Body.String())
	}
	var listResp struct {
		Events     []*activity.ActivityEvent `json:"events"`
		NextCursor string                    `json:"next_cursor"`
	}
	if err := json.Unmarshal(lw.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listResp.Events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(listResp.Events))
	}
	if listResp.Events[0].EventType != activity.EventChapterCommitted {
		t.Fatalf("expected newest chapter.committed, got %s", listResp.Events[0].EventType)
	}
	if listResp.Events[0].Source != activity.SourceUIManual {
		t.Fatalf("expected source ui_manual, got %s", listResp.Events[0].Source)
	}
	if listResp.Events[0].After != "" {
		t.Fatalf("list view must not include full snapshots")
	}
	if listResp.NextCursor != "" {
		t.Fatalf("expected empty cursor when exhausted, got %q", listResp.NextCursor)
	}

	// 2. Entity filter.
	freq := httptest.NewRequest(http.MethodGet, "/api/projects/proj-act/activity?entity_type=chapter", nil)
	fw := httptest.NewRecorder()
	srv.ServeHTTP(fw, freq)
	var fResp struct {
		Events []*activity.ActivityEvent `json:"events"`
	}
	_ = json.Unmarshal(fw.Body.Bytes(), &fResp)
	if len(fResp.Events) != 1 || fResp.Events[0].EventType != activity.EventChapterCommitted {
		t.Fatalf("entity filter broken: %+v", fResp.Events)
	}

	// 3. Detail decodes the full gzipped chapter text.
	detailURL := fmt.Sprintf("/api/projects/proj-act/activity/%s", listResp.Events[0].ID)
	dreq := httptest.NewRequest(http.MethodGet, detailURL, nil)
	dw := httptest.NewRecorder()
	srv.ServeHTTP(dw, dreq)
	if dw.Code != http.StatusOK {
		t.Fatalf("detail: expected 200, got %d: %s", dw.Code, dw.Body.String())
	}
	var detail activity.ActivityEvent
	if err := json.Unmarshal(dw.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.Codec != activity.CodecGzip {
		t.Fatalf("expected gzip codec, got %q", detail.Codec)
	}
	if !strings.Contains(detail.After, bigContent[:60]) {
		t.Fatalf("detail must carry decoded full content")
	}

	// 4. Unknown event id is a clean 404, not a 500.
	nreq := httptest.NewRequest(http.MethodGet, "/api/projects/proj-act/activity/evt_missing", nil)
	nw := httptest.NewRecorder()
	srv.ServeHTTP(nw, nreq)
	if nw.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing event, got %d", nw.Code)
	}
}
