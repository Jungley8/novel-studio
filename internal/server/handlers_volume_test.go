package server_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/server"
	"github.com/Jungley8/novel-studio/internal/store"
)

func setupTestServerForVolume(t *testing.T) (*server.Server, store.Store, *httptest.Server) {
	mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		var systemPrompt, userPrompt string
		for _, m := range req.Messages {
			if m.Role == "system" {
				systemPrompt = m.Content
			} else if m.Role == "user" {
				userPrompt = m.Content
			}
		}

		content := "夜色渐深，冷风呼啸。林凡拔出青玉长剑，一剑断去黑衣人退路。"
		if strings.Contains(systemPrompt, "分卷分节架构师") {
			from, to := 1, 1
			if m := regexp.MustCompile(`章节范围：第 (\d+) 章 ~ 第 (\d+) 章`).FindStringSubmatch(userPrompt); len(m) == 3 {
				from, _ = strconv.Atoi(m[1])
				to, _ = strconv.Atoi(m[2])
			}
			content = fmt.Sprintf(`{"sections":[{"title":"主线推进","from_chapter":%d,"to_chapter":%d,"goal":"推进本卷主线","key_events":["关键推进"]}]}`, from, to)
		} else if strings.Contains(systemPrompt, "精算师") || strings.Contains(systemPrompt, "结算") || strings.Contains(systemPrompt, "Settle") {
			content = `{"inventory_delta":"","power_delta":"","character_mutations":[],"evidence":[]}`
		} else if strings.Contains(systemPrompt, "分庭总审编") || strings.Contains(systemPrompt, "总审编") || strings.Contains(systemPrompt, "总编审") || strings.Contains(systemPrompt, "QualityGate") {
			content = `{"verdict":"ACCEPTED","score":90,"issues":[],"suggestions":"通过"}`
		} else if strings.Contains(systemPrompt, "总设计师") || strings.Contains(systemPrompt, "剧情节拍") || strings.Contains(systemPrompt, "Beats") {
			content = `{
				"beats": [
					{"phase": "蓄力", "tension": 6, "action": "对峙", "expectation_broken": "反派失算"},
					{"phase": "破局", "tension": 9, "action": "逆转", "expectation_broken": "全面击溃"}
				],
				"state_mutation": {"inventory_delta": "+战利品", "power_delta": "突破"}
			}`
		}
		_ = userPrompt

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": content,
					},
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     80,
				"completion_tokens": 160,
				"total_tokens":      240,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "vol_test.db")
	cfgPath := filepath.Join(tmpDir, "cfg.json")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.APIBase = mockLLM.URL
	cfg.APIKey = "sk-mock-key"

	llmClient := engine.NewHTTPLLMClient(mockLLM.URL, "sk-mock-key")
	orch := engine.NewOrchestrator(llmClient)
	linter := engine.NewLinter(nil)

	srv, err := server.New(cfg, cfgPath, s, llmClient, orch, linter)
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	return srv, s, mockLLM
}

func seedTestProject(t *testing.T, s store.Store, projectID string, startCh, endCh int) {
	proj := &domain.Project{
		ID:             projectID,
		Title:          "万域之主",
		TargetPlatform: "番茄脑洞",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{
					VolumeIndex:  1,
					Title:        "第一卷 崛兴",
					StartChapter: startCh,
					EndChapter:   endCh,
				},
			},
		},
	}
	if err := s.SaveProject(context.Background(), proj); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	prot := &domain.CodexEntry{
		ID:        "codex_" + projectID + "_prot",
		ProjectID: projectID,
		Category:  domain.CategoryCharacter,
		Archetype: "PROTAGONIST",
		Name:      "林凡",
		Realm:     "练气三层",
		Inventory: "生锈铁剑",
	}
	if err := s.SaveCodexEntry(context.Background(), prot); err != nil {
		t.Fatalf("SaveCodexEntry failed: %v", err)
	}
}

func TestHandleVolumeRoutes_MethodNotAllowed(t *testing.T) {
	srv, _, mockLLM := setupTestServerForVolume(t)
	defer mockLLM.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/volumes/p-1/1/produce", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed, got %d", w.Code)
	}
}

func TestHandleVolumeRoutes_NotFound(t *testing.T) {
	srv, _, mockLLM := setupTestServerForVolume(t)
	defer mockLLM.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/volumes/p-1/1/unknown", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", w.Code)
	}
}

func TestHandleVolumeRoutes_InvalidVolumeIndex(t *testing.T) {
	srv, _, mockLLM := setupTestServerForVolume(t)
	defer mockLLM.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/volumes/p-1/abc/produce", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", w.Code)
	}
}

func TestHandleVolumeRoutes_InvalidJSONBody(t *testing.T) {
	srv, _, mockLLM := setupTestServerForVolume(t)
	defer mockLLM.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/volumes/p-1/1/produce", strings.NewReader("{invalid-json}"))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on invalid json body, got %d", w.Code)
	}
}

func TestHandleVolumeRoutes_NonSSE_Success(t *testing.T) {
	srv, s, mockLLM := setupTestServerForVolume(t)
	defer mockLLM.Close()

	seedTestProject(t, s, "p-test-nonsse", 1, 2)

	bodyBytes, _ := json.Marshal(map[string]any{
		"words_target":      2000,
		"max_rewrite_loops": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/volumes/p-test-nonsse/1/produce", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var result engine.VolumeProduceResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal result failed: %v", err)
	}

	if result.VolumeIndex != 1 {
		t.Errorf("expected VolumeIndex 1, got %d", result.VolumeIndex)
	}
	if result.Stopped {
		t.Errorf("expected Stopped == false, got true")
	}
	if len(result.Chapters) != 2 {
		t.Errorf("expected 2 chapters, got %d", len(result.Chapters))
	}
}

func TestHandleVolumeRoutes_SSE_Stream(t *testing.T) {
	srv, s, mockLLM := setupTestServerForVolume(t)
	defer mockLLM.Close()

	seedTestProject(t, s, "p-test-sse", 1, 2)

	req := httptest.NewRequest(http.MethodPost, "/api/volumes/p-test-sse/1/produce?stream=true", nil)
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for SSE stream, got %d: %s", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		t.Errorf("expected Content-Type text/event-stream, got %s", contentType)
	}

	// Parse SSE events
	scanner := bufio.NewScanner(w.Body)
	var events []engine.VolumeEvent
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataPayload := strings.TrimPrefix(line, "data: ")
			var ev engine.VolumeEvent
			if err := json.Unmarshal([]byte(dataPayload), &ev); err == nil {
				events = append(events, ev)
			}
		}
	}

	if len(events) == 0 {
		t.Fatalf("expected streamed SSE events, got none. Body:\n%s", w.Body.String())
	}

	// Verify events contain VOLUME_START and VOLUME_COMPLETE
	hasStart := false
	hasComplete := false
	for _, ev := range events {
		if ev.Phase == "VOLUME_START" {
			hasStart = true
		}
		if ev.Phase == "VOLUME_COMPLETE" {
			hasComplete = true
		}
	}
	if !hasStart {
		t.Errorf("expected VOLUME_START event in SSE stream")
	}
	if !hasComplete {
		t.Errorf("expected VOLUME_COMPLETE event in SSE stream")
	}
}
