package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/engine"
)

type PipelineAPIRequest struct {
	Title           string `json:"title"`
	Concept         string `json:"concept"`
	TargetPlatform  string `json:"target_platform"`
	ReasoningModel  string `json:"reasoning_model"`
	Volumes         int    `json:"volumes"`
	AutoLockIntents *bool  `json:"auto_lock_intents"`
	WordsTarget     int    `json:"words_target"`
	MaxRewriteLoops int    `json:"max_rewrite_loops"`
	NarrativeStyle  string `json:"narrative_style"`
	ChapterFrom     int    `json:"chapter_from"`
}

func resolvePipelineDefaults(body PipelineAPIRequest, cfg *config.Config) (words int, loops int, style string, autoLock bool) {
	words = 2000
	if body.WordsTarget > 0 {
		words = body.WordsTarget
	} else if cfg != nil && cfg.DefaultWordsTarget > 0 {
		words = cfg.DefaultWordsTarget
	}

	loops = 3
	if body.MaxRewriteLoops > 0 {
		loops = body.MaxRewriteLoops
	} else if cfg != nil && cfg.DefaultMaxRewriteLoops > 0 {
		loops = cfg.DefaultMaxRewriteLoops
	}

	style = ""
	if body.NarrativeStyle != "" {
		style = body.NarrativeStyle
	} else if cfg != nil {
		style = cfg.DefaultNarrativeStyle
	}

	autoLock = false
	if body.AutoLockIntents != nil {
		autoLock = *body.AutoLockIntents
	} else if cfg != nil {
		autoLock = cfg.DefaultAutoLockIntents
	}

	return words, loops, style, autoLock
}

func ResolvePipelineDefaults(body PipelineAPIRequest, cfg *config.Config) (words int, loops int, style string, autoLock bool) {
	return resolvePipelineDefaults(body, cfg)
}

func (s *Server) handlePipeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	runID := activity.NewRunID()
	ctx := activity.WithRunID(activity.WithSource(r.Context(), activity.SourceBootstrap), runID)
	r = r.WithContext(ctx)

	var reqBody PipelineAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if strings.TrimSpace(reqBody.Title) == "" {
		errorResponse(w, http.StatusBadRequest, "title cannot be empty")
		return
	}
	if strings.TrimSpace(reqBody.Concept) == "" {
		errorResponse(w, http.StatusBadRequest, "concept cannot be empty")
		return
	}

	if s.orch == nil || s.workshop == nil || s.store == nil {
		errorResponse(w, http.StatusInternalServerError, "orchestrator, workshop, or store not configured")
		return
	}

	reasoningModel := reqBody.ReasoningModel
	if reasoningModel == "" && s.cfg != nil {
		reasoningModel = s.cfg.ReasoningModel
	}

	words, loops, style, autoLock := resolvePipelineDefaults(reqBody, s.cfg)

	batch := engine.NewVolumeBatch(s.workshop, s.store)
	pipeline := engine.NewStudioPipeline(s.orch, batch, s.store)

	isSSE := strings.Contains(r.Header.Get("Accept"), "text/event-stream") || r.URL.Query().Get("stream") == "true"
	if isSSE {
		rc := http.NewResponseController(w)
		_ = rc.SetWriteDeadline(time.Time{})

		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		var writeMu sync.Mutex
		safeWrite := func(data string) {
			writeMu.Lock()
			defer writeMu.Unlock()
			_, _ = fmt.Fprint(w, data)
			flusher.Flush()
		}

		stopHeartbeat := make(chan struct{})
		defer close(stopHeartbeat)
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					safeWrite(": ping\n\n")
				case <-stopHeartbeat:
					return
				case <-r.Context().Done():
					return
				}
			}
		}()

		req := engine.PipelineRequest{
			Title:           reqBody.Title,
			TargetPlatform:  reqBody.TargetPlatform,
			Concept:         reqBody.Concept,
			ReasoningModel:  reasoningModel,
			Volumes:         reqBody.Volumes,
			AutoLockIntents: autoLock,
			WordsTarget:     words,
			MaxRewriteLoops: loops,
			NarrativeStyle:  style,
			ChapterFrom:     reqBody.ChapterFrom,
			RunID:           runID,
			OnProgress: func(ev engine.PipelineEvent) {
				evBytes, _ := json.Marshal(ev)
				safeWrite(fmt.Sprintf("data: %s\n\n", string(evBytes)))
			},
		}

		res, err := pipeline.Run(r.Context(), req)
		if err != nil {
			errBytes, _ := json.Marshal(map[string]any{
				"phase": "PIPELINE_ERROR",
				"error": err.Error(),
			})
			safeWrite(fmt.Sprintf("data: %s\n\n", string(errBytes)))
			return
		}

		resBytes, _ := json.Marshal(map[string]any{
			"phase":  "PIPELINE_RESULT",
			"result": res,
		})
		safeWrite(fmt.Sprintf("data: %s\n\n", string(resBytes)))
		return
	}

	req := engine.PipelineRequest{
		Title:           reqBody.Title,
		TargetPlatform:  reqBody.TargetPlatform,
		Concept:         reqBody.Concept,
		ReasoningModel:  reasoningModel,
		Volumes:         reqBody.Volumes,
		AutoLockIntents: autoLock,
		WordsTarget:     words,
		MaxRewriteLoops: loops,
		NarrativeStyle:  style,
		ChapterFrom:     reqBody.ChapterFrom,
		RunID:           runID,
	}

	res, err := pipeline.Run(r.Context(), req)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, res)
}
