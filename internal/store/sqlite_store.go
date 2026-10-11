package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite failed: %w", err)
	}

	// Optimize for concurrency & single-file responsiveness
	if _, err := db.Exec(`PRAGMA journal_mode = WAL; PRAGMA foreign_keys = ON; PRAGMA busy_timeout = 5000;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set pragma failed: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return s, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// execer is satisfied by both *sql.DB and *sql.Tx so state mutations and
// their ledger events always share one transaction.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// ErrNonLatestChapter is returned when uncommitting a chapter that is not the latest committed chapter.
var ErrNonLatestChapter = errors.New("cannot uncommit non-latest chapter")

// NonLatestChapterError records details when attempting to uncommit a non-latest chapter.
type NonLatestChapterError struct {
	ChapterIndex int
	MaxChapter   int
	Downstream   []int
}

func (e *NonLatestChapterError) Error() string {
	var downstreamStr string
	if len(e.Downstream) == 1 {
		downstreamStr = fmt.Sprintf("%d", e.Downstream[0])
	} else if len(e.Downstream) > 1 {
		downstreamStr = fmt.Sprintf("%d..%d", e.Downstream[0], e.Downstream[len(e.Downstream)-1])
	}
	return fmt.Sprintf("cannot uncommit chapter %d: chapters %s exist; uncommit chapter %d first or rewrite from latest",
		e.ChapterIndex, downstreamStr, e.MaxChapter)
}

func (e *NonLatestChapterError) Unwrap() error {
	return ErrNonLatestChapter
}
