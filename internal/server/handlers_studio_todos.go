package server

import (
	"fmt"
	"net/http"
	"time"
)

// StudioTodoItem represents a chapter checkpoint awaiting human intervention.
type StudioTodoItem struct {
	ProjectID    string   `json:"project_id"`
	ProjectTitle string   `json:"project_title"`
	ChapterIndex int      `json:"chapter_index"`
	Verdict      string   `json:"verdict"`
	Score        int      `json:"score"`
	Issues       []string `json:"issues"`
	UpdatedAt    string   `json:"updated_at"`
}

// StudioTodosResponse encapsulates the list of pending todos.
type StudioTodosResponse struct {
	Todos []StudioTodoItem `json:"todos"`
}

// handleStudioTodos handles GET /api/studio/todos.
func (s *Server) handleStudioTodos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := r.Context()
	checkpoints, err := s.store.ListNeedsHumanCheckpoints(ctx)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "list needs human checkpoints: "+err.Error())
		return
	}

	projects, err := s.store.ListProjects(ctx)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "list projects: "+err.Error())
		return
	}

	titleMap := make(map[string]string, len(projects))
	archivedChapters := make(map[string]map[int]bool, len(projects))
	for _, p := range projects {
		titleMap[p.ID] = p.Title
		chapters, err := s.store.ListChapters(ctx, p.ID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("list chapters for project %s: %v", p.ID, err))
			return
		}
		chMap := make(map[int]bool, len(chapters))
		for _, ch := range chapters {
			chMap[ch.ChapterIndex] = true
		}
		archivedChapters[p.ID] = chMap
	}

	todos := make([]StudioTodoItem, 0)
	for _, cp := range checkpoints {
		if cp == nil || cp.AuditReport == nil || !cp.AuditReport.NeedsHuman {
			continue
		}
		// 过滤掉已归档章（防陈旧断点）
		if archived, ok := archivedChapters[cp.ProjectID]; ok && archived[cp.ChapterIndex] {
			continue
		}

		title := titleMap[cp.ProjectID]
		if title == "" {
			title = cp.ProjectID
		}

		issues := cp.AuditReport.Issues
		if issues == nil {
			issues = []string{}
		}

		updatedAtStr := cp.UpdatedAt.Format(time.RFC3339)
		if cp.UpdatedAt.IsZero() {
			updatedAtStr = time.Now().Format(time.RFC3339)
		}

		todos = append(todos, StudioTodoItem{
			ProjectID:    cp.ProjectID,
			ProjectTitle: title,
			ChapterIndex: cp.ChapterIndex,
			Verdict:      string(cp.AuditReport.Verdict),
			Score:        cp.AuditReport.Score,
			Issues:       issues,
			UpdatedAt:    updatedAtStr,
		})
	}

	jsonResponse(w, http.StatusOK, StudioTodosResponse{Todos: todos})
}
