package pricechecker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func setUserHomeEnv(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func withPricecheckerHTTPSeams(t *testing.T, client *http.Client, baseURL string) {
	t.Helper()
	origClient := httpClient
	origBaseURL := statsBaseURL
	httpClient = client
	statsBaseURL = baseURL
	t.Cleanup(func() {
		httpClient = origClient
		statsBaseURL = origBaseURL
	})
}

func writeInventoryFixture(t *testing.T, homeDir string, body string) {
	t.Helper()
	dir := filepath.Join(homeDir, ".warframe-helper")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	path := filepath.Join(dir, "inventory.json")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func TestNetRecipes(t *testing.T) {
	recipes := []inventoryEntry{
		{ItemType: "alpha", ItemCount: 2},
		{ItemType: "beta", ItemCount: 1},
		{ItemType: "alpha", ItemCount: 1},
		{ItemType: "", ItemCount: 99},
	}
	pending := []inventoryEntry{
		{ItemType: "alpha", ItemCount: 1},
		{ItemType: "gamma", ItemCount: 1},
		{ItemType: "", ItemCount: 99},
	}

	got := netRecipes(recipes, pending)
	want := map[string]int{
		"alpha": 2,
		"beta":  1,
		"gamma": -1,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("netRecipes() = %#v, want %#v", got, want)
	}
}

func TestFilterOwned(t *testing.T) {
	tradable := []GameObject{
		{Name: "Alpha", GameRef: "ref-a"},
		{Name: "Beta", GameRef: "ref-b"},
		{Name: "Gamma", GameRef: "ref-c"},
	}

	tests := []struct {
		name  string
		owned []string
		want  []GameObject
	}{
		{
			name:  "keeps owned order and skips missing refs",
			owned: []string{"ref-b", "missing", "ref-a"},
			want: []GameObject{
				{Name: "Beta", GameRef: "ref-b"},
				{Name: "Alpha", GameRef: "ref-a"},
			},
		},
		{
			name:  "preserves duplicate refs",
			owned: []string{"ref-a", "ref-a", "ref-b"},
			want: []GameObject{
				{Name: "Alpha", GameRef: "ref-a"},
				{Name: "Alpha", GameRef: "ref-a"},
				{Name: "Beta", GameRef: "ref-b"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterOwned(tt.owned, tradable)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("FilterOwned() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSortResults(t *testing.T) {
	items := []OutputItem{
		{Name: "failed", AvgPrice: 100, Count: 10, FetchFailed: true},
		{Name: "low", AvgPrice: 10, Count: 2},
		{Name: "high", AvgPrice: 15, Count: 3},
		{Name: "mid", AvgPrice: 8, Count: 3},
	}

	SortResults(items)

	got := []string{items[0].Name, items[1].Name, items[2].Name, items[3].Name}
	want := []string{"high", "mid", "low", "failed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SortResults() order = %#v, want %#v", got, want)
	}
}

func TestSortDucatResults(t *testing.T) {
	items := []OutputItem{
		{Name: "failed", Ducats: 500, AvgPrice: 1, FetchFailed: true},
		{Name: "best", Ducats: 45, AvgPrice: 5},
		{Name: "mid", Ducats: 20, AvgPrice: 4},
		{Name: "zero", Ducats: 100, AvgPrice: 0},
	}

	SortDucatResults(items)

	got := []string{items[0].Name, items[1].Name, items[2].Name, items[3].Name}
	want := []string{"best", "mid", "zero", "failed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SortDucatResults() order = %#v, want %#v", got, want)
	}
}

func TestReadInventory(t *testing.T) {
	t.Run("valid inventory combines categories and net recipes", func(t *testing.T) {
		homeDir := t.TempDir()
		setUserHomeEnv(t, homeDir)
		writeInventoryFixture(t, homeDir, `{
			"RawUpgrades": [
				{"ItemType": "raw_alpha", "ItemCount": 1},
				{"ItemType": "raw_alpha", "ItemCount": 2}
			],
			"MiscItems": [
				{"ItemType": "misc_beta", "ItemCount": 1}
			],
			"Recipes": [
				{"ItemType": "recipe_gamma", "ItemCount": 2},
				{"ItemType": "recipe_zero", "ItemCount": 1}
			],
			"PendingRecipes": [
				{"ItemType": "recipe_gamma", "ItemCount": 1},
				{"ItemType": "recipe_zero", "ItemCount": 1},
				{"ItemType": "recipe_negative", "ItemCount": 1}
			]
		}`)

		got, err := ReadInventory()
		if err != nil {
			t.Fatalf("ReadInventory() error = %v", err)
		}

		want := []string{"raw_alpha", "misc_beta", "recipe_gamma"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ReadInventory() = %#v, want %#v", got, want)
		}
	})

	t.Run("deduplicates refs across inventory categories", func(t *testing.T) {
		homeDir := t.TempDir()
		setUserHomeEnv(t, homeDir)
		writeInventoryFixture(t, homeDir, `{
			"RawUpgrades": [
				{"ItemType": "shared_ref", "ItemCount": 1}
			],
			"MiscItems": [
				{"ItemType": "shared_ref", "ItemCount": 3},
				{"ItemType": "misc_only", "ItemCount": 1}
			],
			"Recipes": [
				{"ItemType": "shared_ref", "ItemCount": 2},
				{"ItemType": "recipe_only", "ItemCount": 1}
			],
			"PendingRecipes": [
				{"ItemType": "shared_ref", "ItemCount": 1}
			]
		}`)

		got, err := ReadInventory()
		if err != nil {
			t.Fatalf("ReadInventory() error = %v", err)
		}

		want := []string{"shared_ref", "misc_only", "recipe_only"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ReadInventory() = %#v, want %#v", got, want)
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		setUserHomeEnv(t, t.TempDir())

		got, err := ReadInventory()
		if err == nil {
			t.Fatalf("ReadInventory() error = nil, want non-nil, got %#v", got)
		}
	})

	t.Run("malformed json returns error", func(t *testing.T) {
		homeDir := t.TempDir()
		setUserHomeEnv(t, homeDir)
		writeInventoryFixture(t, homeDir, `{not valid json}`)

		got, err := ReadInventory()
		if err == nil {
			t.Fatalf("ReadInventory() error = nil, want non-nil, got %#v", got)
		}
	})
}

func TestFetchStats(t *testing.T) {
	t.Run("empty slug returns error", func(t *testing.T) {
		avg, count, err := fetchStats(context.Background(), "")
		if err == nil || err.Error() != "empty slug" {
			t.Fatalf("fetchStats() error = %v, want empty slug", err)
		}
		if avg != 0 || count != 0 {
			t.Fatalf("fetchStats() = (%v, %v), want (0, 0)", avg, count)
		}
	})

	t.Run("success path filters ranked entries and verifies request path", func(t *testing.T) {
		const slug = "arcane-energize"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Fatalf("method = %s, want GET", r.Method)
			}
			wantPath := "/v1/items/" + slug + "/statistics"
			if r.URL.Path != wantPath {
				t.Fatalf("path = %s, want %s", r.URL.Path, wantPath)
			}
			_, _ = io.WriteString(w, `{
				"payload": {
					"statistics_closed": {
						"48hours": [
							{"avg_price": 10, "volume": 2, "mod_rank": 0},
							{"avg_price": 999, "volume": 100, "mod_rank": 5},
							{"avg_price": 20, "volume": 4, "mod_rank": 0}
						]
					}
				}
			}`)
		}))
		defer server.Close()

		withPricecheckerHTTPSeams(t, server.Client(), server.URL)

		avg, count, err := fetchStats(context.Background(), slug)
		if err != nil {
			t.Fatalf("fetchStats() error = %v", err)
		}
		if avg != 15 {
			t.Fatalf("fetchStats() avg = %v, want 15", avg)
		}
		if count != 106 {
			t.Fatalf("fetchStats() count = %v, want 106", count)
		}
	})

	t.Run("all ranked entries filtered out preserves zero average and volume", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{
				"payload": {
					"statistics_closed": {
						"48hours": [
							{"avg_price": 10, "volume": 2, "mod_rank": 2},
							{"avg_price": 20, "volume": 3, "mod_rank": 3}
						]
					}
				}
			}`)
		}))
		defer server.Close()

		withPricecheckerHTTPSeams(t, server.Client(), server.URL)

		avg, count, err := fetchStats(context.Background(), "ranked-only")
		if err != nil {
			t.Fatalf("fetchStats() error = %v", err)
		}
		if avg != 0 {
			t.Fatalf("fetchStats() avg = %v, want 0", avg)
		}
		if count != 5 {
			t.Fatalf("fetchStats() count = %v, want 5", count)
		}
	})

	t.Run("missing 48hours returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"payload":{"statistics_closed":{"90days":[]}}}`)
		}))
		defer server.Close()

		withPricecheckerHTTPSeams(t, server.Client(), server.URL)

		_, _, err := fetchStats(context.Background(), "missing-bucket")
		if err == nil || err.Error() != "no 48hours stats for missing-bucket" {
			t.Fatalf("fetchStats() error = %v, want no 48hours stats", err)
		}
	})

	t.Run("invalid json returns decode error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{invalid`)
		}))
		defer server.Close()

		withPricecheckerHTTPSeams(t, server.Client(), server.URL)

		_, _, err := fetchStats(context.Background(), "bad-json")
		if err == nil || !strings.Contains(err.Error(), "invalid") {
			t.Fatalf("fetchStats() error = %v, want json decode error", err)
		}
	})

	t.Run("non 200 returns status error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer server.Close()

		withPricecheckerHTTPSeams(t, server.Client(), server.URL)

		_, _, err := fetchStats(context.Background(), "bad-status")
		if err == nil || err.Error() != "status 502" {
			t.Fatalf("fetchStats() error = %v, want status 502", err)
		}
	})

	t.Run("transport error propagates", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("transport boom")
		})}
		withPricecheckerHTTPSeams(t, client, "https://example.invalid")

		_, _, err := fetchStats(context.Background(), "transport")
		if err == nil || err.Error() != "Get \"https://example.invalid/v1/items/transport/statistics\": transport boom" {
			t.Fatalf("fetchStats() error = %v, want propagated transport error", err)
		}
	})
}

