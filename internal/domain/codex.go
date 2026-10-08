package domain

import (
	"errors"
	"strings"
	"time"
)

// CodexCategory represents the category of an entity in The Codex.
type CodexCategory string

const (
	CategoryCharacter CodexCategory = "CHARACTER"
	CategoryLocation  CodexCategory = "LOCATION"
	CategoryItem      CodexCategory = "ITEM"
	CategoryLore      CodexCategory = "LORE"
	CategoryFaction   CodexCategory = "FACTION"
)

// TrackingMode defines how an entity is brought into prompt assembly.
type TrackingMode string

const (
	TrackingModeAutoMention  TrackingMode = "AUTO_MENTION"
	TrackingModeAlwaysInject TrackingMode = "ALWAYS_INJECT"
	TrackingModeManual       TrackingMode = "MANUAL"
)

// Progression captures an entity's status snapshot at or after a specific chapter.
type Progression struct {
	ID                string    `json:"id"`
	EntryID           string    `json:"entry_id"`
	ActiveFromChapter int       `json:"active_from_chapter"`
	StatePayloadJSON  string    `json:"state_payload_json"` // JSON string containing realm, gear, health, disposition
	Notes             string    `json:"notes,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// EntityRelation defines a directed relationship edge between two Codex entities.
type EntityRelation struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	SourceEntryID string    `json:"source_entry_id"`
	TargetEntryID string    `json:"target_entry_id"`
	TargetName    string    `json:"target_name,omitempty"`
	RelationType  string    `json:"relation_type"` // e.g. "NEMESIS", "ALLY", "MASTER_DISCIPLE", "KINSHIP", "CRUSH"
	Description   string    `json:"description,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// CodexEntry represents a worldbuilding entity in The Codex.
type CodexEntry struct {
	ID                 string           `json:"id"`
	ProjectID          string           `json:"project_id"`
	Category           CodexCategory    `json:"category"`
	Name               string           `json:"name"`
	Aliases            []string         `json:"aliases,omitempty"`
	ColorTag           string           `json:"color_tag,omitempty"`
	Summary            string           `json:"summary"`
	DetailsMarkdown    string           `json:"details_markdown"`
	TrackingMode       TrackingMode     `json:"tracking_mode"`
	Archetype          string           `json:"archetype,omitempty"`           // 角色定位: "PROTAGONIST" | "ANTAGONIST" | "DEUTERAGONIST" | "MENTOR" | "SUPPORTING"
	VoiceTone          string           `json:"voice_tone,omitempty"`          // 台词声口与语言风格 (如: 惜字如金、冷嘲热讽、文雅阴柔)
	CoreMotivation     string           `json:"core_motivation,omitempty"`     // 核心底层动机与软肋
	CurrentDisposition string           `json:"current_disposition,omitempty"` // 当前对主角立场态势: "HOSTILE" | "WARY" | "NEUTRAL" | "FRIENDLY" | "DEVOTED"
	Progressions       []Progression    `json:"progressions,omitempty"`
	Relations          []EntityRelation `json:"relations,omitempty"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

// ActiveProgression returns the most relevant Progression snapshot for a given chapter index.
func (e *CodexEntry) ActiveProgression(chapterIndex int) *Progression {
	var best *Progression
	for i := range e.Progressions {
		p := &e.Progressions[i]
		if p.ActiveFromChapter <= chapterIndex {
			if best == nil || p.ActiveFromChapter > best.ActiveFromChapter {
				best = p
			}
		}
	}
	return best
}

func (e *CodexEntry) Validate() error {
	e.Name = strings.TrimSpace(e.Name)
	if e.Name == "" {
		return errors.New("codex entry name cannot be empty")
	}
	if e.ProjectID == "" {
		return errors.New("codex entry project_id cannot be empty")
	}
	switch e.Category {
	case CategoryCharacter, CategoryLocation, CategoryItem, CategoryLore, CategoryFaction:
	default:
		return errors.New("invalid codex category: " + string(e.Category))
	}
	switch e.TrackingMode {
	case TrackingModeAutoMention, TrackingModeAlwaysInject, TrackingModeManual:
	default:
		e.TrackingMode = TrackingModeAutoMention
	}
	return nil
}
