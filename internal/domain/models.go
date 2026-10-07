package domain

import (
	"errors"
	"strings"
	"time"
)

// HookStatus defines the lifecycle status of a plot foreshadowing / hook.
type HookStatus string

const (
	HookStatusOpen       HookStatus = "OPEN"
	HookStatusFermenting HookStatus = "FERMENTING"
	HookStatusResolved   HookStatus = "RESOLVED"
	HookStatusAbandoned  HookStatus = "ABANDONED"
)

// LevelTransition records an individual cultivation / power breakthrough event.
type LevelTransition struct {
	FromRealm string    `json:"from_realm"`
	ToRealm   string    `json:"to_realm"`
	Chapter   int       `json:"chapter,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// PowerLevel represents a structured cultivation realm and progression history.
type PowerLevel struct {
	Realm    string            `json:"realm"`
	SubLevel int               `json:"sub_level"`
	History  []LevelTransition `json:"history,omitempty"`
}

// InventoryItem represents a structured inventory entry with quantity and tier.
type InventoryItem struct {
	Name       string `json:"name"`
	Quantity   int    `json:"quantity"`
	Quality    string `json:"quality,omitempty"`
	AcquiredAt int    `json:"acquired_at,omitempty"`
}

// Protagonist holds the structured state machine for the main character.
type Protagonist struct {
	NameAndLevel    string          `json:"name_and_level"`
	Inventory       string          `json:"inventory"`
	CoreGoal        string          `json:"core_goal"`
	HealthStatus    string          `json:"health_status"`
	StructuredLevel *PowerLevel     `json:"structured_level,omitempty"`
	StructuredItems []InventoryItem `json:"structured_items,omitempty"`
}

// Project represents a novel book project.
type Project struct {
	ID             string      `json:"id"`
	Title          string      `json:"title"`
	TargetPlatform string      `json:"target_platform"` // e.g. "番茄脑洞", "起点仙侠", "知乎盐言"
	WorldRules     string      `json:"world_rules"`
	Protagonist    Protagonist `json:"protagonist"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

func (p *Project) Validate() error {
	p.Title = strings.TrimSpace(p.Title)
	if p.Title == "" {
		return errors.New("project title cannot be empty")
	}
	p.TargetPlatform = strings.TrimSpace(p.TargetPlatform)
	if p.TargetPlatform == "" {
		p.TargetPlatform = "通用网文"
	}
	return nil
}

// SceneBeat represents an atomic narrative beat within a chapter.
type SceneBeat struct {
	Phase             string `json:"phase"`                        // e.g. "蓄力压迫", "试探下套", "绝地反杀", "章末留钩"
	Tension           int    `json:"tension"`                      // 1-10
	Action            string `json:"action"`                       // Physical action & fact
	ExpectationBroken string `json:"expectation_broken"`           // Whose expectation is broken
	ReaderEmotion     string `json:"reader_emotion,omitempty"`     // 期望读者此刻的情绪: 紧张/好奇/解气/心疼
	InfoGap           string `json:"info_gap,omitempty"`           // 信息差: 读者知角色不知 / 角色知读者不知
	HookType          string `json:"hook_type,omitempty"`          // 仅最后一拍: CLIFFHANGER/REVERSAL/MYSTERY/POWER_UP
}

// StateMutation represents expected changes after a chapter ends.
type StateMutation struct {
	InventoryDelta string `json:"inventory_delta"`
	PowerDelta     string `json:"power_delta"`
}

// PlotHook represents a foreshadowing or plot clue.
type PlotHook struct {
	ID             string     `json:"id"`
	ProjectID      string     `json:"project_id"`
	Title          string     `json:"title"`
	Details        string     `json:"details"`
	CreatedChapter int        `json:"created_chapter"`
	TargetChapter  int        `json:"target_chapter"`
	Status         HookStatus `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (h *PlotHook) Validate() error {
	h.Title = strings.TrimSpace(h.Title)
	if h.Title == "" {
		return errors.New("plot hook title cannot be empty")
	}
	if h.CreatedChapter <= 0 {
		h.CreatedChapter = 1
	}
	if h.TargetChapter < h.CreatedChapter {
		h.TargetChapter = h.CreatedChapter + 5
	}
	switch h.Status {
	case HookStatusOpen, HookStatusFermenting, HookStatusResolved, HookStatusAbandoned:
	default:
		h.Status = HookStatusOpen
	}
	return nil
}

// ReviewVerdict indicates whether a chapter draft passes the editor review.
type ReviewVerdict string

const (
	ReviewVerdictAccepted ReviewVerdict = "ACCEPTED"
	ReviewVerdictRevision ReviewVerdict = "REVISION_NEEDED"
)

// ReviewResult encapsulates the exact-body editor review output.
type ReviewResult struct {
	Verdict         ReviewVerdict `json:"verdict"`
	Score           int           `json:"score"` // 1-100
	Issues          []string      `json:"issues"`
	Suggestions     string        `json:"suggestions"`
	ResolvedHookIDs []string      `json:"resolved_hook_ids,omitempty"`
	ReviewedAt      time.Time     `json:"reviewed_at"`
}

// AuditReport unifies statistical heuristic metrics and semantic editor review verdicts into a single seam.
type AuditReport struct {
	Verdict            ReviewVerdict `json:"verdict"`
	Score              int           `json:"score"` // 1-100
	BurstinessScore    int           `json:"burstiness_score"`
	HitBannedWords     []string      `json:"hit_banned_words"`
	DialogueRatio      float64       `json:"dialogue_ratio,omitempty"`
	ParagraphVariance  int           `json:"paragraph_variance,omitempty"`
	TopRepeatedNgrams  []string      `json:"top_repeated_ngrams,omitempty"`
	ExclamationDensity float64       `json:"exclamation_density,omitempty"`
	Issues             []string      `json:"issues"`
	Suggestions        string        `json:"suggestions"`
	ResolvedHookIDs    []string      `json:"resolved_hook_ids,omitempty"`
	ReviewedAt         time.Time     `json:"reviewed_at"`
}

func (a *AuditReport) ToReviewResult() *ReviewResult {
	if a == nil {
		return nil
	}
	return &ReviewResult{
		Verdict:         a.Verdict,
		Score:           a.Score,
		Issues:          a.Issues,
		Suggestions:     a.Suggestions,
		ResolvedHookIDs: a.ResolvedHookIDs,
		ReviewedAt:      a.ReviewedAt,
	}
}

// CheckpointPhase indicates the progression stage reached in ChapterCheckpoint.
type CheckpointPhase string

const (
	CheckpointPhaseBeats     CheckpointPhase = "BEATS_DERIVED"
	CheckpointPhaseDrafted   CheckpointPhase = "DRAFTED"
	CheckpointPhaseAudited   CheckpointPhase = "AUDITED"
	CheckpointPhaseRewriting CheckpointPhase = "REWRITING"
)

// ChapterCheckpoint encapsulates an in-progress chapter generation state to survive restarts/crashes.
type ChapterCheckpoint struct {
	ProjectID     string          `json:"project_id"`
	ChapterIndex  int             `json:"chapter_index"`
	Phase         CheckpointPhase `json:"phase"`
	CoreConflict  string          `json:"core_conflict"`
	Beats         []SceneBeat     `json:"beats,omitempty"`
	StateMutation StateMutation   `json:"state_mutation,omitempty"`
	DraftText     string          `json:"draft_text,omitempty"`
	AuditReport   *AuditReport    `json:"audit_report,omitempty"`
	RewriteLoops  int             `json:"rewrite_loops"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// Chapter represents a generated or drafted chapter.
type Chapter struct {
	ID              string        `json:"id"`
	ProjectID       string        `json:"project_id"`
	ChapterIndex    int           `json:"chapter_index"`
	Title           string        `json:"title"`
	CoreConflict    string        `json:"core_conflict"`
	Beats           []SceneBeat   `json:"beats"`
	StateMutation   StateMutation `json:"state_mutation"`
	Content         string        `json:"content"`
	WordCount       int           `json:"word_count"`
	BurstinessScore int           `json:"burstiness_score"`
	LinterPassed    bool          `json:"linter_passed"`
	Review          *ReviewResult `json:"review,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
}

// LinterResult represents quality analysis of a chapter draft.
type LinterResult struct {
	BurstinessScore    int      `json:"burstiness_score"`
	HitBannedWords     []string `json:"hit_banned_words"`
	DialogueRatio      float64  `json:"dialogue_ratio"`
	ParagraphVariance  int      `json:"paragraph_variance"`
	TopRepeatedNgrams  []string `json:"top_repeated_ngrams,omitempty"`
	ExclamationDensity float64  `json:"exclamation_density"`
	Passed             bool     `json:"passed"`
	Message            string   `json:"message"`
}
