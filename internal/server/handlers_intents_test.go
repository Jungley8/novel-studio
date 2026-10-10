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

func TestIntentsAPI_FullLifecycle(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	projectID := "proj-intents-api"
	p := &domain.Project{
		ID:             projectID,
		Title:          "意图卡API测试",
		TargetPlatform: "番茄脑洞",
		WorldRules:     "玄幻世界",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 1. PUT /api/projects/{id}/intents/1 (Default status: DRAFT)
	putDraftBody := domain.ChapterIntent{
		Conflict:      "主角遭遇家族考核刁难",
		POV:           "林动",
		EmotionCurve:  []string{"压抑", "反转", "扬眉吐气"},
		TensionTarget: 7,
		VolumeIndex:   1,
	}
	raw, _ := json.Marshal(putDraftBody)
	req := httptest.NewRequest(http.MethodPut, "/api/projects/"+projectID+"/intents/1", bytes.NewReader(raw))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT intent draft: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var created domain.ChapterIntent
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created intent: %v", err)
	}
	if created.ProjectID != projectID {
		t.Errorf("expected ProjectID %q, got %q", projectID, created.ProjectID)
	}
	if created.ChapterIndex != 1 {
		t.Errorf("expected ChapterIndex 1, got %d", created.ChapterIndex)
	}
	if created.Status != domain.IntentStatusDraft {
		t.Errorf("expected default Status DRAFT, got %s", created.Status)
	}

	// 2. GET /api/projects/{id}/intents/1
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/intents/1", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET intent 1: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got domain.ChapterIntent
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal got intent: %v", err)
	}
	if got.Conflict != putDraftBody.Conflict || got.TensionTarget != 7 {
		t.Errorf("unexpected intent content: %+v", got)
	}

	// 3. PUT /api/projects/{id}/intents/1 (Transition to LOCKED)
	putLockBody := putDraftBody
	putLockBody.Status = domain.IntentStatusLocked
	raw, _ = json.Marshal(putLockBody)
	req = httptest.NewRequest(http.MethodPut, "/api/projects/"+projectID+"/intents/1", bytes.NewReader(raw))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT intent locked: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var locked domain.ChapterIntent
	if err := json.Unmarshal(w.Body.Bytes(), &locked); err != nil {
		t.Fatalf("unmarshal locked intent: %v", err)
	}
	if locked.Status != domain.IntentStatusLocked {
		t.Errorf("expected Status LOCKED, got %s", locked.Status)
	}

	// 4. Create chapter 2 intent (Volume 1)
	putCh2Body := domain.ChapterIntent{
		Conflict:      "主角进入后山古洞探宝",
		TensionTarget: 6,
		VolumeIndex:   1,
	}
	raw, _ = json.Marshal(putCh2Body)
	req = httptest.NewRequest(http.MethodPut, "/api/projects/"+projectID+"/intents/2", bytes.NewReader(raw))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT intent 2: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// 5. GET /api/projects/{id}/intents (List all)
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/intents", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("LIST intents: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var listAll []*domain.ChapterIntent
	if err := json.Unmarshal(w.Body.Bytes(), &listAll); err != nil {
		t.Fatalf("unmarshal list all: %v", err)
	}
	if len(listAll) != 2 {
		t.Fatalf("expected 2 intents in all list, got %d", len(listAll))
	}

	// 6. GET /api/projects/{id}/intents?volume=1
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/intents?volume=1", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("LIST intents vol 1: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var listVol1 []*domain.ChapterIntent
	if err := json.Unmarshal(w.Body.Bytes(), &listVol1); err != nil {
		t.Fatalf("unmarshal list vol 1: %v", err)
	}
	if len(listVol1) != 2 {
		t.Fatalf("expected 2 intents in vol 1 list, got %d", len(listVol1))
	}

	// 7. GET /api/projects/{id}/intents?volume=2 (Empty)
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/intents?volume=2", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("LIST intents vol 2: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var listVol2 []*domain.ChapterIntent
	if err := json.Unmarshal(w.Body.Bytes(), &listVol2); err != nil {
		t.Fatalf("unmarshal list vol 2: %v", err)
	}
	if len(listVol2) != 0 {
		t.Fatalf("expected 0 intents in vol 2 list, got %d", len(listVol2))
	}

	// 8. DELETE /api/projects/{id}/intents/1
	req = httptest.NewRequest(http.MethodDelete, "/api/projects/"+projectID+"/intents/1", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("DELETE intent 1: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify GET 1 is now 404
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/intents/1", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET deleted intent 1: expected 404, got %d", w.Code)
	}

	// 9. DELETE /api/projects/{id}/intents/1 (Idempotent)
	req = httptest.NewRequest(http.MethodDelete, "/api/projects/"+projectID+"/intents/1", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("idempotent DELETE intent 1: expected 200, got %d", w.Code)
	}
}

