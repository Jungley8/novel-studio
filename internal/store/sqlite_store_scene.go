package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/domain"
)

// -------------------------------------------------------------
// The Matrix (场景场次与矩阵大纲) 存储实现
// -------------------------------------------------------------

func (s *SQLiteStore) SaveScene(ctx context.Context, sc *domain.Scene) error {
	if err := sc.Validate(); err != nil {
		return err
	}
	if sc.ID == "" {
		sc.ID = fmt.Sprintf("sc-%d", time.Now().UnixNano())
	}
	if sc.CreatedAt.IsZero() {
		sc.CreatedAt = time.Now()
	}
	sc.UpdatedAt = time.Now()

	beatsJSON, _ := json.Marshal(sc.Beats)
	stateMutJSON, _ := json.Marshal(sc.StateMutation)
	if sc.WordCount <= 0 && sc.ProseContent != "" {
		sc.WordCount = len([]rune(sc.ProseContent))
	}

	var beforeProse string
	orow := s.db.QueryRowContext(ctx, `SELECT prose_content FROM scenes WHERE id = ?`, sc.ID)
	existed := false
	if oerr := orow.Scan(&beforeProse); oerr == nil {
		existed = true
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	query := `
	INSERT INTO scenes (
		id, chapter_id, project_id, scene_index, title, location_entry_id, dramatic_goal, conflict_barrier,
		tension_level, beats_json, state_mutation_json, prose_content, word_count, exclude_from_ai, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		chapter_id = excluded.chapter_id,
		scene_index = excluded.scene_index,
		title = excluded.title,
		location_entry_id = excluded.location_entry_id,
		dramatic_goal = excluded.dramatic_goal,
		conflict_barrier = excluded.conflict_barrier,
		tension_level = excluded.tension_level,
		beats_json = excluded.beats_json,
		state_mutation_json = excluded.state_mutation_json,
		prose_content = excluded.prose_content,
		word_count = excluded.word_count,
		exclude_from_ai = excluded.exclude_from_ai,
		updated_at = excluded.updated_at;
	`
	if _, err := tx.ExecContext(ctx, query,
		sc.ID, sc.ChapterID, sc.ProjectID, sc.SceneIndex, sc.Title, sc.LocationEntryID,
		sc.DramaticGoal, sc.ConflictBarrier, sc.TensionLevel, string(beatsJSON), string(stateMutJSON),
		sc.ProseContent, sc.WordCount, sc.ExcludeFromAI, sc.CreatedAt, sc.UpdatedAt,
	); err != nil {
		return err
	}

	afterBytes, _ := json.Marshal(sc)
	title := fmt.Sprintf("场次「%s」已保存（%d 字）", sc.Title, sc.WordCount)
	if !existed {
		title = fmt.Sprintf("场次「%s」已创建", sc.Title)
	}
	e := newEvent(ctx, sc.ProjectID, activity.EntityScene, sc.ID, 0,
		activity.EventSceneUpserted, title, sc.DramaticGoal)
	withTextSnapshots(e, beforeProse, string(afterBytes), []string{"prose_content"})
	e.ExcerptBefore = activity.Excerpt(beforeProse, activity.ExcerptRunes)
	e.ExcerptAfter = activity.Excerpt(sc.ProseContent, activity.ExcerptRunes)
	if err := emitLedger(ctx, tx, e); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
}

func (s *SQLiteStore) GetScene(ctx context.Context, id string) (*domain.Scene, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, chapter_id, project_id, scene_index, title, location_entry_id, dramatic_goal, conflict_barrier,
		       tension_level, beats_json, state_mutation_json, prose_content, word_count, exclude_from_ai, created_at, updated_at
		FROM scenes WHERE id = ?
	`, id)

	var sc domain.Scene
	var title, loc, goal, barrier, prose sql.NullString
	var beatsStr, stateMutStr string
	if err := row.Scan(
		&sc.ID, &sc.ChapterID, &sc.ProjectID, &sc.SceneIndex, &title, &loc, &goal, &barrier,
		&sc.TensionLevel, &beatsStr, &stateMutStr, &prose, &sc.WordCount, &sc.ExcludeFromAI, &sc.CreatedAt, &sc.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan scene: %w", err)
	}
	sc.Title = title.String
	sc.LocationEntryID = loc.String
	sc.DramaticGoal = goal.String
	sc.ConflictBarrier = barrier.String
	sc.ProseContent = prose.String
	_ = json.Unmarshal([]byte(beatsStr), &sc.Beats)
	_ = json.Unmarshal([]byte(stateMutStr), &sc.StateMutation)

	markers, err := s.ListSceneMarkers(ctx, sc.ID)
	if err == nil {
		sc.Markers = markers
	}

	return &sc, nil
}

func (s *SQLiteStore) ListScenes(ctx context.Context, chapterID string) ([]*domain.Scene, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, chapter_id, project_id, scene_index, title, location_entry_id, dramatic_goal, conflict_barrier,
		       tension_level, beats_json, state_mutation_json, prose_content, word_count, exclude_from_ai, created_at, updated_at
		FROM scenes WHERE chapter_id = ? ORDER BY scene_index ASC
	`, chapterID)
	if err != nil {
		return nil, fmt.Errorf("list scenes: %w", err)
	}
	defer rows.Close()

	var scenes []*domain.Scene
	for rows.Next() {
		var sc domain.Scene
		var title, loc, goal, barrier, prose sql.NullString
		var beatsStr, stateMutStr string
		if err := rows.Scan(
			&sc.ID, &sc.ChapterID, &sc.ProjectID, &sc.SceneIndex, &title, &loc, &goal, &barrier,
			&sc.TensionLevel, &beatsStr, &stateMutStr, &prose, &sc.WordCount, &sc.ExcludeFromAI, &sc.CreatedAt, &sc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan scene row: %w", err)
		}
		sc.Title = title.String
		sc.LocationEntryID = loc.String
		sc.DramaticGoal = goal.String
		sc.ConflictBarrier = barrier.String
		sc.ProseContent = prose.String
		_ = json.Unmarshal([]byte(beatsStr), &sc.Beats)
		_ = json.Unmarshal([]byte(stateMutStr), &sc.StateMutation)
		scenes = append(scenes, &sc)
	}
	return scenes, nil
}

