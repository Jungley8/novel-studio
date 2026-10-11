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

func seedTestAutomationProject(t *testing.T, srv http.Handler, storeObj any, id, title string) {
	t.Helper()
	type saver interface {
		SaveProject(ctx context.Context, p *domain.Project) error
	}
	s, ok := storeObj.(saver)
	if !ok {
		t.Fatalf("store does not implement SaveProject")
	}
	p := &domain.Project{
		ID:             id,
		Title:          title,
		TargetPlatform: "番茄脑洞",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.SaveProject(context.Background(), p); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}
}

func TestAutomation_GET_DefaultRow(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	seedTestAutomationProject(t, srv, s, "proj-1", "全职剑修")

	req := httptest.NewRequest(http.MethodGet, "/api/studio/automation", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Items []domain.ProjectAutomation `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}

	item := resp.Items[0]
	if item.ProjectID != "proj-1" || item.ProjectTitle != "全职剑修" {
		t.Errorf("unexpected project id/title: %s / %s", item.ProjectID, item.ProjectTitle)
	}
	if item.Enabled {
		t.Errorf("expected enabled false, got true")
	}
	if item.Times == nil || len(item.Times) != 0 {
		t.Errorf("expected empty non-nil times, got %v", item.Times)
	}
	if item.QuotaKind != domain.QuotaKindChapters {
		t.Errorf("expected quota kind chapters, got %s", item.QuotaKind)
	}
	if item.QuotaCount != 5 {
		t.Errorf("expected quota count 5, got %d", item.QuotaCount)
	}
	if item.Paused {
		t.Errorf("expected paused false, got true")
	}
}

func TestAutomation_PUT(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	seedTestAutomationProject(t, srv, s, "proj-1", "全职剑修")

	// 1. Success: Enabled + valid times -> 200
	putBody := `{"enabled": true, "times": ["08:00", "20:00"], "quota_kind": "chapters", "quota_count": 10}`
	req := httptest.NewRequest(http.MethodPut, "/api/studio/automation/proj-1", bytes.NewBufferString(putBody))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var saved domain.ProjectAutomation
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if !saved.Enabled || len(saved.Times) != 2 || saved.QuotaCount != 10 {
		t.Errorf("unexpected saved automation: %+v", saved)
	}

	// GET reflects the change
	getReq := httptest.NewRequest(http.MethodGet, "/api/studio/automation", nil)
	getW := httptest.NewRecorder()
	srv.ServeHTTP(getW, getReq)
	var getResp struct {
		Items []domain.ProjectAutomation `json:"items"`
	}
	_ = json.Unmarshal(getW.Body.Bytes(), &getResp)
	if len(getResp.Items) != 1 || !getResp.Items[0].Enabled || len(getResp.Items[0].Times) != 2 {
		t.Errorf("GET did not reflect PUT: %+v", getResp.Items)
	}

	// 2. Invalid daily time format -> 400
	badTimeBody := `{"enabled": true, "times": ["25:00"], "quota_kind": "chapters", "quota_count": 5}`
	badTimeReq := httptest.NewRequest(http.MethodPut, "/api/studio/automation/proj-1", bytes.NewBufferString(badTimeBody))
	badTimeW := httptest.NewRecorder()
	srv.ServeHTTP(badTimeW, badTimeReq)
	if badTimeW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid time, got %d: %s", badTimeW.Code, badTimeW.Body.String())
	}

	// 3. Enabled + empty times -> 400
	emptyTimesBody := `{"enabled": true, "times": [], "quota_kind": "chapters", "quota_count": 5}`
	emptyTimesReq := httptest.NewRequest(http.MethodPut, "/api/studio/automation/proj-1", bytes.NewBufferString(emptyTimesBody))
	emptyTimesW := httptest.NewRecorder()
	srv.ServeHTTP(emptyTimesW, emptyTimesReq)
	if emptyTimesW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for enabled with empty times, got %d: %s", emptyTimesW.Code, emptyTimesW.Body.String())
	}

	// 4. Quota count 0 -> 400
	zeroQuotaBody := `{"enabled": false, "times": [], "quota_kind": "chapters", "quota_count": 0}`
	zeroQuotaReq := httptest.NewRequest(http.MethodPut, "/api/studio/automation/proj-1", bytes.NewBufferString(zeroQuotaBody))
	zeroQuotaW := httptest.NewRecorder()
	srv.ServeHTTP(zeroQuotaW, zeroQuotaReq)
	if zeroQuotaW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for quota count 0, got %d: %s", zeroQuotaW.Code, zeroQuotaW.Body.String())
	}

	// 5. Unknown project -> 404
	ghostBody := `{"enabled": false, "times": [], "quota_kind": "chapters", "quota_count": 5}`
	ghostReq := httptest.NewRequest(http.MethodPut, "/api/studio/automation/ghost-proj", bytes.NewBufferString(ghostBody))
	ghostW := httptest.NewRecorder()
	srv.ServeHTTP(ghostW, ghostReq)
	if ghostW.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown project, got %d: %s", ghostW.Code, ghostW.Body.String())
	}

	// 6. PUT does not alter paused state
	ctx := context.Background()
	if err := s.SetAutomationPaused(ctx, "proj-1", true, "quality_gate_failed"); err != nil {
		t.Fatalf("SetAutomationPaused failed: %v", err)
	}
	putAgainBody := `{"enabled": false, "times": [], "quota_kind": "chapters", "quota_count": 5}`
	putAgainReq := httptest.NewRequest(http.MethodPut, "/api/studio/automation/proj-1", bytes.NewBufferString(putAgainBody))
	putAgainW := httptest.NewRecorder()
	srv.ServeHTTP(putAgainW, putAgainReq)
	if putAgainW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", putAgainW.Code, putAgainW.Body.String())
	}
	var afterPut domain.ProjectAutomation
	_ = json.Unmarshal(putAgainW.Body.Bytes(), &afterPut)
	if !afterPut.Paused || afterPut.PausedReason != "quality_gate_failed" {
		t.Errorf("PUT unexpectedly cleared paused state: %+v", afterPut)
	}
}

func TestAutomation_Resume(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	seedTestAutomationProject(t, srv, s, "proj-1", "全职剑修")
	ctx := context.Background()

	// Initial save
	_ = s.SaveProjectAutomation(ctx, &domain.ProjectAutomation{
		ProjectID:  "proj-1",
		Enabled:    true,
		Times:      []string{"10:00"},
		QuotaKind:  domain.QuotaKindChapters,
		QuotaCount: 5,
	})

	// Set paused
	if err := s.SetAutomationPaused(ctx, "proj-1", true, "halted"); err != nil {
		t.Fatalf("SetAutomationPaused failed: %v", err)
	}

	// POST /api/studio/automation/proj-1/resume -> 200 {"ok":true}
	resumeReq := httptest.NewRequest(http.MethodPost, "/api/studio/automation/proj-1/resume", nil)
	resumeW := httptest.NewRecorder()
	srv.ServeHTTP(resumeW, resumeReq)

	if resumeW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", resumeW.Code, resumeW.Body.String())
	}

	var resp map[string]bool
	if err := json.Unmarshal(resumeW.Body.Bytes(), &resp); err != nil || !resp["ok"] {
		t.Errorf("unexpected resume response: %s", resumeW.Body.String())
	}

	// GET reflects paused=false
	got, err := s.GetProjectAutomation(ctx, "proj-1")
	if err != nil {
		t.Fatalf("GetProjectAutomation failed: %v", err)
	}
	if got.Paused {
		t.Errorf("expected paused false after resume, got true")
	}

	// Non-existent automation -> 404
	ghostReq := httptest.NewRequest(http.MethodPost, "/api/studio/automation/ghost-proj/resume", nil)
	ghostW := httptest.NewRecorder()
	srv.ServeHTTP(ghostW, ghostReq)
	if ghostW.Code != http.StatusNotFound {
		t.Errorf("expected 404 for ghost-proj resume, got %d: %s", ghostW.Code, ghostW.Body.String())
	}
}

func TestQueue_Lifecycle(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	// 1. POST -> 201 pending with default auto_lock=true
	postBody1 := `{"title": "剑开天门", "concept": "传统修仙", "platform": "起点", "volumes": 3}`
	postReq1 := httptest.NewRequest(http.MethodPost, "/api/studio/queue", bytes.NewBufferString(postBody1))
	postW1 := httptest.NewRecorder()
	srv.ServeHTTP(postW1, postReq1)

	if postW1.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", postW1.Code, postW1.Body.String())
	}

	var item1 domain.BookQueueItem
	if err := json.Unmarshal(postW1.Body.Bytes(), &item1); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if item1.ID == "" || item1.Title != "剑开天门" || item1.Status != domain.QueueStatusPending || !item1.AutoLock {
		t.Errorf("unexpected item1 attributes: %+v", item1)
	}

	// 2. POST second item with explicit auto_lock=false
	postBody2 := `{"title": "星际拓荒", "concept": "硬科幻", "platform": "番茄", "volumes": 1, "auto_lock": false}`
	postReq2 := httptest.NewRequest(http.MethodPost, "/api/studio/queue", bytes.NewBufferString(postBody2))
	postW2 := httptest.NewRecorder()
	srv.ServeHTTP(postW2, postReq2)

	if postW2.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", postW2.Code, postW2.Body.String())
	}

	var item2 domain.BookQueueItem
	if err := json.Unmarshal(postW2.Body.Bytes(), &item2); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if item2.AutoLock {
		t.Errorf("expected auto_lock false, got true")
	}

	// 3. GET /api/studio/queue -> FIFO order
	getReq := httptest.NewRequest(http.MethodGet, "/api/studio/queue", nil)
	getW := httptest.NewRecorder()
	srv.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", getW.Code, getW.Body.String())
	}

	var listResp struct {
		Items []domain.BookQueueItem `json:"items"`
	}
	if err := json.Unmarshal(getW.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(listResp.Items) != 2 || listResp.Items[0].ID != item1.ID || listResp.Items[1].ID != item2.ID {
		t.Errorf("expected FIFO items [%s, %s], got: %+v", item1.ID, item2.ID, listResp.Items)
	}

	// 4. PUT pending -> 200 (auto_lock nil keeps existing value true)
	putBody := `{"title": "剑开天门(精修)", "concept": "修仙升级", "platform": "起点", "volumes": 5}`
	putReq := httptest.NewRequest(http.MethodPut, "/api/studio/queue/"+item1.ID, bytes.NewBufferString(putBody))
	putW := httptest.NewRecorder()
	srv.ServeHTTP(putW, putReq)

	if putW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", putW.Code, putW.Body.String())
	}
	var updatedItem1 domain.BookQueueItem
	_ = json.Unmarshal(putW.Body.Bytes(), &updatedItem1)
	if updatedItem1.Title != "剑开天门(精修)" || updatedItem1.Volumes != 5 || !updatedItem1.AutoLock {
		t.Errorf("unexpected updated item1: %+v", updatedItem1)
	}

	// 5. Set item1 status to running in store
	updatedItem1.Status = domain.QueueStatusRunning
	if err := s.SaveBookQueueItem(ctx, &updatedItem1); err != nil {
		t.Fatalf("SaveBookQueueItem running failed: %v", err)
	}

	// PUT on running -> 409
	putRunningReq := httptest.NewRequest(http.MethodPut, "/api/studio/queue/"+item1.ID, bytes.NewBufferString(putBody))
	putRunningW := httptest.NewRecorder()
	srv.ServeHTTP(putRunningW, putRunningReq)
	if putRunningW.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for PUT on running, got %d: %s", putRunningW.Code, putRunningW.Body.String())
	}

	// DELETE on running -> 409
	delRunningReq := httptest.NewRequest(http.MethodDelete, "/api/studio/queue/"+item1.ID, nil)
	delRunningW := httptest.NewRecorder()
	srv.ServeHTTP(delRunningW, delRunningReq)
	if delRunningW.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for DELETE on running, got %d: %s", delRunningW.Code, delRunningW.Body.String())
	}

	// 6. DELETE on pending (item2) -> 204
	delPendingReq := httptest.NewRequest(http.MethodDelete, "/api/studio/queue/"+item2.ID, nil)
	delPendingW := httptest.NewRecorder()
	srv.ServeHTTP(delPendingW, delPendingReq)
	if delPendingW.Code != http.StatusNoContent {
		t.Errorf("expected 204 No Content, got %d: %s", delPendingW.Code, delPendingW.Body.String())
	}

	// 7. DELETE unknown -> 404
	delGhostReq := httptest.NewRequest(http.MethodDelete, "/api/studio/queue/ghost-queue-id", nil)
	delGhostW := httptest.NewRecorder()
	srv.ServeHTTP(delGhostW, delGhostReq)
	if delGhostW.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown delete, got %d: %s", delGhostW.Code, delGhostW.Body.String())
	}
}

func TestConfig_QueueTimes_Endpoint(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	// 1. POST /api/config with valid queue_times -> 200
	postValid := `{"queue_times": ["02:00"]}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewBufferString(postValid))
	w1 := httptest.NewRecorder()
	srv.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w1.Code, w1.Body.String())
	}

	// GET reflects queue_times
	getReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	getW := httptest.NewRecorder()
	srv.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on GET, got %d: %s", getW.Code, getW.Body.String())
	}

	var getCfg struct {
		QueueTimes []string `json:"queue_times"`
	}
	if err := json.Unmarshal(getW.Body.Bytes(), &getCfg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(getCfg.QueueTimes) != 1 || getCfg.QueueTimes[0] != "02:00" {
		t.Errorf("expected ['02:00'], got %v", getCfg.QueueTimes)
	}

	// 2. POST /api/config with invalid queue_times -> 400
	postInvalid := `{"queue_times": ["25:00"]}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewBufferString(postInvalid))
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid queue_times, got %d: %s", w2.Code, w2.Body.String())
	}
}
