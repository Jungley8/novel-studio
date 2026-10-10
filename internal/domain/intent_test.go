package domain

import (
	"testing"
	"time"
)

func TestChapterIntent_Validate(t *testing.T) {
	tests := []struct {
		name    string
		intent  ChapterIntent
		wantErr bool
	}{
		{
			name: "valid draft intent",
			intent: ChapterIntent{
				ProjectID:     "proj-1",
				ChapterIndex:  1,
				VolumeIndex:   1,
				Conflict:      "主角被困试炼崖底，遭遇宿敌搜山",
				POV:           "主角",
				TensionTarget: 7,
				Status:        IntentStatusDraft,
			},
			wantErr: false,
		},
		{
			name: "valid locked intent",
			intent: ChapterIntent{
				ProjectID:     "proj-1",
				ChapterIndex:  2,
				VolumeIndex:   1,
				Conflict:      "反杀追兵并缴获密令",
				POV:           "主角",
				TensionTarget: 9,
				Status:        IntentStatusLocked,
			},
			wantErr: false,
		},
		{
			name: "valid stale intent",
			intent: ChapterIntent{
				ProjectID:     "proj-1",
				ChapterIndex:  3,
				VolumeIndex:   1,
				Conflict:      "休整疗伤发现心法残篇",
				POV:           "主角",
				TensionTarget: 4,
				Status:        IntentStatusStale,
			},
			wantErr: false,
		},
		{
			name: "invalid chapter index zero",
			intent: ChapterIntent{
				ChapterIndex:  0,
				Conflict:      "冲突描述",
				TensionTarget: 5,
				Status:        IntentStatusDraft,
			},
			wantErr: true,
		},
		{
			name: "invalid chapter index negative",
			intent: ChapterIntent{
				ChapterIndex:  -5,
				Conflict:      "冲突描述",
				TensionTarget: 5,
				Status:        IntentStatusDraft,
			},
			wantErr: true,
		},
		{
			name: "invalid empty conflict",
			intent: ChapterIntent{
				ChapterIndex:  1,
				Conflict:      "",
				TensionTarget: 5,
				Status:        IntentStatusDraft,
			},
			wantErr: true,
		},
		{
			name: "invalid whitespace only conflict",
			intent: ChapterIntent{
				ChapterIndex:  1,
				Conflict:      "   \t\n  ",
				TensionTarget: 5,
				Status:        IntentStatusDraft,
			},
			wantErr: true,
		},
		{
			name: "invalid tension target zero",
			intent: ChapterIntent{
				ChapterIndex:  1,
				Conflict:      "冲突描述",
				TensionTarget: 0,
				Status:        IntentStatusDraft,
			},
			wantErr: true,
		},
		{
			name: "invalid tension target greater than ten",
			intent: ChapterIntent{
				ChapterIndex:  1,
				Conflict:      "冲突描述",
				TensionTarget: 11,
				Status:        IntentStatusDraft,
			},
			wantErr: true,
		},
		{
			name: "invalid tension target negative",
			intent: ChapterIntent{
				ChapterIndex:  1,
				Conflict:      "冲突描述",
				TensionTarget: -1,
				Status:        IntentStatusDraft,
			},
			wantErr: true,
		},
		{
			name: "invalid empty status",
			intent: ChapterIntent{
				ChapterIndex:  1,
				Conflict:      "冲突描述",
				TensionTarget: 5,
				Status:        "",
			},
			wantErr: true,
		},
		{
			name: "invalid unknown status",
			intent: ChapterIntent{
				ChapterIndex:  1,
				Conflict:      "冲突描述",
				TensionTarget: 5,
				Status:        "ARCHIVED",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.intent.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestChapterIntent_StatusTransitions(t *testing.T) {
	// DRAFT -> LOCKED (valid intent passes)
	valid := ChapterIntent{
		ChapterIndex:  1,
		Conflict:      "有效冲突",
		TensionTarget: 6,
		Status:        IntentStatusDraft,
	}
	if err := valid.TransitionTo(IntentStatusLocked); err != nil {
		t.Fatalf("expected valid DRAFT -> LOCKED transition to succeed: %v", err)
	}
	if valid.Status != IntentStatusLocked {
		t.Errorf("expected status LOCKED, got %s", valid.Status)
	}

	// LOCKED -> STALE passes
	if err := valid.TransitionTo(IntentStatusStale); err != nil {
		t.Fatalf("expected LOCKED -> STALE transition to succeed: %v", err)
	}
	if valid.Status != IntentStatusStale {
		t.Errorf("expected status STALE, got %s", valid.Status)
	}

	// STALE -> LOCKED passes
	if err := valid.TransitionTo(IntentStatusLocked); err != nil {
		t.Fatalf("expected STALE -> LOCKED transition to succeed: %v", err)
	}
	if valid.Status != IntentStatusLocked {
		t.Errorf("expected status LOCKED, got %s", valid.Status)
	}

	// Fail-closed: invalid intent cannot transition to LOCKED
	invalid := ChapterIntent{
		ChapterIndex:  1,
		Conflict:      "", // invalid: empty conflict
		TensionTarget: 6,
		Status:        IntentStatusDraft,
	}
	if err := invalid.TransitionTo(IntentStatusLocked); err == nil {
		t.Fatal("expected fail-closed error when locking invalid intent")
	}

	// Disallowed transitions fail
	disallowedCases := []struct {
		from IntentStatus
		to   IntentStatus
	}{
		{IntentStatusDraft, IntentStatusStale},
		{IntentStatusLocked, IntentStatusDraft},
		{IntentStatusStale, IntentStatusDraft},
		{IntentStatusDraft, "UNKNOWN"},
	}

	for _, dc := range disallowedCases {
		ci := ChapterIntent{
			ChapterIndex:  1,
			Conflict:      "冲突描述",
			TensionTarget: 5,
			Status:        dc.from,
		}
		if err := ci.TransitionTo(dc.to); err == nil {
			t.Errorf("expected error for transition %s -> %s, got nil", dc.from, dc.to)
		}
	}
}

func TestResolveVolume_BoundaryCases(t *testing.T) {
	spans := []VolumeSpan{
		{VolumeIndex: 1, StartChapter: 1, EndChapter: 10},
		{VolumeIndex: 2, StartChapter: 11, EndChapter: 20},
		{VolumeIndex: 3, StartChapter: 21, EndChapter: 30},
	}

	// Case 1: 章恰在边界 (exact boundaries of spans)
	boundaryTests := []struct {
		chapter int
		want    int
	}{
		{chapter: 1, want: 1},  // start of volume 1
		{chapter: 10, want: 1}, // end of volume 1
		{chapter: 11, want: 2}, // start of volume 2
		{chapter: 20, want: 2}, // end of volume 2
		{chapter: 21, want: 3}, // start of volume 3
		{chapter: 30, want: 3}, // end of volume 3
	}

	for _, tt := range boundaryTests {
		got := ResolveVolume(tt.chapter, spans)
		if got != tt.want {
			t.Errorf("ResolveVolume(%d) = %d, want %d (boundary case)", tt.chapter, got, tt.want)
		}
	}
}

func TestResolveVolume_GapCases(t *testing.T) {
	// Case 2: 章在缝隙 (chapters falling in gaps between non-contiguous spans or out-of-bounds)
	spansWithGaps := []VolumeSpan{
		{VolumeIndex: 1, StartChapter: 1, EndChapter: 10},
		{VolumeIndex: 2, StartChapter: 15, EndChapter: 20},
	}

	gapTests := []struct {
		chapter int
		want    int
	}{
		{chapter: 11, want: -1}, // gap between vol 1 and vol 2
		{chapter: 12, want: -1}, // gap
		{chapter: 14, want: -1}, // gap right before vol 2 start
		{chapter: 0, want: -1},  // before first chapter
		{chapter: -1, want: -1}, // negative chapter
		{chapter: 21, want: -1}, // past last volume (never guess last volume)
		{chapter: 100, want: -1},
	}

	for _, tt := range gapTests {
		got := ResolveVolume(tt.chapter, spansWithGaps)
		if got != tt.want {
			t.Errorf("ResolveVolume(%d) = %d, want %d (gap case)", tt.chapter, got, tt.want)
		}
	}
}

func TestResolveVolume_EmptySpans(t *testing.T) {
	// Case 3: 空表 (empty or nil span list)
	if got := ResolveVolume(1, nil); got != -1 {
		t.Errorf("ResolveVolume(1, nil) = %d, want -1 (empty table)", got)
	}

	if got := ResolveVolume(1, []VolumeSpan{}); got != -1 {
		t.Errorf("ResolveVolume(1, []VolumeSpan{}) = %d, want -1 (empty table)", got)
	}
}

func TestMarkStaleOnVolumeChange_OnlyTargetVolume(t *testing.T) {
	now := time.Now()
	input := []ChapterIntent{
		{ProjectID: "p1", ChapterIndex: 1, VolumeIndex: 1, Conflict: "c1", TensionTarget: 5, Status: IntentStatusLocked, UpdatedAt: now},
		{ProjectID: "p1", ChapterIndex: 2, VolumeIndex: 1, Conflict: "c2", TensionTarget: 5, Status: IntentStatusDraft, UpdatedAt: now},
		{ProjectID: "p1", ChapterIndex: 3, VolumeIndex: 1, Conflict: "c3", TensionTarget: 5, Status: IntentStatusStale, UpdatedAt: now},
		{ProjectID: "p1", ChapterIndex: 4, VolumeIndex: 2, Conflict: "c4", TensionTarget: 6, Status: IntentStatusLocked, UpdatedAt: now},
		{ProjectID: "p1", ChapterIndex: 5, VolumeIndex: 2, Conflict: "c5", TensionTarget: 6, Status: IntentStatusDraft, UpdatedAt: now},
		{ProjectID: "p1", ChapterIndex: 6, VolumeIndex: 3, Conflict: "c6", TensionTarget: 7, Status: IntentStatusLocked, UpdatedAt: now},
	}

	// Change volume 1
	result := MarkStaleOnVolumeChange(input, 1)

	// Verify only volume 1 LOCKED changed to STALE
	if result[0].Status != IntentStatusStale {
		t.Errorf("expected vol 1 locked intent to become STALE, got %s", result[0].Status)
	}
	if result[1].Status != IntentStatusDraft {
		t.Errorf("expected vol 1 draft intent to remain DRAFT, got %s", result[1].Status)
	}
	if result[2].Status != IntentStatusStale {
		t.Errorf("expected vol 1 stale intent to remain STALE, got %s", result[2].Status)
	}
	if result[3].Status != IntentStatusLocked {
		t.Errorf("expected vol 2 locked intent to remain LOCKED, got %s", result[3].Status)
	}
	if result[4].Status != IntentStatusDraft {
		t.Errorf("expected vol 2 draft intent to remain DRAFT, got %s", result[4].Status)
	}
	if result[5].Status != IntentStatusLocked {
		t.Errorf("expected vol 3 locked intent to remain LOCKED, got %s", result[5].Status)
	}

	// Pure function check: original input slice must not be modified
	if input[0].Status != IntentStatusLocked {
		t.Errorf("pure function violated: input[0].Status was modified to %s", input[0].Status)
	}

	// Nil / empty checks
	if got := MarkStaleOnVolumeChange(nil, 1); got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}
	if got := MarkStaleOnVolumeChange([]ChapterIntent{}, 1); len(got) != 0 {
		t.Errorf("expected empty slice for empty input, got %v", got)
	}
}
