package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// -----------------------------------------------------------------------------
// The Forecast & Narrative Tree (前瞻推演与活跃规划)
// -----------------------------------------------------------------------------

// ComputeFingerprint calculates the deterministic v2 fingerprint of a project's upstream premise:
// Rule: "v2:<frameworkHash16>:<hooksHash16>", each segment the first 16 hex chars of sha256
// (crypto/sha256 + hex precedent per chapter_history.go) over:
//   - framework: the raw framework_json column of projects ("" when NULL/absent —
//     framework-less legacy data is a legal state, hashed as empty, never an error)
//   - hooks: the hook IDENTITY set — plot_hooks id + created_chapter + target_chapter,
//     WITHOUT status, sorted before joining for determinism
//
// v2 semantics: normal execution NEVER stales a plan. Archiving chapters (positional shift:
// maxChapter/lastID) and hook status flow (OPEN/FERMENTING/RESOLVED) are execution, not rewrites.
// Only upstream rewrites stale a plan: framework/volume-outline premise changes, history rewritten
// below base (uncommit/delete), or hook-set surgery (add/delete hooks, as opposed to status flow).
// Empty projects (no framework, no hooks) hash zero values: deterministic and stable, never random.
// Clean switch (pre-v1.0, no dual-track, no migration code): the legacy "maxChapter:lastID:hookCount"
// fingerprint never carries a "v2:" prefix, so any stored legacy fingerprint mismatches on first
// compare and goes stale fail-closed (regenerate rather than hallucinate injections).
func (s *SQLiteStore) ComputeFingerprint(ctx context.Context, projectID string) (string, error) {
	var frameworkRaw sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(framework_json, '')
		FROM projects WHERE id = ?`, projectID).Scan(&frameworkRaw); err != nil {
		return "", fmt.Errorf("compute fingerprint: query framework: %w", err)
	}
	fwStr := ""
	if frameworkRaw.Valid {
		fwStr = frameworkRaw.String
	}
	fwSum := sha256.Sum256([]byte(fwStr))
	fwHex := hex.EncodeToString(fwSum[:])[:16]

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, created_chapter, target_chapter
		FROM plot_hooks WHERE project_id = ?`, projectID)
	if err != nil {
		return "", fmt.Errorf("compute fingerprint: query hooks: %w", err)
	}
	defer rows.Close()

	var identities []string
	for rows.Next() {
		var id string
		var created, target int
		if err := rows.Scan(&id, &created, &target); err != nil {
			return "", fmt.Errorf("compute fingerprint: scan hook: %w", err)
		}
		identities = append(identities, fmt.Sprintf("%s:%d:%d", id, created, target))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("compute fingerprint: hooks rows: %w", err)
	}
	sort.Strings(identities)
	hooksSum := sha256.Sum256([]byte(strings.Join(identities, "|")))
	hooksHex := hex.EncodeToString(hooksSum[:])[:16]

	return fmt.Sprintf("v2:%s:%s", fwHex, hooksHex), nil
}

