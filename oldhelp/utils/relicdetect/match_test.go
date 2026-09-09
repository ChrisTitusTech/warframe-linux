package relicdetect

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gjrud/warframe-helper/utils/pricechecker"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "collapses whitespace and lowercases",
			input: "  Mesa\tPRIME\n Systems  ",
			want:  "mesa prime systems",
		},
		{
			name:  "preserves punctuation",
			input: "  Mesa,\tPRIME!\n Systems?  ",
			want:  "mesa, prime! systems?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalize(tt.input); got != tt.want {
				t.Fatalf("normalize() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMatchNames(t *testing.T) {
	tests := []struct {
		name     string
		prepared PreparedItems
		input    string
		want     MatchResult
	}{
		{
			name:     "exact match",
			prepared: PrepareItems([]pricechecker.GameObject{{Name: "Mesa Prime Systems", GameRef: "mesa-systems"}}),
			input:    "  mesa   prime systems ",
			want: MatchResult{
				InputName:   "  mesa   prime systems ",
				Matched:     true,
				Approximate: false,
				Item:        pricechecker.GameObject{Name: "Mesa Prime Systems", GameRef: "mesa-systems"},
			},
		},
		{
			name: "approximate match",
			prepared: PrepareItems([]pricechecker.GameObject{
				{Name: "Mesa Prime Systems", GameRef: "mesa-systems"},
				{Name: "Saryn Prime Chassis", GameRef: "saryn-chassis"},
			}),
			input: "Mesa Prime Sysems",
			want: MatchResult{
				InputName:   "Mesa Prime Sysems",
				Matched:     true,
				Approximate: true,
				Item:        pricechecker.GameObject{Name: "Mesa Prime Systems", GameRef: "mesa-systems"},
			},
		},
		{
			name:     "cutoff boundary stays unmatched",
			prepared: PrepareItems([]pricechecker.GameObject{{Name: "abcd", GameRef: "abcd"}}),
			input:    "wxyz",
			want: MatchResult{
				InputName: "wxyz",
			},
		},
		{
			name: "tie break uses first prepared item",
			prepared: PrepareItems([]pricechecker.GameObject{
				{Name: "cat", GameRef: "first"},
				{Name: "cut", GameRef: "second"},
			}),
			input: "cot",
			want: MatchResult{
				InputName:   "cot",
				Matched:     true,
				Approximate: true,
				Item:        pricechecker.GameObject{Name: "cat", GameRef: "first"},
			},
		},
		{
			name:     "unmatched input",
			prepared: PrepareItems([]pricechecker.GameObject{{Name: "Mesa Prime Systems", GameRef: "mesa-systems"}}),
			input:    "Completely Different Item",
			want: MatchResult{
				InputName: "Completely Different Item",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchNames([]string{tt.input}, tt.prepared)
			if len(got) != 1 {
				t.Fatalf("MatchNames() returned %d results, want 1", len(got))
			}
			if !reflect.DeepEqual(got[0], tt.want) {
				t.Fatalf("MatchNames() = %#v, want %#v", got[0], tt.want)
			}
		})
	}
}

func TestWriteWordList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "words.txt")
	items := []pricechecker.GameObject{
		{Name: "Mesa Prime Systems"},
		{Name: "Prime Chassis"},
		{Name: "Mesa Blueprint"},
	}

	if err := WriteWordList(items, path); err != nil {
		t.Fatalf("WriteWordList() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	want := "Blueprint\nChassis\nMesa\nPrime\nSystems\n"
	if string(got) != want {
		t.Fatalf("WriteWordList() wrote %q, want %q", string(got), want)
	}
}
