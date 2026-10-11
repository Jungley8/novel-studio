package server_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/server"
	"github.com/Jungley8/novel-studio/internal/store"
)

func setupPipelineTestServer(t *testing.T, failLLM bool) (*server.Server, store.Store, *httptest.Server) {
	t.Helper()

	mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failLLM {
			http.Error(w, "mock llm failed", http.StatusInternalServerError)
			return
		}

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
		if strings.Contains(systemPrompt, "创世总纲") || strings.Contains(userPrompt, "创世总纲") {
			content = `{
				"theme_premise": "万古长夜，一人一剑逆伐诸仙",
				"world_axioms": ["天道崩塌", "灵气含毒"],
				"power_ladder": [
					{"realm": "凡胎境", "description": "凡俗武夫", "bottleneck": "开辟气海", "drawback": "寿不过百"}
				],
				"factions": [
					{"name": "天衍圣宗", "alignment": "守序正统", "doctrine": "替天行道", "threat_level": "极高"}
				],
				"key_characters": [
					{"name": "楚掌柜", "role": "引路导师", "realm": "凝气九重", "goal": "培育新秀", "fate_arc": "云游四海"}
				],
				"volume_arcs": [
					{
						"volume_index": 1,
						"title": "第一卷：边荒觉醒",
						"theme": "求存与复仇",
						"core_goal": "冲出边荒杀阵",
						"climax": "斩杀血煞老祖",
						"estimated_chapters": 1,
						"key_payoffs": ["觉醒不灭印"]
					}
				],
				"seed_hooks": [
					{
						"title": "胸前残印之谜",
						"details": "遇血发热的神秘铁印",
						"created_chapter": 1,
						"target_chapter": 10,
						"status": "OPEN"
					}
				]
			}`
		} else if strings.Contains(systemPrompt, "剧情规划师") || strings.Contains(systemPrompt, "章节意图卡") {
			content = `[
				{
					"chapter_index": 1,
					"conflict": "第 1 章生死冲突阻碍",
					"pov": "顾渊",
					"emotion_curve": ["压抑", "反转", "扬眉吐气"],
					"hook_plants": ["偶得古剑残片"],
					"hook_payoffs": [],
					"tension_target": 7,
					"expected_mutation": {
						"inventory_delta": "获得聚气散",
						"power_delta": "突破凝气一重"
					}
				}
			]`
		} else if strings.Contains(systemPrompt, "关系网络分析") || strings.Contains(systemPrompt, "实体之间") {
			content = `[]`
		} else if strings.Contains(systemPrompt, "精算师") || strings.Contains(systemPrompt, "结算") || strings.Contains(systemPrompt, "Settle") {
			content = `{"inventory_delta":"","power_delta":"","character_mutations":[],"evidence":[]}`
		} else if strings.Contains(systemPrompt, "分庭总审编") || strings.Contains(systemPrompt, "总审编") || strings.Contains(systemPrompt, "总编审") || strings.Contains(systemPrompt, "QualityGate") {
			content = `{"verdict":"ACCEPTED","score":90,"issues":[],"suggestions":"通过"}`
		} else if strings.Contains(systemPrompt, "总设计师") || strings.Contains(systemPrompt, "剧情节拍") || strings.Contains(systemPrompt, "Beats") {
			content = `{
				"beats": [
					{"phase": "破局", "tension": 9, "action": "逆转", "expectation_broken": "全面击溃"}
				],
				"state_mutation": {"inventory_delta": "+战利品", "power_delta": "突破"}
			}`
		}

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
	dbPath := filepath.Join(tmpDir, "pipeline_test.db")
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

func TestHandlePipeline_MethodNotAllowed(t *testing.T) {
	srv, _, mockLLM := setupPipelineTestServer(t, false)
	defer mockLLM.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/pipeline", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed, got %d", w.Code)
	}
}

func TestHandlePipeline_InvalidJSON(t *testing.T) {
	srv, _, mockLLM := setupPipelineTestServer(t, false)
	defer mockLLM.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/pipeline", strings.NewReader("{invalid-json"))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", w.Code)
	}
}

func TestHandlePipeline_Validation(t *testing.T) {
	srv, _, mockLLM := setupPipelineTestServer(t, false)
	defer mockLLM.Close()

	// Empty title
	body, _ := json.Marshal(map[string]any{
		"title":   "",
		"concept": "凡人修仙",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/pipeline", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty title, got %d", w.Code)
	}

	// Empty concept
	body, _ = json.Marshal(map[string]any{
		"title":   "逆苍穹",
		"concept": "   ",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/pipeline", bytes.NewReader(body))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty concept, got %d", w.Code)
	}
}

func TestHandlePipeline_DependenciesNotConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "dep_test.db")
	cfgPath := filepath.Join(tmpDir, "cfg.json")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

	cfg := config.DefaultConfig()
	llmClient := engine.NewHTTPLLMClient("http://localhost:9999", "sk-mock-key")
	linter := engine.NewLinter(nil)

	// orch == nil
	srv, err := server.New(cfg, cfgPath, s, llmClient, nil, linter)
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"title":   "逆苍穹",
		"concept": "凡人修仙",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/pipeline", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when orchestrator is nil, got %d", w.Code)
	}
}

