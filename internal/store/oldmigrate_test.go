package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/store"
	_ "modernc.org/sqlite"
)

// TestOldDBMigration builds a pre-ledger database file (projects/chapters
// tables only, no project_events) and proves the store migrates it in place:
// old rows stay readable and the first new write lands a ledger row.
func TestOldDBMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	schema := `
	CREATE TABLE projects (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		target_platform TEXT NOT NULL,
		world_rules TEXT,
		protagonist_json TEXT,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);
	CREATE TABLE chapters (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		chapter_index INTEGER NOT NULL,
		title TEXT NOT NULL,
		core_conflict TEXT,
		beats_json TEXT,
		state_mutation_json TEXT,
		content TEXT,
		word_count INTEGER DEFAULT 0,
		burstiness_score INTEGER DEFAULT 0,
		linter_passed BOOLEAN DEFAULT 0,
		created_at TIMESTAMP NOT NULL,
		FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE,
		UNIQUE(project_id, chapter_index)
	);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	now := time.Now()
	if _, err := db.Exec(
		`INSERT INTO projects (id, title, target_platform, world_rules, protagonist_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"proj_old", "老作品", "通用网文", "r0", `{"name_and_level":"A"}`, now, now,
	); err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
	_ = db.Close()

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("open legacy db with new store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	p, err := s.GetProject(ctx, "proj_old")
	if err != nil || p.Title != "老作品" {
		t.Fatalf("legacy project unreadable: %v %+v", err, p)
	}
	prot, err := s.GetProtagonistEntry(ctx, "proj_old")
	if err != nil || prot == nil || prot.Name != "A" {
		t.Fatalf("expected migrated protagonist 'A', got: %v, %+v", err, prot)
	}
	p.WorldRules = "r1"
	if err := s.SaveProject(ctx, p); err != nil {
		t.Fatalf("save on migrated db: %v", err)
	}
	list, _, err := s.ListEvents(ctx, activity.EventQuery{ProjectID: "proj_old", IncludeCheckpoint: true})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(list) != 1 || list[0].EventType != activity.EventProjectUpdated {
		t.Fatalf("expected single project.updated on migrated db, got %+v", list)
	}
}