func TestFetchItemPrice(t *testing.T) {
	t.Run("copies metadata and fills fetched stats", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{
				"payload": {
					"statistics_closed": {
						"48hours": [
							{"avg_price": 12, "volume": 2, "mod_rank": 0},
							{"avg_price": 18, "volume": 5, "mod_rank": 0}
						]
					}
				}
			}`)
		}))
		defer server.Close()

		withPricecheckerHTTPSeams(t, server.Client(), server.URL)

		item := GameObject{
			Name:       "Arcane Energize",
			GameRef:    "/Lotus/Upgrades/Mods/Example",
			Slug:       "arcane-energize",
			Category:   "arcanes",
			Syndicates: "Steel Meridian",
			Ducats:     100,
			Rotation:   "A",
			Chance:     12.5,
		}

		got, err := FetchItemPrice(context.Background(), item)
		if err != nil {
			t.Fatalf("FetchItemPrice() error = %v", err)
		}

		want := OutputItem{
			Name:       item.Name,
			GameRef:    item.GameRef,
			AvgPrice:   15,
			Count:      7,
			Category:   item.Category,
			Syndicates: item.Syndicates,
			Ducats:     item.Ducats,
			Rotation:   item.Rotation,
			Chance:     item.Chance,
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("FetchItemPrice() = %#v, want %#v", got, want)
		}
	})

	t.Run("fetch error returns empty item and propagates error", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("transport boom")
		})}
		withPricecheckerHTTPSeams(t, client, "https://example.invalid")

		got, err := FetchItemPrice(context.Background(), GameObject{Name: "Broken", Slug: "broken"})
		if err == nil {
			t.Fatal("FetchItemPrice() error = nil, want non-nil")
		}
		if got != (OutputItem{}) {
			t.Fatalf("FetchItemPrice() item = %#v, want zero OutputItem", got)
		}
		wantErr := "Get \"https://example.invalid/v1/items/broken/statistics\": transport boom"
		if err.Error() != wantErr {
			t.Fatalf("FetchItemPrice() error = %v, want %s", err, wantErr)
		}
	})
}