// forecastStaleByFingerprint reports whether an active forecast must transition to stale under v2 rules.
// Rule: stored fingerprint != current v2 fingerprint (upstream premise rewrite) → stale; else
// MAX(chapter_index) < BaseChapter (history below base rewritten via uncommit/delete) → stale.
// BaseChapter is a string column: strconv.Atoi parse failure is fail-closed stale. The MAX lookup
// lives here (ComputeFingerprint no longer queries it), copying the COALESCE(MAX(...)) precedent.
// Running past base+horizon introduces NO new rule (GetActivePlan's in-memory consumed derivation
// already covers overrun; out-of-range stays untouched).
// Pure query (one MAX lookup) + no row iteration: safe to call after rows.Close() in ListForecasts;
// MUST NOT be called inside a rows loop on this single-connection pool (deadlock regression line).
func (s *SQLiteStore) forecastStaleByFingerprint(ctx context.Context, projectID string, f *domain.Forecast, currFP string) bool {
	if f.Fingerprint != currFP {
		return true
	}
	base, err := strconv.Atoi(strings.TrimSpace(f.BaseChapter))
	if err != nil {
		return true
	}
	var maxChapter int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(chapter_index), 0) FROM chapters WHERE project_id = ?`, projectID).Scan(&maxChapter); err != nil {
		return true
	}
	return maxChapter < base
}

// SaveForecast persists a new narrative forecast and enforces the single-active invariant:
// existing active forecasts for the project are archived before saving the new one.
// Activity ledger records an event with ID and branch count without dumping the large branches_json.
func (s *SQLiteStore) SaveForecast(ctx context.Context, f *domain.Forecast) error {
	if f == nil {
		return errors.New("forecast cannot be nil")
	}
	f.ProjectID = strings.TrimSpace(f.ProjectID)
	if f.ProjectID == "" {
		return errors.New("forecast project_id cannot be empty")
	}
	f.ID = strings.TrimSpace(f.ID)
	if f.ID == "" {
		f.ID = fmt.Sprintf("fc_%d", time.Now().UnixNano())
	}
	if f.Fingerprint == "" {
		fp, err := s.ComputeFingerprint(ctx, f.ProjectID)
		if err != nil {
			return fmt.Errorf("compute fingerprint for save forecast: %w", err)
		}
		f.Fingerprint = fp
	}
	if f.Status == "" {
		f.Status = domain.ForecastStatusActive
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now()
	}

	branchesJSON, err := json.Marshal(f.Branches)
	if err != nil {
		return fmt.Errorf("marshal branches failed: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// Single-active invariant: archive existing active forecasts for this project
	if _, err := tx.ExecContext(ctx, `
		UPDATE narrative_forecasts 
		SET status = ? 
		WHERE project_id = ? AND status = ?`,
		domain.ForecastStatusArchived, f.ProjectID, domain.ForecastStatusActive,
	); err != nil {
		return fmt.Errorf("archive previous active forecasts failed: %w", err)
	}

	// Insert the new forecast
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO narrative_forecasts (
			id, project_id, base_chapter, divergence, branch_count, horizon, fingerprint, status, branches_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.ProjectID, f.BaseChapter, f.Divergence, f.BranchCount, f.Horizon, f.Fingerprint, f.Status, string(branchesJSON), f.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert narrative forecast failed: %w", err)
	}

	// Activity ledger: only records ID and branch count, NEVER branches_json snapshot
	baseIdx, _ := strconv.Atoi(f.BaseChapter)
	e := newEvent(ctx, f.ProjectID, "forecast", f.ID, baseIdx, "forecast.created", "生成前瞻推演",
		fmt.Sprintf("前瞻分支数: %d, 跨度: %d", f.BranchCount, f.Horizon))
	if err := emitLedger(ctx, tx, e); err != nil {
		return fmt.Errorf("emit forecast ledger failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	tx = nil
	return nil
}

// LoadForecast retrieves a forecast by ID, evaluating its fingerprint against the current canon state.
// If the fingerprint mismatches, the forecast and its associated active plan are transitioned to stale.
func (s *SQLiteStore) LoadForecast(ctx context.Context, projectID, forecastID string) (*domain.Forecast, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, base_chapter, divergence, branch_count, horizon, fingerprint, status, branches_json, created_at
		FROM narrative_forecasts
		WHERE project_id = ? AND id = ?`, projectID, forecastID)

	var f domain.Forecast
	var branchesJSON string
	if err := row.Scan(
		&f.ID, &f.ProjectID, &f.BaseChapter, &f.Divergence, &f.BranchCount, &f.Horizon, &f.Fingerprint, &f.Status, &branchesJSON, &f.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("forecast %s not found", forecastID)
		}
		return nil, err
	}

	if branchesJSON != "" {
		_ = json.Unmarshal([]byte(branchesJSON), &f.Branches)
	}

	// Fingerprint check: v2 premise fingerprint plus fail-closed guard for history
	// rewritten below base (see forecastStaleByFingerprint). Normal execution never stales.
	currFP, fpErr := s.ComputeFingerprint(ctx, projectID)
	// 指纹查询失败必须 Fail-Closed 判 stale，不许用残缺计算冒充合法指纹
	if f.Status == domain.ForecastStatusActive && (fpErr != nil || s.forecastStaleByFingerprint(ctx, projectID, &f, currFP)) {
		f.Status = domain.ForecastStatusStale
		_, _ = s.db.ExecContext(ctx, `UPDATE narrative_forecasts SET status = ? WHERE id = ?`, domain.ForecastStatusStale, f.ID)
		_, _ = s.db.ExecContext(ctx, `
			UPDATE project_active_plan 
			SET status = ?, updated_at = ? 
			WHERE project_id = ? AND forecast_id = ? AND status = ?`,
			domain.PlanStatusStale, time.Now(), projectID, f.ID, domain.PlanStatusActive,
		)
	}

	return &f, nil
}

