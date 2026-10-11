package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// -------------------------------------------------------------
// Memory Retrieval Projection (FTS5 + 中文 2-gram 只读检索层)
// -------------------------------------------------------------

type memoryIndexItem struct {
	kind         string
	refID        string
	chapterIndex int
	content      string
	raw          string
}

// RebuildMemoryIndex 从真值表重建本项目的检索投影（幂等：先清后插；先读齐再开事务）。
func (s *SQLiteStore) RebuildMemoryIndex(ctx context.Context, projectID string) error {
	if strings.TrimSpace(projectID) == "" {
		return errors.New("project_id cannot be empty")
	}

	// 1. 确保源数据表存在
	if err := s.ensureSummaryTables(ctx); err != nil {
		return fmt.Errorf("ensure summary tables: %w", err)
	}

	// 2. 事务外读齐全部源数据（规避 MaxOpenConns(1) 连接池死锁）
	summaries, err := s.ListChapterSummaries(ctx, projectID, 0, 0)
	if err != nil {
		return fmt.Errorf("list chapter summaries: %w", err)
	}

	hooks, err := s.ListPlotHooks(ctx, projectID)
	if err != nil {
		return fmt.Errorf("list plot hooks: %w", err)
	}

	type retroRow struct {
		volumeIndex int
		text        string
	}
	retroRows, err := s.db.QueryContext(ctx, `SELECT volume_index, text FROM volume_retrospectives WHERE project_id = ? ORDER BY volume_index ASC;`, projectID)
	if err != nil {
		return fmt.Errorf("query volume retrospectives: %w", err)
	}
	defer retroRows.Close()

	var retros []retroRow
	for retroRows.Next() {
		var r retroRow
		if err := retroRows.Scan(&r.volumeIndex, &r.text); err != nil {
			return fmt.Errorf("scan volume retrospective: %w", err)
		}
		retros = append(retros, r)
	}
	if err := retroRows.Err(); err != nil {
		return fmt.Errorf("iterate volume retrospectives: %w", err)
	}
	retroRows.Close()

	// 3. 在内存中预构建检索项与分词
	var items []memoryIndexItem

	// 3.1 章节摘要 -> kind="summary"、ref_id="chapter:<n>"、chapter_index=摘要章号、raw=摘要文本、content=同源分词
	for _, cs := range summaries {
		raw := cs.Text
		items = append(items, memoryIndexItem{
			kind:         "summary",
			refID:        fmt.Sprintf("chapter:%d", cs.ChapterIndex),
			chapterIndex: cs.ChapterIndex,
			content:      domain.TokenizeForSearch(raw),
			raw:          raw,
		})
	}

	// 3.2 钩子 -> kind="hook"、ref_id=钩子 ID、chapter_index=埋设章、raw=标题+类型+备注、content=同源分词
	for _, h := range hooks {
		var parts []string
		if strings.TrimSpace(h.Title) != "" {
			parts = append(parts, strings.TrimSpace(h.Title))
		}
		if strings.TrimSpace(string(h.Status)) != "" {
			parts = append(parts, strings.TrimSpace(string(h.Status)))
		}
		if strings.TrimSpace(h.Details) != "" {
			parts = append(parts, strings.TrimSpace(h.Details))
		}
		raw := strings.Join(parts, " ")
		items = append(items, memoryIndexItem{
			kind:         "hook",
			refID:        h.ID,
			chapterIndex: h.CreatedChapter,
			content:      domain.TokenizeForSearch(raw),
			raw:          raw,
		})
	}

	// 3.3 卷回顾 -> kind="retro"、ref_id="volume:<volIdx>"、chapter_index=0、raw=回顾正文、content=同源分词
	for _, r := range retros {
		raw := r.text
		items = append(items, memoryIndexItem{
			kind:         "retro",
			refID:        fmt.Sprintf("volume:%d", r.volumeIndex),
			chapterIndex: 0,
			content:      domain.TokenizeForSearch(raw),
			raw:          raw,
		})
	}

	// 4. 开启事务：先清后插
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx for rebuild memory index: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM memory_fts WHERE project_id = ?;", projectID); err != nil {
		return fmt.Errorf("delete from memory_fts: %w", err)
	}

	if len(items) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO memory_fts (project_id, kind, ref_id, chapter_index, content, raw)
			VALUES (?, ?, ?, ?, ?, ?);
		`)
		if err != nil {
			return fmt.Errorf("prepare insert into memory_fts: %w", err)
		}
		defer stmt.Close()

		for _, it := range items {
			if _, err := stmt.ExecContext(ctx, projectID, it.kind, it.refID, it.chapterIndex, it.content, it.raw); err != nil {
				return fmt.Errorf("insert into memory_fts: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit rebuild memory index: %w", err)
	}
	return nil
}

// SearchMemory 以 2-gram 同源分词做 BM25 候选召回；limit 夹取 1..50（<=0 取 20）。
func (s *SQLiteStore) SearchMemory(ctx context.Context, projectID, query string, limit int) ([]*domain.MemoryHit, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("project_id cannot be empty")
	}

	tokens := strings.Fields(domain.TokenizeForSearch(query))
	if len(tokens) == 0 {
		return []*domain.MemoryHit{}, nil
	}

	if limit <= 0 {
		limit = 20
	} else if limit > 50 {
		limit = 50
	}

	quotedTokens := make([]string, len(tokens))
	for i, tok := range tokens {
		cleanTok := strings.ReplaceAll(tok, `"`, `""`)
		quotedTokens[i] = `"` + cleanTok + `"`
	}
	matchExpr := strings.Join(quotedTokens, " OR ")

	querySQL := `
	SELECT kind, ref_id, chapter_index, raw, bm25(memory_fts) AS score
	FROM memory_fts
	WHERE project_id = ? AND memory_fts MATCH ?
	ORDER BY score ASC
	LIMIT ?;
	`
	rows, err := s.db.QueryContext(ctx, querySQL, projectID, matchExpr, limit)
	if err != nil {
		return nil, fmt.Errorf("search memory: %w", err)
	}
	defer rows.Close()

	hits := make([]*domain.MemoryHit, 0)
	for rows.Next() {
		var hit domain.MemoryHit
		if err := rows.Scan(&hit.Kind, &hit.RefID, &hit.ChapterIndex, &hit.Raw, &hit.Score); err != nil {
			return nil, fmt.Errorf("scan memory hit: %w", err)
		}
		hits = append(hits, &hit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory hits: %w", err)
	}
	return hits, nil
}
