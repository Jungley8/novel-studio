package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Jungley8/novel-studio/internal/store"
)

func TestSQLiteStore_CompactionCache(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_compcache.db")
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	projectID := "proj_compaction_1"
	cacheKey := "hash_key_12345"

	t.Run("cache miss returns empty string and nil error", func(t *testing.T) {
		val, err := s.GetCompactionCache(ctx, projectID, "missing_key")
		if err != nil {
			t.Fatalf("expected nil err, got %v", err)
		}
		if val != "" {
			t.Fatalf("expected empty string on miss, got %q", val)
		}
	})

	t.Run("roundtrip save and get", func(t *testing.T) {
		compiledText := "【压缩摘录】第一轮压缩产物。"
		if err := s.PutCompactionCache(ctx, projectID, cacheKey, compiledText); err != nil {
			t.Fatalf("PutCompactionCache failed: %v", err)
		}

		got, err := s.GetCompactionCache(ctx, projectID, cacheKey)
		if err != nil {
			t.Fatalf("GetCompactionCache failed: %v", err)
		}
		if got != compiledText {
			t.Fatalf("expected %q, got %q", compiledText, got)
		}
	})

	t.Run("upsert overwrites existing cache entry", func(t *testing.T) {
		updatedText := "【压缩摘录】更新后的覆盖产物。"
		if err := s.PutCompactionCache(ctx, projectID, cacheKey, updatedText); err != nil {
			t.Fatalf("PutCompactionCache (upsert) failed: %v", err)
		}

		got, err := s.GetCompactionCache(ctx, projectID, cacheKey)
		if err != nil {
			t.Fatalf("GetCompactionCache failed: %v", err)
		}
		if got != updatedText {
			t.Fatalf("expected updated %q, got %q", updatedText, got)
		}
	})

	t.Run("project isolation", func(t *testing.T) {
		got, err := s.GetCompactionCache(ctx, "other_project", cacheKey)
		if err != nil {
			t.Fatalf("GetCompactionCache failed: %v", err)
		}
		if got != "" {
			t.Fatalf("expected empty string for different project, got %q", got)
		}
	})
}
