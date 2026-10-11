package domain_test

import (
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestTokenizeForSearch_BasicHanBigram(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"伏笔甲", "伏笔 笔甲"},
		{"伏笔", "伏笔"},
		{"伏笔甲乙", "伏笔 笔甲 甲乙"},
	}

	for _, tc := range tests {
		got := domain.TokenizeForSearch(tc.input)
		if got != tc.expected {
			t.Errorf("TokenizeForSearch(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestTokenizeForSearch_MixedASCIIAndHan(t *testing.T) {
	got := domain.TokenizeForSearch("林凡 defeats 宗门")
	expected := "林凡 defeats 宗门"
	if got != expected {
		t.Errorf("TokenizeForSearch('林凡 defeats 宗门') = %q; want %q", got, expected)
	}

	gotCase := domain.TokenizeForSearch("LinFan DEFEATS 宗门123")
	expectedCase := "linfan defeats 宗门 123"
	if gotCase != expectedCase {
		t.Errorf("TokenizeForSearch('LinFan DEFEATS 宗门123') = %q; want %q", gotCase, expectedCase)
	}
}

func TestTokenizeForSearch_PunctuationAndWhitespace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"，。？！；：、  \t\n", ""},
		{"伏笔甲，剑道！", "伏笔 笔甲 剑道"},
		{"-=[hello]!=-", "hello"},
	}

	for _, tc := range tests {
		got := domain.TokenizeForSearch(tc.input)
		if got != tc.expected {
			t.Errorf("TokenizeForSearch(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestTokenizeForSearch_SingleHan(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"甲", "甲"},
		{"甲 乙", "甲 乙"},
		{"道", "道"},
	}

	for _, tc := range tests {
		got := domain.TokenizeForSearch(tc.input)
		if got != tc.expected {
			t.Errorf("TokenizeForSearch(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestTokenizeForSearch_Empty(t *testing.T) {
	tests := []string{
		"",
		"   ",
		"\t\n\r",
	}

	for _, input := range tests {
		got := domain.TokenizeForSearch(input)
		if got != "" {
			t.Errorf("TokenizeForSearch(%q) = %q; want empty string", input, got)
		}
	}
}

func TestTokenizeForSearch_Homology(t *testing.T) {
	rawDoc := "第十章：林凡在此埋下伏笔甲，待三月后宗门大比揭晓。"
	query := "伏笔"

	docTokens := strings.Fields(domain.TokenizeForSearch(rawDoc))
	queryTokens := strings.Fields(domain.TokenizeForSearch(query))

	if len(queryTokens) != 1 || queryTokens[0] != "伏笔" {
		t.Fatalf("unexpected query tokens: %v", queryTokens)
	}

	found := false
	for _, dt := range docTokens {
		if dt == queryTokens[0] {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected query token %q to be in doc tokens %v", queryTokens[0], docTokens)
	}
}