// ListForecasts returns all forecasts for a project, refreshing stale statuses for any active forecasts.
func (s *SQLiteStore) ListForecasts(ctx context.Context, projectID string) ([]*domain.Forecast, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, base_chapter, divergence, branch_count, horizon, fingerprint, status, branches_json, created_at
		FROM narrative_forecasts
		WHERE project_id = ?
		ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.Forecast
	var hasActive bool

	for rows.Next() {
		var f domain.Forecast
		var branchesJSON string
		if err := rows.Scan(
			&f.ID, &f.ProjectID, &f.BaseChapter, &f.Divergence, &f.BranchCount, &f.Horizon, &f.Fingerprint, &f.Status, &branchesJSON, &f.CreatedAt,
		); err != nil {
			return nil, err
		}
		if branchesJSON != "" {
			_ = json.Unmarshal([]byte(branchesJSON), &f.Branches)
		}
		if f.Status == domain.ForecastStatusActive {
			hasActive = true
		}

		list = append(list, &f)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	_ = rows.Close()

	if hasActive {
		currFP, fpErr := s.ComputeFingerprint(ctx, projectID)
		now := time.Now()
		for _, f := range list {
			// 指纹查询失败必须 Fail-Closed 判 stale，不许用残缺计算冒充合法指纹
			if f.Status == domain.ForecastStatusActive && (fpErr != nil || s.forecastStaleByFingerprint(ctx, projectID, f, currFP)) {
				f.Status = domain.ForecastStatusStale
				_, _ = s.db.ExecContext(ctx, `UPDATE narrative_forecasts SET status = ? WHERE id = ?`, domain.ForecastStatusStale, f.ID)
				_, _ = s.db.ExecContext(ctx, `
					UPDATE project_active_plan 
					SET status = ?, updated_at = ? 
					WHERE project_id = ? AND forecast_id = ? AND status = ?`,
					domain.PlanStatusStale, now, projectID, f.ID, domain.PlanStatusActive,
				)
			}
		}
	}

	if list == nil {
		list = []*domain.Forecast{}
	}
	return list, nil
}

// SelectBranch adopts a branch from a forecast as the project's single active plan.
// STRICT ISOLATION GUARD: This function writes ONLY to narrative_forecasts and project_active_plan.
// Any modifications to chapters, codex_entries, plot_hooks, or runtime state are forbidden.
func (s *SQLiteStore) SelectBranch(ctx context.Context, projectID, forecastID, branchID string) (*domain.ActivePlan, error) {
	fc, err := s.LoadForecast(ctx, projectID, forecastID)
	if err != nil {
		return nil, err
	}

	var selectedBranch *domain.ForecastBranch
	for i := range fc.Branches {
		if fc.Branches[i].ID == branchID {
			selectedBranch = &fc.Branches[i]
			break
		}
	}
	if selectedBranch == nil {
		return nil, fmt.Errorf("branch %s not found in forecast %s", branchID, forecastID)
	}

	baseIdx, _ := strconv.Atoi(fc.BaseChapter)
	var planChapters []domain.PlanChapter
	for i := 1; i <= fc.Horizon; i++ {
		chIdx := baseIdx + i
		planChapters = append(planChapters, domain.PlanChapter{
			ChapterIndex: chIdx,
			Title:        fmt.Sprintf("第%d章", chIdx),
			Summary:      selectedBranch.BeatsSummary,
			Status:       domain.PlanStatusActive,
		})
	}

	planChaptersJSON, err := json.Marshal(planChapters)
	if err != nil {
		return nil, fmt.Errorf("marshal plan chapters failed: %w", err)
	}

	planText := fmt.Sprintf("分支: %s\n节拍概述: %s\n角色抉择: %s\n预期变化: %s\n潜在风险: %s\n意图匹配: %s",
		selectedBranch.ID, selectedBranch.BeatsSummary, selectedBranch.Decisions,
		selectedBranch.ExpectedDeltas, selectedBranch.Risks, selectedBranch.IntentMatch)

	now := time.Now()
	activePlan := &domain.ActivePlan{
		ProjectID:      projectID,
		ForecastID:     forecastID,
		BranchID:       branchID,
		PlanText:       planText,
		Status:         domain.PlanStatusActive,
		PlanChapters:   planChapters,
		ExpectedDeltas: selectedBranch.ExpectedDeltas,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// Status guard: ensure target forecast is active. stale and archived forecasts are strictly rejected.
	var currentStatus string
	if err := tx.QueryRowContext(ctx, `
		SELECT status 
		FROM narrative_forecasts 
		WHERE project_id = ? AND id = ?`,
		projectID, forecastID,
	).Scan(&currentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("forecast %s not found", forecastID)
		}
		return nil, fmt.Errorf("query forecast status failed: %w", err)
	}

	if currentStatus != domain.ForecastStatusActive {
		return nil, fmt.Errorf("cannot select branch: forecast is %s, regenerate or create a new forecast first", currentStatus)
	}

	// Single-active invariant: archive any other active forecasts for this project
	if _, err := tx.ExecContext(ctx, `
		UPDATE narrative_forecasts 
		SET status = ? 
		WHERE project_id = ? AND status = ? AND id != ?`,
		domain.ForecastStatusArchived, projectID, domain.ForecastStatusActive, forecastID,
	); err != nil {
		return nil, fmt.Errorf("archive previous active forecasts failed: %w", err)
	}

	// 1. Write narrative_forecasts status
	if _, err := tx.ExecContext(ctx, `
		UPDATE narrative_forecasts 
		SET status = ? 
		WHERE id = ?`,
		domain.ForecastStatusActive, forecastID,
	); err != nil {
		return nil, fmt.Errorf("update forecast status failed: %w", err)
	}

	// 2. Write project_active_plan (single active plan guaranteed by project_id PRIMARY KEY)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_active_plan (
			project_id, forecast_id, branch_id, plan_text, status,
			plan_chapters_json, expected_deltas, warning_message, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id) DO UPDATE SET
			forecast_id = excluded.forecast_id,
			branch_id = excluded.branch_id,
			plan_text = excluded.plan_text,
			status = excluded.status,
			plan_chapters_json = excluded.plan_chapters_json,
			expected_deltas = excluded.expected_deltas,
			warning_message = excluded.warning_message,
			updated_at = excluded.updated_at`,
		projectID, forecastID, branchID, planText, domain.PlanStatusActive,
		string(planChaptersJSON), selectedBranch.ExpectedDeltas, "", now, now,
	); err != nil {
		return nil, fmt.Errorf("upsert active plan failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil

	return activePlan, nil
}

// GetActivePlan retrieves the active plan for a project with memory-derived consumption status
// and keyword overlap divergence checking against committed chapters.
func (s *SQLiteStore) GetActivePlan(ctx context.Context, projectID string) (*domain.ActivePlan, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT project_id, forecast_id, branch_id, plan_text, status,
		       plan_chapters_json, expected_deltas, warning_message, created_at, updated_at
		FROM project_active_plan
		WHERE project_id = ?`, projectID)

	var plan domain.ActivePlan
	var chaptersJSON string
	if err := row.Scan(
		&plan.ProjectID, &plan.ForecastID, &plan.BranchID, &plan.PlanText, &plan.Status,
		&chaptersJSON, &plan.ExpectedDeltas, &plan.WarningMessage, &plan.CreatedAt, &plan.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if chaptersJSON != "" {
		if err := json.Unmarshal([]byte(chaptersJSON), &plan.PlanChapters); err != nil {
			return nil, fmt.Errorf("decode plan chapters for active plan check failed: %w", err)
		}
	}

	// Check forecast fingerprint status
	fc, err := s.LoadForecast(ctx, projectID, plan.ForecastID)
	if err != nil {
		return nil, fmt.Errorf("load forecast for active plan check failed: %w", err)
	}

	// Read chapters for memory-derived consumed status & divergence check
	chapters, err := s.ListChapters(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list chapters for active plan check failed: %w", err)
	}

	archivedMap := make(map[int]*domain.Chapter, len(chapters))
	for _, ch := range chapters {
		archivedMap[ch.ChapterIndex] = ch
	}

	// In-memory derivation of plan_chapters consumption (does not write to DB)
	allConsumed := len(plan.PlanChapters) > 0
	hasCoveredChapter := false
	for i := range plan.PlanChapters {
		idx := plan.PlanChapters[i].ChapterIndex
		if archivedMap[idx] != nil {
			plan.PlanChapters[i].Status = domain.PlanStatusConsumed
			hasCoveredChapter = true
		} else {
			plan.PlanChapters[i].Status = domain.PlanStatusActive
			allConsumed = false
		}
	}

	// Divergence detection: check ExpectedDeltas against state_mutation_json of archived chapters
	if hasCoveredChapter {
		var actualMutations strings.Builder
		for _, pc := range plan.PlanChapters {
			if ch, ok := archivedMap[pc.ChapterIndex]; ok {
				actualMutations.WriteString(" ")
				actualMutations.WriteString(ch.StateMutation.InventoryDelta)
				actualMutations.WriteString(" ")
				actualMutations.WriteString(ch.StateMutation.PowerDelta)
				for _, m := range ch.StateMutation.CharacterMutations {
					actualMutations.WriteString(" ")
					actualMutations.WriteString(m.Name)
					actualMutations.WriteString(" ")
					actualMutations.WriteString(m.StatusDelta)
					actualMutations.WriteString(" ")
					actualMutations.WriteString(m.RelationDelta)
				}
			}
		}

		diverged, warning := checkPlanDivergence(plan.ExpectedDeltas, actualMutations.String())
		if diverged {
			plan.Status = domain.PlanStatusDiverged
			plan.WarningMessage = warning
			// Update status in own table without auto-recalculating
			_, _ = s.db.ExecContext(ctx, `
				UPDATE project_active_plan 
				SET status = ?, warning_message = ?, updated_at = ? 
				WHERE project_id = ?`,
				domain.PlanStatusDiverged, warning, time.Now(), projectID)
		}
	}

	if plan.Status != domain.PlanStatusDiverged {
		if allConsumed {
			plan.Status = domain.PlanStatusConsumed
		} else if fc != nil && fc.Status == domain.ForecastStatusStale {
			if plan.Status == domain.PlanStatusActive {
				plan.Status = domain.PlanStatusStale
				_, _ = s.db.ExecContext(ctx, `
					UPDATE project_active_plan 
					SET status = ?, updated_at = ? 
					WHERE project_id = ?`, domain.PlanStatusStale, time.Now(), projectID)
			}
		}
	}

	return &plan, nil
}

