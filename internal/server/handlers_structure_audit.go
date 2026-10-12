package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

type structureFindingStore interface {
	SaveStructureFinding(ctx context.Context, f *domain.StructureFinding) error
	ListStructureFindings(ctx context.Context, projectID string) ([]*domain.StructureFinding, error)
	UpdateStructureFindingStatus(ctx context.Context, id, status string) error
	DeleteStructureFindingsByProject(ctx context.Context, projectID string) error
	StructureFindingProjectID(ctx context.Context, id string) (string, error)
}

func (s *Server) getStructureFindingStore() structureFindingStore {
	if sfs, ok := s.store.(structureFindingStore); ok {
		return sfs
	}
	return nil
}

// handleProjectStructureAudit dispatches:
// - POST /api/projects/:id/structure-audit (run)
// - GET  /api/projects/:id/structure-audit (list)
// - POST /api/projects/:id/structure-audit/status (update status)
func (s *Server) handleProjectStructureAudit(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	fStore := s.getStructureFindingStore()
	if fStore == nil {
		errorResponse(w, http.StatusInternalServerError, "store does not support structure audit operations")
		return
	}

	// 1. Root: /api/projects/:id/structure-audit
	if len(subParts) == 0 || (len(subParts) == 1 && subParts[0] == "") {
		switch r.Method {
		case http.MethodGet:
			s.handleListStructureFindings(w, r, projectID, fStore)
		case http.MethodPost:
			s.handleRunStructureAudit(w, r, projectID, fStore)
		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// 2. Subroute: /api/projects/:id/structure-audit/status
	if len(subParts) == 1 && subParts[0] == "status" {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handleUpdateStructureFindingStatus(w, r, projectID, fStore)
		return
	}

	errorResponse(w, http.StatusNotFound, "not found")
}

func (s *Server) handleListStructureFindings(w http.ResponseWriter, r *http.Request, projectID string, fStore structureFindingStore) {
	ctx := r.Context()
	findings, err := fStore.ListStructureFindings(ctx, projectID)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "list structure findings failed: "+err.Error())
		return
	}
	if findings == nil {
		findings = []*domain.StructureFinding{}
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"findings": findings,
	})
}

