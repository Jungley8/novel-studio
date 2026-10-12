package engine

import (
	"strings"
	"testing"
)

func TestUnmarshalJSONArray(t *testing.T) {
	t.Run("bare array", func(t *testing.T) {
		got, err := UnmarshalJSONArray[rawIntentCard](`[{"chapter_index":1,"conflict":"a"},{"chapter_index":2,"conflict":"b"}]`)
		if err != nil {
			t.Fatalf("bare array must decode: %v", err)
		}
		if len(got) != 2 || got[0].ChapterIndex != 1 || got[1].Conflict != "b" {
			t.Fatalf("unexpected decode: %+v", got)
		}
	})

	t.Run("wrapped under preferred key", func(t *testing.T) {
		got, err := UnmarshalJSONArray[rawIntentCard](`{"intents":[{"chapter_index":1,"conflict":"a"}]}`)
		if err != nil {
			t.Fatalf("wrapped array must decode: %v", err)
		}
		if len(got) != 1 || got[0].Conflict != "a" {
			t.Fatalf("unexpected decode: %+v", got)
		}
	})

	t.Run("wrapped under unknown key", func(t *testing.T) {
		got, err := UnmarshalJSONArray[rawIntentCard](`{"chapter_intent_cards":[{"chapter_index":7,"conflict":"x"}]}`)
		if err != nil {
			t.Fatalf("any-key wrapped array must decode: %v", err)
		}
		if len(got) != 1 || got[0].ChapterIndex != 7 {
			t.Fatalf("unexpected decode: %+v", got)
		}
	})

	t.Run("preferred key wins over unrelated array", func(t *testing.T) {
		got, err := UnmarshalJSONArray[rawIntentCard](`{"notes":[{"chapter_index":99,"conflict":"noise"}],"intents":[{"chapter_index":1,"conflict":"pick"}]}`)
		if err != nil {
			t.Fatalf("must decode: %v", err)
		}
		if len(got) != 1 || got[0].Conflict != "pick" {
			t.Fatalf("expected the preferred key payload, got %+v", got)
		}
	})

	t.Run("numeric keyed map ordered", func(t *testing.T) {
		got, err := UnmarshalJSONArray[rawIntentCard](`{"2":{"chapter_index":2,"conflict":"b"},"1":{"chapter_index":1,"conflict":"a"}}`)
		if err != nil {
			t.Fatalf("numeric map must decode: %v", err)
		}
		if len(got) != 2 || got[0].ChapterIndex != 1 || got[1].ChapterIndex != 2 {
			t.Fatalf("expected numeric ordering, got %+v", got)
		}
	})

	t.Run("empty wrapped array is a valid no-items result", func(t *testing.T) {
		got, err := UnmarshalJSONArray[rawIntentCard](`{"hooks":[]}`)
		if err != nil {
			t.Fatalf("empty array must not error: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected empty, got %+v", got)
		}
	})

	t.Run("unlocatable payload errors", func(t *testing.T) {
		_, err := UnmarshalJSONArray[rawIntentCard](`{"foo":42}`)
		if err == nil {
			t.Fatal("expected error for payload without any array")
		}
		if !strings.Contains(err.Error(), "could not locate") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
