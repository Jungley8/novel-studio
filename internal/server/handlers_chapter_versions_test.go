package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

func TestServer_ChapterVersions(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	p := &domain.Project{
		ID:    "proj-ver-srv",
		Title: "版本接口测试",
	}
	if err := s.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}
	content := "版本接口测试正文内容。"
	ch := &domain.Chapter{
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      content,
	}
	if _, err := s.CommitChapter(ctx, p.ID, ch); err != nil {
		t.Fatalf("CommitChapter failed: %v", err)
	}

	sq, ok := s.(*store.SQLiteStore)
	if !ok {
		t.Fatalf("test store does not support chapter versions")
	}
	vers, err := sq.ListChapterVersions(ctx, p.ID, 1)
	if err != nil || len(vers) == 0 {
		t.Fatalf("ListChapterVersions failed: vers=%v, err=%v", vers, err)
	}
	verID := vers[0].ID

	// 1. GET /api/projects/{id}/chapters/1/versions → 200 且数组长度 ≥1
	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+p.ID+"/chapters/1/versions", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for versions list, got %d: %s", w.Code, w.Body.String())
	}
	var list []domain.ChapterVersionMeta
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal versions list failed: %v", err)
	}
	if len(list) < 1 {
		t.Fatalf("expected at least 1 version, got %d", len(list))
	}

	// 2. GET .../versions/{id} → 200 且 content 匹配
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+p.ID+"/chapters/1/versions/"+verID, nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for single version, got %d: %s", w.Code, w.Body.String())
	}
	var single domain.ChapterVersion
	if err := json.Unmarshal(w.Body.Bytes(), &single); err != nil {
		t.Fatalf("unmarshal single version failed: %v", err)
	}
	if single.Content != content {
		t.Errorf("expected content %q, got %q", content, single.Content)
	}

	// 3. GET .../versions/{不存在} → 404
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+p.ID+"/chapters/1/versions/ver_not_exist_xyz", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing version, got %d: %s", w.Code, w.Body.String())
	}

	// 4. GET 章号非法（如 /chapters/abc/versions）→ 400
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+p.ID+"/chapters/abc/versions", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid chapter index, got %d: %s", w.Code, w.Body.String())
	}
}
