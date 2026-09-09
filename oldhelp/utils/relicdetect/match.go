package relicdetect

import (
	"os"
	"sort"
	"strings"

	"github.com/gjrud/warframe-helper/utils/pricechecker"
)

type MatchResult struct {
	InputName   string
	Matched     bool
	Approximate bool
	Item        pricechecker.GameObject
}

type normItem struct {
	norm string
	item pricechecker.GameObject
}

type PreparedItems struct {
	items []normItem
}

func PrepareItems(gameObjects []pricechecker.GameObject) PreparedItems {
	items := make([]normItem, len(gameObjects))
	for i, obj := range gameObjects {
		items[i] = normItem{norm: normalize(obj.Name), item: obj}
	}
	return PreparedItems{items: items}
}

func MatchNames(names []string, prepared PreparedItems) []MatchResult {
	results := make([]MatchResult, len(names))
	for i, name := range names {
		norm := normalize(name)
		var best pricechecker.GameObject
		bestDist := 4
		matched := false
		approximate := false

		for _, ni := range prepared.items {
			if ni.norm == norm {
				best = ni.item
				matched = true
				approximate = false
				bestDist = 0
				break
			}
			d := levenshtein(norm, ni.norm)
			if d < bestDist {
				bestDist = d
				best = ni.item
				matched = true
				approximate = true
			}
		}
		results[i] = MatchResult{InputName: name, Matched: matched, Approximate: approximate, Item: best}
	}
	return results
}

func WriteWordList(items []pricechecker.GameObject, path string) error {
	seen := make(map[string]bool)
	var words []string
	for _, item := range items {
		for _, word := range strings.Fields(item.Name) {
			if !seen[word] {
				seen[word] = true
				words = append(words, word)
			}
		}
	}
	sort.Strings(words)
	return os.WriteFile(path, []byte(strings.Join(words, "\n")+"\n"), 0644)
}

func normalize(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				curr[j] = prev[j-1]
			} else {
				curr[j] = 1 + min(prev[j], curr[j-1], prev[j-1])
			}
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}