func (s *SQLiteStore) DeleteScene(ctx context.Context, id string) error {
	var beforeProse, projectID, sceneTitle string
	orow := s.db.QueryRowContext(ctx, `SELECT project_id, title, prose_content FROM scenes WHERE id = ?`, id)
	_ = orow.Scan(&projectID, &sceneTitle, &beforeProse)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `DELETE FROM scenes WHERE id = ?`, id); err != nil {
		return err
	}

	if projectID != "" {
		e := newEvent(ctx, projectID, activity.EntityScene, id, 0,
			activity.EventSceneDeleted, fmt.Sprintf("场次「%s」已删除", sceneTitle), "场次及其批注已级联删除，墓碑留存")
		withTextSnapshots(e, beforeProse, "", []string{"deleted"})
		if err := insertEvent(ctx, tx, e); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
}

func (s *SQLiteStore) SaveSceneMarker(ctx context.Context, m *domain.SceneMarker) error {
	if m.ID == "" {
		m.ID = fmt.Sprintf("marker-%d", time.Now().UnixNano())
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	query := `
	INSERT INTO scene_markers (id, scene_id, marker_type, color, text_range_start, text_range_end, content, resolved, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		marker_type = excluded.marker_type,
		color = excluded.color,
		text_range_start = excluded.text_range_start,
		text_range_end = excluded.text_range_end,
		content = excluded.content,
		resolved = excluded.resolved;
	`
	if _, err := tx.ExecContext(ctx, query,
		m.ID, m.SceneID, string(m.MarkerType), m.Color, m.TextRangeStart, m.TextRangeEnd, m.Content, m.Resolved, m.CreatedAt,
	); err != nil {
		return err
	}

	var projectID string
	_ = tx.QueryRowContext(ctx, `SELECT project_id FROM scenes WHERE id = ?`, m.SceneID).Scan(&projectID)
	if projectID != "" {
		afterBytes, _ := json.Marshal(m)
		e := newEvent(ctx, projectID, activity.EntitySceneMarker, m.ID, 0,
			activity.EventSceneMarkerAdded, fmt.Sprintf("批注 [%s] %s", m.MarkerType, m.Content), fmt.Sprintf("场次 %s · 区间 %d-%d", m.SceneID, m.TextRangeStart, m.TextRangeEnd))
		withTextSnapshots(e, "", string(afterBytes), []string{"marker"})
		if err := insertEvent(ctx, tx, e); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
}

func (s *SQLiteStore) ListSceneMarkers(ctx context.Context, sceneID string) ([]domain.SceneMarker, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, scene_id, marker_type, color, text_range_start, text_range_end, content, resolved, created_at
		FROM scene_markers WHERE scene_id = ? ORDER BY text_range_start ASC
	`, sceneID)
	if err != nil {
		return nil, fmt.Errorf("list markers: %w", err)
	}
	defer rows.Close()

	var markers []domain.SceneMarker
	for rows.Next() {
		var m domain.SceneMarker
		var mt string
		if err := rows.Scan(
			&m.ID, &m.SceneID, &mt, &m.Color, &m.TextRangeStart, &m.TextRangeEnd, &m.Content, &m.Resolved, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan marker: %w", err)
		}
		m.MarkerType = domain.MarkerType(mt)
		markers = append(markers, m)
	}
	return markers, nil
}

func (s *SQLiteStore) DeleteSceneMarker(ctx context.Context, id string) error {
	var sceneID, content, markerType string
	orow := s.db.QueryRowContext(ctx, `SELECT scene_id, marker_type, content FROM scene_markers WHERE id = ?`, id)
	_ = orow.Scan(&sceneID, &markerType, &content)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `DELETE FROM scene_markers WHERE id = ?`, id); err != nil {
		return err
	}

	var projectID string
	if sceneID != "" {
		_ = tx.QueryRowContext(ctx, `SELECT project_id FROM scenes WHERE id = ?`, sceneID).Scan(&projectID)
	}
	if projectID != "" {
		e := newEvent(ctx, projectID, activity.EntitySceneMarker, id, 0,
			activity.EventSceneMarkerDelete, fmt.Sprintf("批注 [%s] 已删除", markerType), content)
		withTextSnapshots(e, content, "", []string{"deleted"})
		if err := insertEvent(ctx, tx, e); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
}

func (s *SQLiteStore) GetMatrixOverview(ctx context.Context, projectID string) (*domain.MatrixOverview, error) {
	proj, err := s.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	chapters, err := s.ListChapters(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list chapters: %w", err)
	}

	overview := &domain.MatrixOverview{
		ProjectID: projectID,
	}

	chapterRows := make([]*domain.MatrixChapterRow, 0, len(chapters))
	totalWords := 0
	totalScenes := 0

	for _, ch := range chapters {
		scenes, err := s.ListScenes(ctx, ch.ID)
		if err != nil {
			scenes = []*domain.Scene{}
		}
		chWords := ch.WordCount
		for _, sc := range scenes {
			chWords += sc.WordCount
			totalScenes++
		}
		totalWords += chWords
		chapterRows = append(chapterRows, &domain.MatrixChapterRow{
			Chapter: ch,
			Scenes:  scenes,
		})
	}
	overview.TotalWords = totalWords
	overview.TotalScenes = totalScenes

	if proj.Framework != nil && len(proj.Framework.VolumeArcs) > 0 {
		startIdx := 1
		for _, arc := range proj.Framework.VolumeArcs {
			est := arc.EstimatedChapters
			if est <= 0 {
				est = 30
			}
			endIdx := startIdx + est - 1

			group := &domain.MatrixVolumeGroup{
				VolumeIndex: arc.VolumeIndex,
				Title:       arc.Title,
				Theme:       arc.Theme,
				CoreGoal:    arc.CoreGoal,
				Climax:      arc.Climax,
			}

			var volWords int
			var tensionSum int
			var sceneCount int

			for _, row := range chapterRows {
				if row.Chapter.ChapterIndex >= startIdx && row.Chapter.ChapterIndex <= endIdx {
					group.Chapters = append(group.Chapters, row)
					volWords += row.Chapter.WordCount
					for _, sc := range row.Scenes {
						tensionSum += sc.TensionLevel
						sceneCount++
					}
				}
			}

			group.TotalWords = volWords
			if sceneCount > 0 {
				group.AvgTension = float64(tensionSum) / float64(sceneCount)
			}
			overview.Volumes = append(overview.Volumes, group)
			startIdx = endIdx + 1
		}
	} else {
		defVol := &domain.MatrixVolumeGroup{
			VolumeIndex: 1,
			Title:       "第一卷",
			Chapters:    chapterRows,
			TotalWords:  totalWords,
		}
		overview.Volumes = append(overview.Volumes, defVol)
	}

	return overview, nil
}
