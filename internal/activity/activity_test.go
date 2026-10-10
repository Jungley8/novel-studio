package activity

import (
	"strings"
	"testing"
)

func TestEncodeSmallStaysPlain(t *testing.T) {
	blob, codec, err := Encode([]byte("短文本"))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if codec != CodecNone {
		t.Fatalf("expected codec none, got %q", codec)
	}
	out, err := Decode(blob, codec)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(out) != "短文本" {
		t.Fatalf("roundtrip mismatch: %q", out)
	}
}

func TestEncodeLargeGzips(t *testing.T) {
	big := strings.Repeat("拍卖场上灯火如昼，主角指尖轻叩桌面。", 200)
	blob, codec, err := Encode([]byte(big))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if codec != CodecGzip {
		t.Fatalf("expected codec gzip, got %q", codec)
	}
	if len(blob) >= len(big) {
		t.Fatalf("expected compression to shrink payload (%d -> %d)", len(big), len(blob))
	}
	out, err := Decode(blob, codec)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(out) != big {
		t.Fatalf("roundtrip mismatch")
	}
}

func TestEncodeEmpty(t *testing.T) {
	blob, codec, err := Encode(nil)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if codec != CodecNone || len(blob) != 0 {
		t.Fatalf("expected empty none payload, got codec=%q len=%d", codec, len(blob))
	}
}

func TestDecodeUnknownCodecRejected(t *testing.T) {
	if _, err := Decode([]byte("xxx"), "zstd"); err == nil {
		t.Fatalf("expected unknown codec to be rejected")
	}
}

func TestExcerpt(t *testing.T) {
	if got := Excerpt("abcdef", 10); got != "abcdef" {
		t.Fatalf("short text must pass through, got %q", got)
	}
	long := strings.Repeat("字", 300)
	got := Excerpt(long, ExcerptRunes)
	if len([]rune(got)) != ExcerptRunes+1 {
		t.Fatalf("expected %d runes + ellipsis, got %d", ExcerptRunes, len([]rune(got)))
	}
}

func TestCharsDelta(t *testing.T) {
	a, r := CharsDelta("abc", "abcde")
	if a != 2 || r != 0 {
		t.Fatalf("growth must count as added, got %d/%d", a, r)
	}
	a, r = CharsDelta("abcde", "ab")
	if a != 0 || r != 3 {
		t.Fatalf("shrinkage must count as removed, got %d/%d", a, r)
	}
}

func TestChangedFields(t *testing.T) {
	before := map[string]any{"a": 1, "b": "x", "c": true}
	after := map[string]any{"a": 1, "b": "y", "d": 2}
	got := ChangedFields(before, after)
	want := []string{"b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestCtxDefaults(t *testing.T) {
	if ActorFromCtx(nil) != ActorUser {
		t.Fatalf("actor must default to user")
	}
	if SourceFromCtx(nil) != SourceDirect {
		t.Fatalf("source must default to direct")
	}
	if RunIDFromCtx(nil) != "" {
		t.Fatalf("run id must default to empty")
	}
}

func TestValidateFailClosed(t *testing.T) {
	if err := (&ActivityEvent{}).Validate(); err == nil {
		t.Fatalf("empty event must be rejected")
	}
	e := &ActivityEvent{ProjectID: "p", EntityType: EntityChapter, EntityID: "c", EventType: EventChapterCommitted}
	if err := e.Validate(); err != nil {
		t.Fatalf("complete event must pass: %v", err)
	}
}
