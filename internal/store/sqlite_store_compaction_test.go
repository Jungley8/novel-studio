package store_test

import (
	"context"
	"fmt"
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

	t.Run("prune keeps most recent N per project", func(t *testing.T) {
		// 对应 store 包内 compactionCacheMaxRowsPerProject = 500。
		// 写入 503 条（key_000..key_502），超出上限 3 条。
		pruneProject := "proj_compaction_prune"
		for i := 0; i < 503; i++ {
			k := fmt.Sprintf("key_%03d", i)
			v := fmt.Sprintf("text_%03d", i)
			if err := s.PutCompactionCache(ctx, pruneProject, k, v); err != nil {
				t.Fatalf("PutCompactionCache failed for %s: %v", k, err)
			}
		}

		// key_000 / key_001 / key_002 全部返回空串（被裁）
		for _, k := range []string{"key_000", "key_001", "key_002"} {
			got, err := s.GetCompactionCache(ctx, pruneProject, k)
			if err != nil {
				t.Fatalf("GetCompactionCache failed for %s: %v", k, err)
			}
			if got != "" {
				t.Fatalf("expected %s to be pruned (empty string), got %q", k, got)
			}
		}

		// key_003 与 key_502 仍可读回（内容一致）
		for _, k := range []string{"key_003", "key_502"} {
			got, err := s.GetCompactionCache(ctx, pruneProject, k)
			if err != nil {
				t.Fatalf("GetCompactionCache failed for %s: %v", k, err)
			}
			var expected string
			if k == "key_003" {
				expected = "text_003"
			} else {
				expected = "text_502"
			}
			if got != expected {
				t.Fatalf("expected %s to be %q, got %q", k, expected, got)
			}
		}
	})

	t.Run("prune project isolation", func(t *testing.T) {
		// 对应 store 包内 compactionCacheMaxRowsPerProject = 500。
		// 另一项目先 Put 3 条 → 对主项目触发裁剪后再读 → 另一项目 3 条全在。
		isoProjectOther := "proj_iso_other"
		isoProjectMain := "proj_iso_main"

		for i := 0; i < 3; i++ {
			k := fmt.Sprintf("other_key_%03d", i)
			v := fmt.Sprintf("other_text_%03d", i)
			if err := s.PutCompactionCache(ctx, isoProjectOther, k, v); err != nil {
				t.Fatalf("PutCompactionCache for other project failed: %v", err)
			}
		}

		for i := 0; i < 503; i++ {
			k := fmt.Sprintf("main_key_%03d", i)
			v := fmt.Sprintf("main_text_%03d", i)
			if err := s.PutCompactionCache(ctx, isoProjectMain, k, v); err != nil {
				t.Fatalf("PutCompactionCache for main project failed: %v", err)
			}
		}

		for i := 0; i < 3; i++ {
			k := fmt.Sprintf("other_key_%03d", i)
			expected := fmt.Sprintf("other_text_%03d", i)
			got, err := s.GetCompactionCache(ctx, isoProjectOther, k)
			if err != nil {
				t.Fatalf("GetCompactionCache for other project %s failed: %v", k, err)
			}
			if got != expected {
				t.Fatalf("expected other project key %s to be %q, got %q", k, expected, got)
			}
		}
	})
}
