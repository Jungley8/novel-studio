package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/engine"
)

func (s *Server) handleVolumeRoutes(w http.ResponseWriter, r *http.Request) {
	r = r.WithContext(activity.WithSource(r.Context(), activity.SourceUIManual))
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/volumes/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[2] != "produce" {
		errorResponse(w, http.StatusNotFound, "route not found")
		return
	}

	projectID := parts[0]
	if projectID == "" {
		errorResponse(w, http.StatusBadRequest, "missing project id")
		return
	}

	volIndex, err := strconv.Atoi(parts[1])
	if err != nil || volIndex <= 0 {
		errorResponse(w, http.StatusBadRequest, "invalid volume index")
		return
	}

	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var reqBody struct {
		WordsTarget     int    `json:"words_target"`
		MaxRewriteLoops int    `json:"max_rewrite_loops"`
		NarrativeStyle  string `json:"narrative_style"`
		ChapterFrom     int    `json:"chapter_from"`
	}

	if r.Body != nil {
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&reqBody); err != nil && err != io.EOF {
			errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}

	if s.workshop == nil || s.store == nil {
		errorResponse(w, http.StatusInternalServerError, "workshop or store not configured")
		return
	}

	batch := engine.NewVolumeBatch(s.workshop, s.store)

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

		req := engine.VolumeProduceRequest{
			ProjectID:       projectID,
			VolumeIndex:     volIndex,
			ChapterFrom:     reqBody.ChapterFrom,
			WordsTarget:     reqBody.WordsTarget,
			MaxRewriteLoops: reqBody.MaxRewriteLoops,
			NarrativeStyle:  reqBody.NarrativeStyle,
			OnProgress: func(ev engine.VolumeEvent) {
				evBytes, _ := json.Marshal(ev)
				safeWrite(fmt.Sprintf("data: %s\n\n", string(evBytes)))
			},
		}

		_, err := batch.ProduceVolume(r.Context(), req)
		if err != nil {
			errBytes, _ := json.Marshal(engine.VolumeEvent{
				Phase:   "VOLUME_STOPPED",
				Message: err.Error(),
			})
			safeWrite(fmt.Sprintf("data: %s\n\n", string(errBytes)))
			return
		}
		return
	}

	req := engine.VolumeProduceRequest{
		ProjectID:       projectID,
		VolumeIndex:     volIndex,
		ChapterFrom:     reqBody.ChapterFrom,
		WordsTarget:     reqBody.WordsTarget,
		MaxRewriteLoops: reqBody.MaxRewriteLoops,
		NarrativeStyle:  reqBody.NarrativeStyle,
	}

	res, err := batch.ProduceVolume(r.Context(), req)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, res)
}
