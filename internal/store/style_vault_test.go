package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestStyleVault_TruncationAndProjectIsolation(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_style_vault.db")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer s.Close()

	// 1. Non-existent vault returns nil, nil
	v0, err := s.GetStyleVault(ctx, "non-existent")
	if err != nil {
		t.Fatalf("GetStyleVault on non-existent failed: %v", err)
	}
	if v0 != nil {
		t.Fatalf("expected nil vault for non-existent project, got %+v", v0)
	}

	// 2. Test SetGoldenExcerpts truncation: 600 runes should be truncated to exactly 500 runes.
	// Using Chinese runes ("道" = 3 bytes each) to explicitly test rune count vs byte count.
	longChineseRuneText := strings.Repeat("道", 600)
	shortText := "寒风如刀，大地沉寂。"

	if err := s.SetGoldenExcerpts(ctx, "proj-alpha", []string{longChineseRuneText, shortText}); err != nil {
		t.Fatalf("SetGoldenExcerpts proj-alpha failed: %v", err)
	}

	vA, err := s.GetStyleVault(ctx, "proj-alpha")
	if err != nil {
		t.Fatalf("GetStyleVault proj-alpha failed: %v", err)
	}
	if vA == nil {
		t.Fatalf("expected vault for proj-alpha, got nil")
	}
	if len(vA.GoldenExcerpts) != 2 {
		t.Fatalf("expected 2 excerpts, got %d", len(vA.GoldenExcerpts))
	}
	runes0 := []rune(vA.GoldenExcerpts[0])
	if len(runes0) != 500 {
		t.Errorf("expected 500 runes for truncated excerpt, got %d runes", len(runes0))
	}
	if vA.GoldenExcerpts[1] != shortText {
		t.Errorf("expected excerpt[1] to be unchanged, got %s", vA.GoldenExcerpts[1])
	}

	// 3. Test cross-project isolation: proj-beta should still be empty
	vB, err := s.GetStyleVault(ctx, "proj-beta")
	if err != nil {
		t.Fatalf("GetStyleVault proj-beta failed: %v", err)
	}
	if vB != nil {
		t.Fatalf("expected nil vault for proj-beta, got %+v", vB)
	}

	// 4. Save vault for proj-beta and verify independent storage
	betaExcerpt := "青云峰上，落英缤纷。"
	if err := s.SaveStyleVault(ctx, &domain.StyleVault{
		ProjectID:      "proj-beta",
		GoldenExcerpts: []string{betaExcerpt},
		UpdatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveStyleVault proj-beta failed: %v", err)
	}

	vBAfter, err := s.GetStyleVault(ctx, "proj-beta")
	if err != nil {
		t.Fatalf("GetStyleVault proj-beta failed: %v", err)
	}
	if vBAfter == nil || len(vBAfter.GoldenExcerpts) != 1 || vBAfter.GoldenExcerpts[0] != betaExcerpt {
		t.Fatalf("unexpected proj-beta vault: %+v", vBAfter)
	}

	// Re-verify proj-alpha is unchanged
	vAReverify, err := s.GetStyleVault(ctx, "proj-alpha")
	if err != nil {
		t.Fatalf("re-verify proj-alpha failed: %v", err)
	}
	if len(vAReverify.GoldenExcerpts) != 2 || len([]rune(vAReverify.GoldenExcerpts[0])) != 500 {
		t.Fatalf("proj-alpha was mutated by proj-beta: %+v", vAReverify)
	}

	// 5. Fail-closed validations
	if err := s.SaveStyleVault(ctx, nil); err == nil {
		t.Errorf("expected error when saving nil vault")
	}
	if err := s.SaveStyleVault(ctx, &domain.StyleVault{ProjectID: ""}); err == nil {
		t.Errorf("expected error when saving vault with empty projectID")
	}
	if _, err := s.GetStyleVault(ctx, "  "); err == nil {
		t.Errorf("expected error when getting vault with whitespace projectID")
	}
	if err := s.SetGoldenExcerpts(ctx, "", []string{"test"}); err == nil {
		t.Errorf("expected error when SetGoldenExcerpts with empty projectID")
	}
}
