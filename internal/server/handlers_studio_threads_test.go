package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/server"
	"github.com/Jungley8/novel-studio/internal/store"
)

func setupThreadsTestServer(t *testing.T) (*server.Server, store.Store, *httptest.Server) {
	t.Helper()
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": "工坊助手的回复内容：情节推进顺利。",
					},
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     40,
				"completion_tokens": 20,
				"total_tokens":      60,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(func() {
		mockServer.Close()
	})

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "threads_test.db")
	cfgPath := filepath.Join(tmpDir, "cfg.json")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
	})

	cfg := config.DefaultConfig()
	cfg.APIBase = mockServer.URL
	cfg.APIKey = "sk-mock-key"
	llmClient := engine.NewHTTPLLMClient(mockServer.URL, "sk-mock-key")
	orch := engine.NewOrchestrator(llmClient)
	linter := engine.NewLinter(nil)

	srv, err := server.New(cfg, cfgPath, s, llmClient, orch, linter)
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}
	return srv, s, mockServer
}

func TestStudioChat_PersistFlow(t *testing.T) {
	srv, _, _ := setupThreadsTestServer(t)

	// 1. POST chat 无 thread_id → 200 且响应含 thread_id/message_id
	payload1 := map[string]string{
		"message": "你好，想讨论一个主角被退婚的反转设定",
		"mode":    "scene_brainstorm",
	}
	body1, _ := json.Marshal(payload1)
	req1 := httptest.NewRequest(http.MethodPost, "/api/studio/chat", bytes.NewReader(body1))
	w1 := httptest.NewRecorder()
	srv.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for first chat, got %d: %s", w1.Code, w1.Body.String())
	}

	var resp1 struct {
		Reply      string            `json:"reply"`
		Mode       string            `json:"mode"`
		ThreadID   string            `json:"thread_id"`
		MessageID  string            `json:"message_id"`
		TokensUsed engine.TokenUsage `json:"tokens_used"`
	}
	if err := json.Unmarshal(w1.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("unmarshal resp1 error: %v", err)
	}

	if resp1.ThreadID == "" {
		t.Fatalf("expected non-empty thread_id in response")
	}
	if resp1.MessageID == "" {
		t.Fatalf("expected non-empty message_id in response")
	}
	if resp1.Reply == "" {
		t.Fatalf("expected non-empty reply in response")
	}

	threadID := resp1.ThreadID

	// 2. 随后 GET threads 有 1 条
	reqList := httptest.NewRequest(http.MethodGet, "/api/studio/threads", nil)
	wList := httptest.NewRecorder()
	srv.ServeHTTP(wList, reqList)

	if wList.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET threads, got %d: %s", wList.Code, wList.Body.String())
	}

	var listResp struct {
		Threads []domain.StudioThread `json:"threads"`
	}
	if err := json.Unmarshal(wList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal listResp error: %v", err)
	}

	if len(listResp.Threads) != 1 {
		t.Fatalf("expected 1 thread in list, got %d", len(listResp.Threads))
	}
	th := listResp.Threads[0]
	if th.ID != threadID {
		t.Errorf("expected thread ID %s, got %s", threadID, th.ID)
	}
	if th.MessageCount != 2 {
		t.Errorf("expected message count 2, got %d", th.MessageCount)
	}
	if th.Title != "你好，想讨论一个主角被退婚的反转设定" {
		t.Errorf("expected title '你好，想讨论一个主角被退婚的反转设定', got %q", th.Title)
	}

	// 3. GET thread 有 2 条消息（user+assistant）
	reqGet := httptest.NewRequest(http.MethodGet, "/api/studio/threads/"+threadID, nil)
	wGet := httptest.NewRecorder()
	srv.ServeHTTP(wGet, reqGet)

	if wGet.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET thread detail, got %d: %s", wGet.Code, wGet.Body.String())
	}

	var detailResp struct {
		Thread   domain.StudioThread    `json:"thread"`
		Messages []domain.StudioMessage `json:"messages"`
	}
	if err := json.Unmarshal(wGet.Body.Bytes(), &detailResp); err != nil {
		t.Fatalf("unmarshal detailResp error: %v", err)
	}

	if detailResp.Thread.ID != threadID {
		t.Errorf("expected thread id %s, got %s", threadID, detailResp.Thread.ID)
	}
	if len(detailResp.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(detailResp.Messages))
	}
	if detailResp.Messages[0].Role != "user" || detailResp.Messages[0].Content != "你好，想讨论一个主角被退婚的反转设定" {
		t.Errorf("unexpected user message: %+v", detailResp.Messages[0])
	}
	if detailResp.Messages[1].Role != "assistant" || detailResp.Messages[1].ID != resp1.MessageID {
		t.Errorf("unexpected assistant message: %+v", detailResp.Messages[1])
	}

	// 4. 带 thread_id 再 POST → 同线程 4 条消息
	payload2 := map[string]string{
		"thread_id": threadID,
		"message":   "退婚后家族内部的反应如何设计？",
	}
	body2, _ := json.Marshal(payload2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/studio/chat", bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for second chat, got %d: %s", w2.Code, w2.Body.String())
	}

	var resp2 struct {
		ThreadID  string `json:"thread_id"`
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("unmarshal resp2 error: %v", err)
	}
	if resp2.ThreadID != threadID {
		t.Fatalf("expected same thread_id %s, got %s", threadID, resp2.ThreadID)
	}

	// 5. 校验同线程 4 条消息
	wGet2 := httptest.NewRecorder()
	srv.ServeHTTP(wGet2, reqGet)
	if wGet2.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET thread detail after second post, got %d: %s", wGet2.Code, wGet2.Body.String())
	}

	var detailResp2 struct {
		Thread   domain.StudioThread    `json:"thread"`
		Messages []domain.StudioMessage `json:"messages"`
	}
	if err := json.Unmarshal(wGet2.Body.Bytes(), &detailResp2); err != nil {
		t.Fatalf("unmarshal detailResp2 error: %v", err)
	}

	if len(detailResp2.Messages) != 4 {
		t.Fatalf("expected 4 messages in thread, got %d", len(detailResp2.Messages))
	}
	if detailResp2.Messages[2].Role != "user" || detailResp2.Messages[2].Content != "退婚后家族内部的反应如何设计？" {
		t.Errorf("unexpected 3rd message: %+v", detailResp2.Messages[2])
	}
	if detailResp2.Messages[3].Role != "assistant" || detailResp2.Messages[3].ID != resp2.MessageID {
		t.Errorf("unexpected 4th message: %+v", detailResp2.Messages[3])
	}

	// 6. DELETE → 200
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/studio/threads/"+threadID, nil)
	wDel := httptest.NewRecorder()
	srv.ServeHTTP(wDel, reqDel)

	if wDel.Code != http.StatusOK {
		t.Fatalf("expected 200 for DELETE thread, got %d: %s", wDel.Code, wDel.Body.String())
	}

	var delResp map[string]string
	_ = json.Unmarshal(wDel.Body.Bytes(), &delResp)
	if delResp["status"] != "deleted" {
		t.Errorf("expected status 'deleted', got %q", delResp["status"])
	}

	// 7. 再 GET → 404
	wGet3 := httptest.NewRecorder()
	srv.ServeHTTP(wGet3, reqGet)
	if wGet3.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for deleted thread, got %d: %s", wGet3.Code, wGet3.Body.String())
	}

	// 8. threads 空
	wList2 := httptest.NewRecorder()
	srv.ServeHTTP(wList2, reqList)
	if wList2.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET threads after delete, got %d: %s", wList2.Code, wList2.Body.String())
	}

	var listResp2 struct {
		Threads []domain.StudioThread `json:"threads"`
	}
	_ = json.Unmarshal(wList2.Body.Bytes(), &listResp2)
	if len(listResp2.Threads) != 0 {
		t.Fatalf("expected 0 threads after delete, got %d", len(listResp2.Threads))
	}
}