func TestIntentsAPI_ValidationErrors(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	projectID := "proj-intents-val"
	p := &domain.Project{
		ID:             projectID,
		Title:          "校验错误测试",
		TargetPlatform: "番茄脑洞",
		WorldRules:     "现代都市",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 1. tension_target = 11 -> 400
	badTensionBody := domain.ChapterIntent{
		Conflict:      "正常冲突内容",
		TensionTarget: 11,
		Status:        domain.IntentStatusDraft,
	}
	raw, _ := json.Marshal(badTensionBody)
	req := httptest.NewRequest(http.MethodPut, "/api/projects/"+projectID+"/intents/1", bytes.NewReader(raw))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("tension_target=11: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Empty conflict -> 400
	emptyConflictBody := domain.ChapterIntent{
		Conflict:      "   ",
		TensionTarget: 5,
		Status:        domain.IntentStatusDraft,
	}
	raw, _ = json.Marshal(emptyConflictBody)
	req = httptest.NewRequest(http.MethodPut, "/api/projects/"+projectID+"/intents/1", bytes.NewReader(raw))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("empty conflict: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	// 3. User submits LOCKED with invalid intent -> fail-closed, returns 400
	badLockedBody := domain.ChapterIntent{
		Conflict:      "",
		TensionTarget: 8,
		Status:        domain.IntentStatusLocked,
	}
	raw, _ = json.Marshal(badLockedBody)
	req = httptest.NewRequest(http.MethodPut, "/api/projects/"+projectID+"/intents/1", bytes.NewReader(raw))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("locked invalid intent: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Invalid chapter index in path -> 400
	req = httptest.NewRequest(http.MethodPut, "/api/projects/"+projectID+"/intents/0", bytes.NewReader(raw))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("chapter 0: expected 400, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/projects/"+projectID+"/intents/abc", bytes.NewReader(raw))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("chapter abc: expected 400, got %d", w.Code)
	}

	// 5. Invalid volume param in query -> 400
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/intents?volume=notanumber", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid volume param: expected 400, got %d", w.Code)
	}
}

func TestIntentsAPI_FrameworkVolumeChangeLinkage(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	projectID := "proj-framework-linkage"
	p := &domain.Project{
		ID:             projectID,
		Title:          "卷联动测试项目",
		TargetPlatform: "番茄脑洞",
		WorldRules:     "修仙世界",
		Framework: &domain.ProjectFramework{
			ThemePremise: "大道争锋",
			VolumeArcs: []domain.VolumeArc{
				{
					VolumeIndex:  1,
					Title:        "第一卷：破茧",
					StartChapter: 1,
					EndChapter:   10,
					Status:       domain.VolumeStatusDraft,
				},
				{
					VolumeIndex:  2,
					Title:        "第二卷：化蝶",
					StartChapter: 11,
					EndChapter:   20,
					Status:       domain.VolumeStatusDraft,
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// Seed Volume 1 Chapter 1 LOCKED intent
	intentVol1 := &domain.ChapterIntent{
		ProjectID:     projectID,
		ChapterIndex:  1,
		VolumeIndex:   1,
		Conflict:      "第一卷第一章关键锁定意图",
		TensionTarget: 7,
		Status:        domain.IntentStatusLocked,
	}
	if err := s.SaveChapterIntent(ctx, intentVol1); err != nil {
		t.Fatalf("SaveChapterIntent 1 failed: %v", err)
	}

	// Seed Volume 2 Chapter 11 LOCKED intent
	intentVol2 := &domain.ChapterIntent{
		ProjectID:     projectID,
		ChapterIndex:  11,
		VolumeIndex:   2,
		Conflict:      "第二卷第十一章关键锁定意图",
		TensionTarget: 8,
		Status:        domain.IntentStatusLocked,
	}
	if err := s.SaveChapterIntent(ctx, intentVol2); err != nil {
		t.Fatalf("SaveChapterIntent 11 failed: %v", err)
	}

	// PUT /api/projects/{id}/framework modifying Volume 1 boundary (1..15 instead of 1..10)
	// Volume 2 boundary remains unchanged (11..20)
	updatedFw := domain.ProjectFramework{
		ThemePremise: "大道争锋-修订版",
		VolumeArcs: []domain.VolumeArc{
			{
				VolumeIndex:  1,
				Title:        "第一卷：破茧（扩充版）",
				StartChapter: 1,
				EndChapter:   15, // Changed boundary!
				Status:       domain.VolumeStatusDraft,
			},
			{
				VolumeIndex:  2,
				Title:        "第二卷：化蝶",
				StartChapter: 11,
				EndChapter:   20, // Unchanged boundary!
				Status:       domain.VolumeStatusDraft,
			},
		},
	}
	fwBytes, _ := json.Marshal(updatedFw)
	req := httptest.NewRequest(http.MethodPut, "/api/projects/"+projectID+"/framework", bytes.NewReader(fwBytes))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT framework: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify Chapter 1 (Volume 1) LOCKED intent became STALE
	got1, err := s.GetChapterIntent(ctx, projectID, 1)
	if err != nil || got1 == nil {
		t.Fatalf("GetChapterIntent 1 failed: %v", err)
	}
	if got1.Status != domain.IntentStatusStale {
		t.Errorf("expected Chapter 1 status STALE after Vol 1 boundary changed, got %s", got1.Status)
	}

	// Verify Chapter 11 (Volume 2) LOCKED intent remained LOCKED (untouched)
	got11, err := s.GetChapterIntent(ctx, projectID, 11)
	if err != nil || got11 == nil {
		t.Fatalf("GetChapterIntent 11 failed: %v", err)
	}
	if got11.Status != domain.IntentStatusLocked {
		t.Errorf("expected Chapter 11 status LOCKED because Vol 2 boundary was untouched, got %s", got11.Status)
	}
}