func TestHandlePipeline_SSE_ErrorStream(t *testing.T) {
	srv, _, mockLLM := setupPipelineTestServer(t, true)
	defer mockLLM.Close()

	body, _ := json.Marshal(map[string]any{
		"title":   "逆天纪",
		"concept": "绝境求生",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/pipeline?stream=true", bytes.NewReader(body))
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for SSE error stream, got %d: %s", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		t.Errorf("expected Content-Type text/event-stream, got %s", contentType)
	}

	scanner := bufio.NewScanner(w.Body)
	var events []map[string]any
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataPayload := strings.TrimPrefix(line, "data: ")
			var ev map[string]any
			if err := json.Unmarshal([]byte(dataPayload), &ev); err == nil {
				events = append(events, ev)
			}
		}
	}

	hasError := false
	for _, ev := range events {
		if ev["phase"] == "PIPELINE_ERROR" {
			hasError = true
			if ev["error"] == nil || ev["error"] == "" {
				t.Errorf("expected non-empty error message in PIPELINE_ERROR event")
			}
		}
	}
	if !hasError {
		t.Fatalf("expected PIPELINE_ERROR event in stream, body:\n%s", w.Body.String())
	}
}

func TestHandlePipeline_SSE_SuccessStream(t *testing.T) {
	srv, _, mockLLM := setupPipelineTestServer(t, false)
	defer mockLLM.Close()

	body, _ := json.Marshal(map[string]any{
		"title":             "全流程测试仙帝",
		"concept":           "剑逆苍穹",
		"target_platform":   "起点仙侠",
		"volumes":           1,
		"auto_lock_intents": true,
		"words_target":      2000,
		"max_rewrite_loops": 1,
		"chapter_from":      1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/pipeline", bytes.NewReader(body))
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

	scanner := bufio.NewScanner(w.Body)
	var events []map[string]any
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataPayload := strings.TrimPrefix(line, "data: ")
			var ev map[string]any
			if err := json.Unmarshal([]byte(dataPayload), &ev); err == nil {
				events = append(events, ev)
			}
		}
	}

	phases := make(map[string]bool)
	var resultPayload map[string]any
	for _, ev := range events {
		phase, _ := ev["phase"].(string)
		phases[phase] = true
		if phase == "PIPELINE_RESULT" {
			if res, ok := ev["result"].(map[string]any); ok {
				resultPayload = res
			}
		}
	}

	expectedPhases := []string{
		"PIPELINE_START",
		"FRAMEWORK_READY",
		"RELATIONS_DONE",
		"INTENTS_READY",
		"INTENTS_LOCKED",
		"VOLUME_DONE",
		"PIPELINE_COMPLETE",
		"PIPELINE_RESULT",
	}

	for _, ep := range expectedPhases {
		if !phases[ep] {
			t.Errorf("expected phase %s in events, but was missing", ep)
		}
	}

	if resultPayload == nil {
		t.Fatalf("expected valid result in PIPELINE_RESULT event, got nil. Events:\n%+v", events)
	}
	if resultPayload["title"] != "全流程测试仙帝" {
		t.Errorf("expected title '全流程测试仙帝', got %v", resultPayload["title"])
	}
	if resultPayload["project_id"] == nil || resultPayload["project_id"] == "" {
		t.Errorf("expected non-empty project_id")
	}
}

func TestHandlePipeline_Sync_Success(t *testing.T) {
	srv, _, mockLLM := setupPipelineTestServer(t, false)
	defer mockLLM.Close()

	body, _ := json.Marshal(map[string]any{
		"title":             "同步测试神尊",
		"concept":           "逆天登神",
		"target_platform":   "起点仙侠",
		"volumes":           1,
		"auto_lock_intents": true,
		"words_target":      2000,
		"max_rewrite_loops": 1,
		"chapter_from":      1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/pipeline", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var res engine.PipelineResult
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal PipelineResult failed: %v", err)
	}

	if res.Title != "同步测试神尊" {
		t.Errorf("expected Title '同步测试神尊', got %s", res.Title)
	}
	if res.ProjectID == "" {
		t.Errorf("expected non-empty ProjectID")
	}
	if len(res.Volumes) != 1 {
		t.Errorf("expected 1 volume result, got %d", len(res.Volumes))
	}
}

func TestHandlePipeline_Sync_Error(t *testing.T) {
	srv, _, mockLLM := setupPipelineTestServer(t, true)
	defer mockLLM.Close()

	body, _ := json.Marshal(map[string]any{
		"title":   "同步失败测试",
		"concept": "必败逆流",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/pipeline", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal error response failed: %v", err)
	}
	if errResp["error"] == "" {
		t.Errorf("expected non-empty error field in response")
	}
}