func TestStudioChat_UnknownThreadID(t *testing.T) {
	srv, _, _ := setupThreadsTestServer(t)

	payload := map[string]string{
		"thread_id": "non_existent_thread_xyz",
		"message":   "尝试在不存在的线程发消息",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/studio/chat", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown thread_id, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp["error"] != "thread not found" {
		t.Errorf("expected error 'thread not found', got %q", errResp["error"])
	}
}

func TestStudioThreads_Ordering(t *testing.T) {
	srv, _, _ := setupThreadsTestServer(t)

	// 线程 1
	body1, _ := json.Marshal(map[string]string{"message": "线程一第一句话"})
	req1 := httptest.NewRequest(http.MethodPost, "/api/studio/chat", bytes.NewReader(body1))
	w1 := httptest.NewRecorder()
	srv.ServeHTTP(w1, req1)
	var resp1 struct {
		ThreadID string `json:"thread_id"`
	}
	_ = json.Unmarshal(w1.Body.Bytes(), &resp1)

	time.Sleep(20 * time.Millisecond)

	// 线程 2
	body2, _ := json.Marshal(map[string]string{"message": "线程二第一句话"})
	req2 := httptest.NewRequest(http.MethodPost, "/api/studio/chat", bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	var resp2 struct {
		ThreadID string `json:"thread_id"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)

	// 此时线程 2 后发，排在前面
	reqList := httptest.NewRequest(http.MethodGet, "/api/studio/threads", nil)
	wList1 := httptest.NewRecorder()
	srv.ServeHTTP(wList1, reqList)
	var list1 struct {
		Threads []domain.StudioThread `json:"threads"`
	}
	_ = json.Unmarshal(wList1.Body.Bytes(), &list1)

	if len(list1.Threads) != 2 {
		t.Fatalf("expected 2 threads, got %d", len(list1.Threads))
	}
	if list1.Threads[0].ID != resp2.ThreadID || list1.Threads[1].ID != resp1.ThreadID {
		t.Fatalf("expected thread2 first, got [%s, %s]", list1.Threads[0].ID, list1.Threads[1].ID)
	}

	time.Sleep(20 * time.Millisecond)

	// 在线程 1 中发新消息
	body1Next, _ := json.Marshal(map[string]string{"thread_id": resp1.ThreadID, "message": "线程一第二句话"})
	req1Next := httptest.NewRequest(http.MethodPost, "/api/studio/chat", bytes.NewReader(body1Next))
	w1Next := httptest.NewRecorder()
	srv.ServeHTTP(w1Next, req1Next)

	// 重新获取列表：后聊的在前，线程 1 应排到第 0 位
	wList2 := httptest.NewRecorder()
	srv.ServeHTTP(wList2, reqList)
	var list2 struct {
		Threads []domain.StudioThread `json:"threads"`
	}
	_ = json.Unmarshal(wList2.Body.Bytes(), &list2)

	if len(list2.Threads) != 2 {
		t.Fatalf("expected 2 threads, got %d", len(list2.Threads))
	}
	if list2.Threads[0].ID != resp1.ThreadID {
		t.Fatalf("expected thread1 to be first after update, got %s (thread2 is %s)", list2.Threads[0].ID, list2.Threads[1].ID)
	}
}

func TestStudioThreads_ProjectContextOverride(t *testing.T) {
	srv, s, _ := setupThreadsTestServer(t)
	ctx := context.Background()

	pA := &domain.Project{ID: "proj-context-a", Title: "作品A", TargetPlatform: "起点仙侠"}
	pB := &domain.Project{ID: "proj-context-b", Title: "作品B", TargetPlatform: "番茄脑洞"}
	_ = s.SaveProject(ctx, pA)
	_ = s.SaveProject(ctx, pB)

	// 创建线程时绑定 proj-A
	body1, _ := json.Marshal(map[string]string{
		"project_id": pA.ID,
		"message":    "关于作品A的构思",
	})
	req1 := httptest.NewRequest(http.MethodPost, "/api/studio/chat", bytes.NewReader(body1))
	w1 := httptest.NewRecorder()
	srv.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 for chat, got %d: %s", w1.Code, w1.Body.String())
	}

	var resp1 struct {
		ThreadID  string `json:"thread_id"`
		ProjectID string `json:"project_id"`
	}
	_ = json.Unmarshal(w1.Body.Bytes(), &resp1)
	if resp1.ProjectID != pA.ID {
		t.Errorf("expected ProjectID %s, got %s", pA.ID, resp1.ProjectID)
	}

	// 再次发消息：请求中传入不一致的 project_id = pB.ID，以 thread.project_id 为准
	body2, _ := json.Marshal(map[string]string{
		"thread_id":  resp1.ThreadID,
		"project_id": pB.ID, // 不一致值
		"message":    "继续讨论设定",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/studio/chat", bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for chat with thread_id, got %d: %s", w2.Code, w2.Body.String())
	}

	var resp2 struct {
		ThreadID  string `json:"thread_id"`
		ProjectID string `json:"project_id"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	if resp2.ProjectID != pA.ID {
		t.Errorf("expected response ProjectID to follow thread (%s), got %s", pA.ID, resp2.ProjectID)
	}
}

func TestStudioThreadRoutes_MethodsAndValidation(t *testing.T) {
	srv, _, _ := setupThreadsTestServer(t)

	// 1. GET /api/studio/threads/ (空 ID) -> 400
	reqEmptyGet := httptest.NewRequest(http.MethodGet, "/api/studio/threads/", nil)
	wEmptyGet := httptest.NewRecorder()
	srv.ServeHTTP(wEmptyGet, reqEmptyGet)
	if wEmptyGet.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty id GET, got %d", wEmptyGet.Code)
	}

	// 2. DELETE /api/studio/threads/ (空 ID) -> 400
	reqEmptyDel := httptest.NewRequest(http.MethodDelete, "/api/studio/threads/", nil)
	wEmptyDel := httptest.NewRecorder()
	srv.ServeHTTP(wEmptyDel, reqEmptyDel)
	if wEmptyDel.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty id DELETE, got %d", wEmptyDel.Code)
	}

	// 3. DELETE /api/studio/threads/{unknown} -> 404
	reqUnknownDel := httptest.NewRequest(http.MethodDelete, "/api/studio/threads/unknown_123", nil)
	wUnknownDel := httptest.NewRecorder()
	srv.ServeHTTP(wUnknownDel, reqUnknownDel)
	if wUnknownDel.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown id DELETE, got %d", wUnknownDel.Code)
	}

	// 4. PUT /api/studio/threads -> 405
	reqPut := httptest.NewRequest(http.MethodPut, "/api/studio/threads", nil)
	wPut := httptest.NewRecorder()
	srv.ServeHTTP(wPut, reqPut)
	if wPut.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for PUT /api/studio/threads, got %d", wPut.Code)
	}

	// 5. POST /api/studio/threads/th_123 -> 405
	reqPostRoute := httptest.NewRequest(http.MethodPost, "/api/studio/threads/th_123", nil)
	wPostRoute := httptest.NewRecorder()
	srv.ServeHTTP(wPostRoute, reqPostRoute)
	if wPostRoute.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST /api/studio/threads/th_123, got %d", wPostRoute.Code)
	}
}
