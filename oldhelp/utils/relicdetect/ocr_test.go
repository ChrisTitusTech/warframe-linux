package relicdetect

import (
	"image"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPanelNameRegion(t *testing.T) {
	tests := []struct {
		name  string
		index int
		count int
		want  image.Rectangle
	}{
		{name: "single panel", index: 0, count: 1, want: image.Rect(1130, 520, 1431, 660)},
		{name: "four panels first", index: 0, count: 4, want: image.Rect(645, 520, 946, 660)},
		{name: "four panels second", index: 1, count: 4, want: image.Rect(968, 520, 1269, 660)},
		{name: "four panels third", index: 2, count: 4, want: image.Rect(1291, 520, 1592, 660)},
		{name: "four panels fourth", index: 3, count: 4, want: image.Rect(1614, 520, 1915, 660)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := panelNameRegion(tt.index, tt.count); got != tt.want {
				t.Fatalf("panelNameRegion(%d, %d) = %v, want %v", tt.index, tt.count, got, tt.want)
			}
		})
	}
}

func TestLoadWordSet(t *testing.T) {
	t.Run("normalizes deduplicated words", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "words.txt")
		data := " Mesa\n\nPRIME\n  systems  \nprime\n"
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}

		got := loadWordSet(path)
		want := map[string]struct{}{
			"mesa":    {},
			"prime":   {},
			"systems": {},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("loadWordSet() = %#v, want %#v", got, want)
		}
	})

	t.Run("missing file returns nil", func(t *testing.T) {
		if got := loadWordSet(filepath.Join(t.TempDir(), "missing.txt")); got != nil {
			t.Fatalf("loadWordSet() = %#v, want nil", got)
		}
	})
}
