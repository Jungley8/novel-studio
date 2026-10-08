package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/engine"
)

func maskKey(k string) string {
	k = strings.TrimSpace(k)
	if k == "" {
		return ""
	}
	if len(k) <= 8 {
		return "********"
	}
	return k[:3] + "..." + k[len(k)-4:]
}

func (s *Server) maskedConfig() config.Config {
	cpy := *s.cfg
	cpy.APIKey = maskKey(cpy.APIKey)
	if cpy.ReviewerProvider != nil {
		rcpy := *cpy.ReviewerProvider
		rcpy.APIKey = maskKey(rcpy.APIKey)
		cpy.ReviewerProvider = &rcpy
	}
	if cpy.WriterProvider != nil {
		wcpy := *cpy.WriterProvider
		wcpy.APIKey = maskKey(wcpy.APIKey)
		cpy.WriterProvider = &wcpy
	}
	if cpy.ReasonerProvider != nil {
		rpcpy := *cpy.ReasonerProvider
		rpcpy.APIKey = maskKey(rpcpy.APIKey)
		cpy.ReasonerProvider = &rpcpy
	}
	return cpy
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, http.StatusOK, s.maskedConfig())
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		var updated config.Config
		if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid config JSON: "+err.Error())
			return
		}

		s.cfg.APIBase = updated.APIBase
		if updated.APIKey != "" && !strings.Contains(updated.APIKey, "...") && !strings.Contains(updated.APIKey, "***") {
			s.cfg.APIKey = updated.APIKey
		}
		s.cfg.ReasoningModel = updated.ReasoningModel
		s.cfg.WriterModel = updated.WriterModel
		if updated.ReviewerModel != "" {
			s.cfg.ReviewerModel = updated.ReviewerModel
		}

		// Update independent providers if specified
		if updated.ReviewerProvider != nil {
			if s.cfg.ReviewerProvider == nil {
				s.cfg.ReviewerProvider = &config.ProviderConfig{}
			}
			s.cfg.ReviewerProvider.APIBase = updated.ReviewerProvider.APIBase
			s.cfg.ReviewerProvider.Model = updated.ReviewerProvider.Model
			if updated.ReviewerProvider.APIKey != "" && !strings.Contains(updated.ReviewerProvider.APIKey, "...") && !strings.Contains(updated.ReviewerProvider.APIKey, "***") {
				s.cfg.ReviewerProvider.APIKey = updated.ReviewerProvider.APIKey
			}
		}

		if updated.WriterProvider != nil {
			if s.cfg.WriterProvider == nil {
				s.cfg.WriterProvider = &config.ProviderConfig{}
			}
			s.cfg.WriterProvider.APIBase = updated.WriterProvider.APIBase
			s.cfg.WriterProvider.Model = updated.WriterProvider.Model
			if updated.WriterProvider.APIKey != "" && !strings.Contains(updated.WriterProvider.APIKey, "...") && !strings.Contains(updated.WriterProvider.APIKey, "***") {
				s.cfg.WriterProvider.APIKey = updated.WriterProvider.APIKey
			}
		}

		if updated.ReasonerProvider != nil {
			if s.cfg.ReasonerProvider == nil {
				s.cfg.ReasonerProvider = &config.ProviderConfig{}
			}
			s.cfg.ReasonerProvider.APIBase = updated.ReasonerProvider.APIBase
			s.cfg.ReasonerProvider.Model = updated.ReasonerProvider.Model
			if updated.ReasonerProvider.APIKey != "" && !strings.Contains(updated.ReasonerProvider.APIKey, "...") && !strings.Contains(updated.ReasonerProvider.APIKey, "***") {
				s.cfg.ReasonerProvider.APIKey = updated.ReasonerProvider.APIKey
			}
		}

		if s.llmClient != nil {
			s.llmClient.UpdateCredentials(s.cfg.APIBase, s.cfg.APIKey)
		}
		if s.llmRouter != nil {
			s.llmRouter.UpdateFromConfig(s.cfg)
		}

		if err := s.cfg.Save(s.configPath); err != nil {
			errorResponse(w, http.StatusInternalServerError, "save config failed: "+err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, s.maskedConfig())
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

type TestConfigRequest struct {
	Target  string `json:"target"` // "reasoner", "writer", "reviewer", "default", "custom"
	APIBase string `json:"api_base"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

type TestConfigResponse struct {
	Status    string `json:"status"` // "ok" or "error"
	LatencyMS int64  `json:"latency_ms"`
	Model     string `json:"model"`
	APIBase   string `json:"api_base"`
	Reply     string `json:"reply,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (s *Server) handleConfigTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req TestConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request JSON: "+err.Error())
		return
	}

	targetBase := strings.TrimSpace(req.APIBase)
	targetKey := strings.TrimSpace(req.APIKey)
	targetModel := strings.TrimSpace(req.Model)

	// Resolve target credentials based on target role or defaults
	switch req.Target {
	case "reviewer":
		if targetBase == "" && s.cfg.ReviewerProvider != nil && s.cfg.ReviewerProvider.APIBase != "" {
			targetBase = s.cfg.ReviewerProvider.APIBase
		}
		if (targetKey == "" || strings.Contains(targetKey, "...") || strings.Contains(targetKey, "***")) && s.cfg.ReviewerProvider != nil {
			targetKey = s.cfg.ReviewerProvider.APIKey
		}
		if targetModel == "" {
			if s.cfg.ReviewerProvider != nil && s.cfg.ReviewerProvider.Model != "" {
				targetModel = s.cfg.ReviewerProvider.Model
			} else {
				targetModel = s.cfg.ReviewerModel
			}
		}
	case "writer":
		if targetBase == "" && s.cfg.WriterProvider != nil && s.cfg.WriterProvider.APIBase != "" {
			targetBase = s.cfg.WriterProvider.APIBase
		}
		if (targetKey == "" || strings.Contains(targetKey, "...") || strings.Contains(targetKey, "***")) && s.cfg.WriterProvider != nil {
			targetKey = s.cfg.WriterProvider.APIKey
		}
		if targetModel == "" {
			if s.cfg.WriterProvider != nil && s.cfg.WriterProvider.Model != "" {
				targetModel = s.cfg.WriterProvider.Model
			} else {
				targetModel = s.cfg.WriterModel
			}
		}
	case "reasoner":
		if targetBase == "" && s.cfg.ReasonerProvider != nil && s.cfg.ReasonerProvider.APIBase != "" {
			targetBase = s.cfg.ReasonerProvider.APIBase
		}
		if (targetKey == "" || strings.Contains(targetKey, "...") || strings.Contains(targetKey, "***")) && s.cfg.ReasonerProvider != nil {
			targetKey = s.cfg.ReasonerProvider.APIKey
		}
		if targetModel == "" {
			if s.cfg.ReasonerProvider != nil && s.cfg.ReasonerProvider.Model != "" {
				targetModel = s.cfg.ReasonerProvider.Model
			} else {
				targetModel = s.cfg.ReasoningModel
			}
		}
	}

	// Fallback to default server configuration if still unset
	if targetBase == "" {
		targetBase = s.cfg.APIBase
	}
	if targetKey == "" || strings.Contains(targetKey, "...") || strings.Contains(targetKey, "***") {
		targetKey = s.cfg.APIKey
	}
	if targetModel == "" {
		if s.cfg.ReasoningModel != "" {
			targetModel = s.cfg.ReasoningModel
		} else {
			targetModel = s.cfg.WriterModel
		}
	}

	if targetBase == "" {
		jsonResponse(w, http.StatusOK, TestConfigResponse{
			Status:  "error",
			Error:   "未指定 API Base URL",
			Model:   targetModel,
			APIBase: targetBase,
		})
		return
	}
	if targetKey == "" {
		jsonResponse(w, http.StatusOK, TestConfigResponse{
			Status:  "error",
			Error:   "未配置 API Key",
			Model:   targetModel,
			APIBase: targetBase,
		})
		return
	}
	if targetModel == "" {
		jsonResponse(w, http.StatusOK, TestConfigResponse{
			Status:  "error",
			Error:   "未指定待测试的模型名称 (Model)",
			Model:   targetModel,
			APIBase: targetBase,
		})
		return
	}

	// Perform actual lightweight diagnostic ping
	diagnosticClient := engine.NewHTTPLLMClient(targetBase, targetKey)
	testCtx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	start := time.Now()
	reply, err := diagnosticClient.ChatCompletion(
		testCtx,
		targetModel,
		"You are an API diagnostic service. Test connection.",
		"Ping! Please reply with 'NovelStudio Connection OK' only.",
		0.1,
	)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		jsonResponse(w, http.StatusOK, TestConfigResponse{
			Status:    "error",
			LatencyMS: latency,
			Model:     targetModel,
			APIBase:   targetBase,
			Error:     fmt.Sprintf("模型接口调用失败: %v", err),
		})
		return
	}

	jsonResponse(w, http.StatusOK, TestConfigResponse{
		Status:    "ok",
		LatencyMS: latency,
		Model:     targetModel,
		APIBase:   targetBase,
		Reply:     strings.TrimSpace(reply),
	})
}
