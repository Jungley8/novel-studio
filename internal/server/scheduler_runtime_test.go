package server

import (
	"context"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestNextChapterIndex(t *testing.T) {
	tests := []struct {
		name     string
		chapters []*domain.Chapter
		expected int
	}{
		{
			name:     "empty list",
			chapters: nil,
			expected: 1,
		},
		{
			name:     "empty slice",
			chapters: []*domain.Chapter{},
			expected: 1,
		},
		{
			name: "sequential chapters",
			chapters: []*domain.Chapter{
				{ChapterIndex: 1},
				{ChapterIndex: 2},
				{ChapterIndex: 3},
			},
			expected: 4,
		},
		{
			name: "out of order chapters",
			chapters: []*domain.Chapter{
				{ChapterIndex: 5},
				{ChapterIndex: 1},
				{ChapterIndex: 8},
				{ChapterIndex: 2},
			},
			expected: 9,
		},
		{
			name: "chapters with non-positive index",
			chapters: []*domain.Chapter{
				{ChapterIndex: 0},
				{ChapterIndex: -1},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextChapterIndex(tt.chapters)
			if got != tt.expected {
				t.Errorf("nextChapterIndex() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestFindArcForChapter(t *testing.T) {
	arcs := []domain.VolumeArc{
		{VolumeIndex: 1, StartChapter: 1, EndChapter: 40},
		{VolumeIndex: 2, StartChapter: 41, EndChapter: 80},
		{VolumeIndex: 3, StartChapter: 81, EndChapter: 120},
	}

	tests := []struct {
		name      string
		chapter   int
		wantVol   int
		wantFound bool
	}{
		{
			name:      "first chapter of volume 1",
			chapter:   1,
			wantVol:   1,
			wantFound: true,
		},
		{
			name:      "middle of volume 1",
			chapter:   20,
			wantVol:   1,
			wantFound: true,
		},
		{
			name:      "last chapter of volume 1",
			chapter:   40,
			wantVol:   1,
			wantFound: true,
		},
		{
			name:      "first chapter of volume 2",
			chapter:   41,
			wantVol:   2,
			wantFound: true,
		},
		{
			name:      "last chapter of volume 3",
			chapter:   120,
			wantVol:   3,
			wantFound: true,
		},
		{
			name:      "chapter past the end of all volumes",
			chapter:   121,
			wantFound: false,
		},
		{
			name:      "non-positive chapter index",
			chapter:   0,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arc := findArcForChapter(arcs, tt.chapter)
			if tt.wantFound {
				if arc == nil {
					t.Fatalf("expected arc for chapter %d, got nil", tt.chapter)
				}
				if arc.VolumeIndex != tt.wantVol {
					t.Errorf("expected volume index %d, got %d", tt.wantVol, arc.VolumeIndex)
				}
			} else {
				if arc != nil {
					t.Errorf("expected nil arc for chapter %d, got %+v", tt.chapter, arc)
				}
			}
		})
	}
}

func TestSchedulerRuntime_FailClosed(t *testing.T) {
	rt := &schedulerRuntime{
		srv: &Server{},
	}

	// 1. RunContinuation with nil workshop or store
	_, _, err := rt.RunContinuation(context.Background(), "p1", domain.QuotaKindChapters, 1, nil)
	if err == nil {
		t.Error("expected error when workshop/store is nil in RunContinuation, got nil")
	}

	// 2. RunBook with nil orch/workshop/store
	_, _, _, err = rt.RunBook(context.Background(), &domain.BookQueueItem{ID: "q1"}, nil)
	if err == nil {
		t.Error("expected error when orch/workshop/store is nil in RunBook, got nil")
	}

	// 3. RunBook with nil item
	_, _, _, err = rt.RunBook(context.Background(), nil, nil)
	if err == nil {
		t.Error("expected error when item is nil in RunBook, got nil")
	}
}