// MarkForecastStale explicitly marks a forecast and its associated active plan as stale.
func (s *SQLiteStore) MarkForecastStale(ctx context.Context, projectID, forecastID string) error {
	if _, err := s.db.ExecContext(ctx, `
		UPDATE narrative_forecasts 
		SET status = ? 
		WHERE project_id = ? AND id = ?`,
		domain.ForecastStatusStale, projectID, forecastID,
	); err != nil {
		return err
	}

	_, _ = s.db.ExecContext(ctx, `
		UPDATE project_active_plan 
		SET status = ?, updated_at = ? 
		WHERE project_id = ? AND forecast_id = ?`,
		domain.PlanStatusStale, time.Now(), projectID, forecastID,
	)
	return nil
}

// checkPlanDivergence checks if active plan's ExpectedDeltas diverges from the state_mutation_json of archived chapters.
// Rule:
// 1. Extract significant keywords from ExpectedDeltas (>=2 characters, filtered for punctuation).
// 2. If expected keywords is non-empty, match against actual state mutations committed in canon.
// 3. If keyword overlap ratio is lower than 20% threshold, story mutations diverged from plan expectations.
// 4. Returns diverged=true and author-facing warning message.
func checkPlanDivergence(expectedDeltas string, actualMutations string) (bool, string) {
	expectedWords := extractKeywords(expectedDeltas)
	if len(expectedWords) == 0 {
		return false, ""
	}

	matched := 0
	actualLower := strings.ToLower(actualMutations)
	for _, w := range expectedWords {
		if strings.Contains(actualLower, strings.ToLower(w)) {
			matched++
		}
	}

	overlapRatio := float64(matched) / float64(len(expectedWords))
	const threshold = 0.20 // 20% overlap threshold
	if overlapRatio < threshold {
		warning := fmt.Sprintf("正史剧情变迁与计划预期严重偏离（关键词重合度 %.1f%% 低于阈值 %.1f%%），建议重新前瞻推演", overlapRatio*100, threshold*100)
		return true, warning
	}
	return false, ""
}

// extractKeywords splits text into clean semantic words/tokens, filtering out punctuation and stop chars.
func extractKeywords(text string) []string {
	clean := strings.Map(func(r rune) rune {
		switch r {
		case '，', '。', '、', '！', '？', '：', '；', '（', '）', '【', '】', '“', '”', ',', '.', '!', '?', ':', ';', '(', ')', '[', ']', '+', '-', '*', '/', '\n', '\r', '\t', ' ':
			return ' '
		default:
			return r
		}
	}, text)

	fields := strings.Fields(clean)
	var keywords []string
	seen := make(map[string]bool)
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if len([]rune(f)) >= 2 && !seen[f] {
			seen[f] = true
			keywords = append(keywords, f)
		}
	}
	return keywords
}