func (s *Server) handleRunStructureAudit(w http.ResponseWriter, r *http.Request, projectID string, fStore structureFindingStore) {
	ctx := r.Context()

	// 1. Read project
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil || project == nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	// 2. Read chapter summaries (无界: 0, 0)
	var summaries []*domain.ChapterSummary
	if lister, ok := s.store.(interface {
		ListChapterSummaries(ctx context.Context, projectID string, from, to int) ([]*domain.ChapterSummary, error)
	}); ok {
		sums, err := lister.ListChapterSummaries(ctx, projectID, 0, 0)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "list chapter summaries failed: "+err.Error())
			return
		}
		summaries = sums
	}

	// 无章摘要时返回空列表 + {"findings": [], "note": "暂无章摘要可审读"}，不报错
	if len(summaries) == 0 {
		jsonResponse(w, http.StatusOK, map[string]any{
			"findings": []domain.StructureFinding{},
			"note":     "暂无章摘要可审读",
		})
		return
	}

	// 3. Read & filter open/fermenting hooks
	hooks, herr := s.store.ListPlotHooks(ctx, projectID)
	if herr != nil {
		errorResponse(w, http.StatusInternalServerError, "list plot hooks failed: "+herr.Error())
		return
	}
	activeHooks := make([]*domain.PlotHook, 0, len(hooks))
	for _, h := range hooks {
		if h != nil && (h.Status == domain.HookStatusOpen || h.Status == domain.HookStatusFermenting) {
			activeHooks = append(activeHooks, h)
		}
	}

	// 4. Bench 摘要 best-effort 复用
	benchSummary := ""
	chapters, chErr := s.store.ListChapters(ctx, projectID)
	if chErr == nil {
		protagonist, _ := s.store.GetProtagonistEntry(ctx, projectID)
		snapshots, _ := s.store.ListStateSnapshots(ctx, projectID)
		styleVaultSeeded := false
		if svStore, ok := s.store.(interface {
			GetStyleVault(ctx context.Context, projectID string) (*domain.StyleVault, error)
		}); ok {
			if vault, err := svStore.GetStyleVault(ctx, projectID); err == nil && vault != nil {
				styleVaultSeeded = true
			}
		}
		report := engine.RunBenchProbes(project, chapters, hooks, protagonist, styleVaultSeeded, snapshots)
		if report != nil {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("已归档章节数: %d\n", report.Chapters))
			sb.WriteString(fmt.Sprintf("字数体检: 目标 %d, 平均 %d, 最少 %d, 最多 %d, 低于及格线 %d 章\n",
				report.WordCount.Target, report.WordCount.Avg, report.WordCount.Min, report.WordCount.Max, report.WordCount.UnderTarget))
			sb.WriteString(fmt.Sprintf("章型节奏: 最长连续同型 %d 章 (分布: %v)\n",
				report.ChapterTypes.MaxConsecutive, report.ChapterTypes.Distribution))
			sb.WriteString(fmt.Sprintf("伏笔台账: 活跃(OPEN) %d, 发酵(FERMENTING) %d, 已回收 %d, 逾期未回收 %d 条\n",
				report.Hooks.Open, report.Hooks.Fermenting, report.Hooks.Resolved, len(report.Hooks.Overdue)))
			if len(report.Milestones.Overdue) > 0 {
				sb.WriteString(fmt.Sprintf("里程碑: 逾期未兑现 %d 个\n", len(report.Milestones.Overdue)))
			}
			if !report.LedgerReplay.Clean {
				sb.WriteString(fmt.Sprintf("账本状态: 存在漂移 (%d 条)\n", len(report.LedgerReplay.Drifts)))
			}
			benchSummary = sb.String()
		}
	}

	// 5. ReviewerModel 回退链（抄 handleReviewDraft）
	var req struct {
		Model         string `json:"model"`
		ReviewerModel string `json:"reviewer_model"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	reviewerModel := strings.TrimSpace(req.ReviewerModel)
	if reviewerModel == "" {
		reviewerModel = strings.TrimSpace(req.Model)
	}
	if reviewerModel == "" {
		reviewerModel = strings.TrimSpace(s.cfg.ReviewerModel)
	}
	if reviewerModel == "" {
		reviewerModel = strings.TrimSpace(s.cfg.ReasoningModel)
	}
	if reviewerModel == "" {
		reviewerModel = strings.TrimSpace(s.cfg.WriterModel)
	}

	if s.orch == nil {
		errorResponse(w, http.StatusInternalServerError, "orchestrator not configured")
		return
	}

	// 6. RunStructureAudit 执行体检
	findings, usage, err := s.orch.RunStructureAudit(ctx, reviewerModel, project, summaries, activeHooks, benchSummary)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "run structure audit failed: "+err.Error())
		return
	}

	// 7. 覆盖语义：运行成功才全量覆盖（删旧存新），失败则旧结果原样保留
	if err := fStore.DeleteStructureFindingsByProject(ctx, projectID); err != nil {
		errorResponse(w, http.StatusInternalServerError, "clear previous structure findings failed: "+err.Error())
		return
	}

	now := time.Now().UTC()
	savedFindings := make([]*domain.StructureFinding, 0, len(findings))
	for i := range findings {
		f := findings[i]
		if strings.TrimSpace(f.ID) == "" {
			f.ID = fmt.Sprintf("find_%d_%d_%d", now.UnixNano(), f.ChapterStart, rand.Intn(10000))
		}
		f.ProjectID = projectID
		if f.CreatedAt.IsZero() {
			f.CreatedAt = now
		}
		if err := fStore.SaveStructureFinding(ctx, &f); err != nil {
			errorResponse(w, http.StatusInternalServerError, "save structure finding failed: "+err.Error())
			return
		}
		savedFindings = append(savedFindings, &f)
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"findings": savedFindings,
		"tokens":   usage,
	})
}

func (s *Server) handleUpdateStructureFindingStatus(w http.ResponseWriter, r *http.Request, projectID string, fStore structureFindingStore) {
	ctx := r.Context()

	var req struct {
		FindingID string `json:"finding_id"`
		Status    string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	req.FindingID = strings.TrimSpace(req.FindingID)
	req.Status = strings.TrimSpace(req.Status)

	if req.FindingID == "" {
		errorResponse(w, http.StatusBadRequest, "finding_id cannot be empty")
		return
	}

	// 租户作用域：finding 必须归属本 project（对齐伏笔 PlotHookProjectID 定式），否则 404。
	owner, err := fStore.StructureFindingProjectID(ctx, req.FindingID)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "resolve finding owner failed: "+err.Error())
		return
	}
	if owner == "" || owner != projectID {
		errorResponse(w, http.StatusNotFound, "structure finding not found")
		return
	}

	// 校验 status 枚举合法，否则 400
	switch domain.StructureFindingStatus(req.Status) {
	case domain.StructureFindingStatusOpen,
		domain.StructureFindingStatusResolved,
		domain.StructureFindingStatusIgnored:
	default:
		errorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid status %q: must be OPEN, RESOLVED, or IGNORED", req.Status))
		return
	}

	if err := fStore.UpdateStructureFindingStatus(ctx, req.FindingID, req.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errorResponse(w, http.StatusNotFound, "structure finding not found")
			return
		}
		errorResponse(w, http.StatusInternalServerError, "update status failed: "+err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"status": "updated",
	})
}
