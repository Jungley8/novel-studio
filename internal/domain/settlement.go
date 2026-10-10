package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// SettlementEvidence links a stated mutation to verbatim draft text evidence.
type SettlementEvidence struct {
	Field        string `json:"field"` // inventory | power | character:<name> | hook:<id>
	Quote        string `json:"quote"` // 原文引用片段，≥8字
	ChapterIndex int    `json:"chapter_index"`
}

// SettlementDelta captures post-draft state machine mutations with evidentiary citations.
type SettlementDelta struct {
	InventoryDelta     string               `json:"inventory_delta"`
	PowerDelta         string               `json:"power_delta"`
	CharacterMutations []CharacterMutation  `json:"character_mutations,omitempty"`
	Evidence           []SettlementEvidence `json:"evidence,omitempty"`
}

// ToStateMutation converts the settlement delta to domain StateMutation.
func (d *SettlementDelta) ToStateMutation() StateMutation {
	return StateMutation{
		InventoryDelta:     d.InventoryDelta,
		PowerDelta:         d.PowerDelta,
		CharacterMutations: d.CharacterMutations,
	}
}

// Validate verifies that every non-empty delta has at least one valid supporting evidence.
// An evidence is valid if its quote length is >= 8 characters and, when draftText is provided,
// the quote is verbatim present in the draft text.
func (d *SettlementDelta) Validate(draftText string) error {
	trimmedDraft := strings.TrimSpace(draftText)

	// Validate quote lengths and presence in draftText first
	for i, ev := range d.Evidence {
		q := strings.TrimSpace(ev.Quote)
		if utf8.RuneCountInString(q) < 8 {
			return fmt.Errorf("settlement evidence[%d] for %q quote too short (got %d chars, need >= 8): %q",
				i, ev.Field, utf8.RuneCountInString(q), q)
		}
		if trimmedDraft != "" && !strings.Contains(trimmedDraft, q) {
			return fmt.Errorf("settlement evidence[%d] for %q quote not found in draft text: %q",
				i, ev.Field, q)
		}
	}

	// 1. InventoryDelta validation: must have at least one valid inventory evidence
	if strings.TrimSpace(d.InventoryDelta) != "" {
		hasInvEv := false
		for _, ev := range d.Evidence {
			f := strings.ToLower(strings.TrimSpace(ev.Field))
			if f == "inventory" || strings.HasPrefix(f, "inventory") || f == "物品" || f == "道具" {
				hasInvEv = true
				break
			}
		}
		if !hasInvEv {
			return errors.New("inventory_delta is non-empty but lacks required evidence (field: 'inventory')")
		}
	}

	// 2. PowerDelta validation: must have at least one valid power evidence
	if strings.TrimSpace(d.PowerDelta) != "" {
		hasPowerEv := false
		for _, ev := range d.Evidence {
			f := strings.ToLower(strings.TrimSpace(ev.Field))
			if f == "power" || strings.HasPrefix(f, "power") || f == "境界" || f == "修为" {
				hasPowerEv = true
				break
			}
		}
		if !hasPowerEv {
			return errors.New("power_delta is non-empty but lacks required evidence (field: 'power')")
		}
	}

	// 3. CharacterMutations validation: each mutation must have at least one valid character evidence
	for _, cm := range d.CharacterMutations {
		cmName := strings.TrimSpace(cm.Name)
		if cmName == "" {
			continue
		}
		if strings.TrimSpace(cm.StatusDelta) != "" || strings.TrimSpace(cm.RelationDelta) != "" {
			hasCharEv := false
			for _, ev := range d.Evidence {
				f := strings.TrimSpace(ev.Field)
				lowerF := strings.ToLower(f)
				if strings.EqualFold(lowerF, "character:"+strings.ToLower(cmName)) ||
					strings.EqualFold(f, cmName) ||
					strings.Contains(f, cmName) {
					hasCharEv = true
					break
				}
			}
			if !hasCharEv {
				return fmt.Errorf("character_mutation for %q is non-empty but lacks required evidence", cmName)
			}
		}
	}

	return nil
}
