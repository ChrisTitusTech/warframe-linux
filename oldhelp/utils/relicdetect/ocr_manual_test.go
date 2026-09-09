//go:build manual

package relicdetect

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gjrud/warframe-helper/utils/pricechecker"
)

type manualOCRFixture struct {
	name     string
	expected []string
}

var manualOCRFixtures = []manualOCRFixture{
	{
		name: "20260410221116_1.jpg",
		expected: []string{
			"okina prime blade",
			"forma blueprint",
			"kavasa prime buckle",
			"velox prime blueprint",
		},
	},
	{
		name: "20260410221536_1.jpg",
		expected: []string{
			"vadarya prime barrel",
			"forma blueprint",
			"sevagoth prime systems blueprint",
		},
	},
	{
		name: "20260410225820_1.jpg",
		expected: []string{
			"daikyu prime string",
			"paris prime upper limb",
			"guandao prime blueprint",
			"sevagoth prime systems blueprint",
		},
	},
}

func manualOCRFixturePath(name string) string {
	return filepath.Join("..", "..", "testdata", "relicdetect", name)
}

func loadManualOCRImage(t *testing.T, name string) image.Image {
	t.Helper()
	path := manualOCRFixturePath(name)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("os.Open(%q) error = %v", path, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("image.Decode(%q) error = %v", path, err)
	}
	return img
}

func manualOCRWordList(t *testing.T, names []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "words.txt")
	seen := map[string]bool{}
	var words []string
	for _, name := range names {
		for _, word := range strings.Fields(name) {
			word = strings.ToUpper(word)
			if seen[word] {
				continue
			}
			seen[word] = true
			words = append(words, word)
		}
	}
	sort.Strings(words)
	if err := os.WriteFile(path, []byte(strings.Join(words, "\n")+"\n"), 0644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return path
}

func manualPreparedItems(names []string) PreparedItems {
	items := make([]pricechecker.GameObject, len(names))
	for i, name := range names {
		items[i] = pricechecker.GameObject{Name: name, GameRef: name}
	}
	return PrepareItems(items)
}

func assertManualMatch(t *testing.T, fixtureName string, slot int, got, want string) {
	t.Helper()
	results := MatchNames([]string{got}, manualPreparedItems([]string{want}))
	if len(results) != 1 {
		t.Fatalf("MatchNames(%s, slot=%d) returned %d results, want 1", fixtureName, slot, len(results))
	}
	if !results[0].Matched || normalize(results[0].Item.Name) != normalize(want) {
		t.Fatalf("ocrRegion(%s, slot=%d) = %q, matched %#v, want %q", fixtureName, slot, got, results[0], want)
	}
	if normalize(got) != normalize(want) {
		t.Logf("ocrRegion(%s, slot=%d) raw OCR = %q, accepted via production matching to %q", fixtureName, slot, got, want)
	}
}

func assertManualMatches(t *testing.T, fixtureName string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ExtractItemNames(%s) returned %d names, want %d: %#v", fixtureName, len(got), len(want), got)
	}
	results := MatchNames(got, manualPreparedItems(want))
	if len(results) != len(want) {
		t.Fatalf("MatchNames(%s) returned %d results, want %d", fixtureName, len(results), len(want))
	}
	for i := range results {
		if !results[i].Matched || normalize(results[i].Item.Name) != normalize(want[i]) {
			t.Fatalf("ExtractItemNames(%s)[%d] = %q, matched %#v, want %q", fixtureName, i, got[i], results[i], want[i])
		}
		if normalize(got[i]) != normalize(want[i]) {
			t.Logf("ExtractItemNames(%s)[%d] raw OCR = %q, accepted via production matching to %q", fixtureName, i, got[i], want[i])
		}
	}
}

func TestManualOCRRegion(t *testing.T) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Fatalf("tesseract not installed or not on PATH: %v", err)
	}
	for _, fixture := range manualOCRFixtures {
		img := loadManualOCRImage(t, fixture.name)
		wordList := manualOCRWordList(t, fixture.expected)
		knownWords := loadWordSet(wordList)
		for i, want := range fixture.expected {
			region := panelNameRegion(i, len(fixture.expected))
			cropped := scaleAndCropImage(img, region)
			got, err := ocrRegion(cropped, wordList, knownWords)
			if err != nil {
				t.Fatalf("ocrRegion(%s, slot=%d) error = %v", fixture.name, i, err)
			}
			assertManualMatch(t, fixture.name, i, got, want)
		}
	}
}

func TestManualExtractItemNames(t *testing.T) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Fatalf("tesseract not installed or not on PATH: %v", err)
	}
	for _, fixture := range manualOCRFixtures {
		img := loadManualOCRImage(t, fixture.name)
		wordList := manualOCRWordList(t, fixture.expected)
		got, err := ExtractItemNames(img, wordList)
		if err != nil {
			t.Fatalf("ExtractItemNames(%s) error = %v", fixture.name, err)
		}
		assertManualMatches(t, fixture.name, got, fixture.expected)
	}
}
