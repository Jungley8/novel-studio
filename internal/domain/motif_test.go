package domain

import (
	"testing"
)

func TestSanitizeMotifs(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{name: "nil 输入返回空切片", in: nil, want: []string{}},
		{name: "trim 与去空", in: []string{"  冰冷光效 ", "", "   "}, want: []string{"冰冷光效"}},
		{name: "按序去重", in: []string{"a", "b", "a"}, want: []string{"a", "b"}},
		{name: "恰为上限通过", in: []string{"1", "2", "3", "4", "5", "6", "7", "8"}, want: []string{"1", "2", "3", "4", "5", "6", "7", "8"}},
		{name: "超过上限报错", in: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}, wantErr: true},
		{name: "去重后不超过上限", in: []string{"1", "1", "2", "2", "3", "3", "4", "4", "5"}, want: []string{"1", "2", "3", "4", "5"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SanitizeMotifs(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result: %v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("length mismatch: got %v want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("index %d mismatch: got %q want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSanitizeMotifs_InputNotMutated(t *testing.T) {
	in := []string{"  a  ", "b"}
	out, err := SanitizeMotifs(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in[0] != "  a  " {
		t.Fatalf("input slice was mutated: %q", in[0])
	}
	if len(out) != 2 || out[0] != "a" || out[1] != "b" {
		t.Fatalf("unexpected result: %v", out)
	}
}
