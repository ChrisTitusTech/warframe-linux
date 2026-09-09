package pricechecker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/gjrud/warframe-helper/utils/database"
)

type itemScanner func(*sql.Rows, *GameObject) error

type GameObject struct {
	Name       string
	GameRef    string
	Slug       string
	Category   string
	Syndicates string
	Ducats     int
	Rotation   string
	Chance     float64
}

type OutputItem struct {
	Name        string
	GameRef     string
	AvgPrice    float64
	Count       int
	Category    string
	Syndicates  string
	Ducats      int
	Rotation    string
	Chance      float64
	FetchFailed bool
}

type inventoryEntry struct {
	ItemType  string `json:"ItemType"`
	ItemCount int    `json:"ItemCount"`
}

type rawInventory struct {
	RawUpgrades    []inventoryEntry `json:"RawUpgrades"`
	MiscItems      []inventoryEntry `json:"MiscItems"`
	Recipes        []inventoryEntry `json:"Recipes"`
	PendingRecipes []inventoryEntry `json:"PendingRecipes"`
}

func loadInventory() (rawInventory, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return rawInventory{}, err
	}
	path := filepath.Join(homeDir, ".warframe-helper", "inventory.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return rawInventory{}, err
	}
	var inv rawInventory
	if err := json.Unmarshal(b, &inv); err != nil {
		return rawInventory{}, err
	}
	return inv, nil
}

func netRecipes(recipes, pending []inventoryEntry) map[string]int {
	counts := make(map[string]int)
	for _, e := range recipes {
		if e.ItemType != "" {
			counts[e.ItemType] += e.ItemCount
		}
	}
	for _, e := range pending {
		if e.ItemType != "" {
			counts[e.ItemType]--
		}
	}
	return counts
}

func appendOwned(result []string, seen map[string]bool, itemType string) []string {
	if itemType == "" || seen[itemType] {
		return result
	}
	seen[itemType] = true
	return append(result, itemType)
}

func ReadInventory() ([]string, error) {
	inv, err := loadInventory()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var result []string
	for _, e := range inv.RawUpgrades {
		result = appendOwned(result, seen, e.ItemType)
	}
	for _, e := range inv.MiscItems {
		result = appendOwned(result, seen, e.ItemType)
	}
	for itemType, count := range netRecipes(inv.Recipes, inv.PendingRecipes) {
		if count > 0 {
			result = appendOwned(result, seen, itemType)
		}
	}
	return result, nil
}

func queryItems(dbPath string, query string, scan itemScanner) ([]GameObject, error) {
	db, err := database.OpenReadOnlyDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []GameObject
	for rows.Next() {
		var item GameObject
		if err := scan(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func QuerySyndicateTradableItems(dbPath string) ([]GameObject, error) {
	return queryItems(dbPath, database.SyndicateTradableItemsQuery(), func(rows *sql.Rows, item *GameObject) error {
		return rows.Scan(&item.GameRef, &item.Slug, &item.Name, &item.Category, &item.Syndicates)
	})
}

func QueryTradableItems(dbPath string) ([]GameObject, error) {
	return queryItems(dbPath, database.TradableItemsQuery(), func(rows *sql.Rows, item *GameObject) error {
		return rows.Scan(&item.GameRef, &item.Slug, &item.Name, &item.Category)
	})
}

func QueryNightmareItems(dbPath string) ([]GameObject, error) {
	return queryItems(dbPath, database.NightmareItemsQuery(), func(rows *sql.Rows, item *GameObject) error {
		return rows.Scan(&item.GameRef, &item.Slug, &item.Name, &item.Rotation, &item.Chance)
	})
}

func QueryVaultItems(dbPath string) ([]GameObject, error) {
	return queryItems(dbPath, database.VaultItemsQuery(), func(rows *sql.Rows, item *GameObject) error {
		return rows.Scan(&item.GameRef, &item.Slug, &item.Name, &item.Chance)
	})
}

func QueryDucatItems(dbPath string) ([]GameObject, error) {
	return queryItems(dbPath, database.DucatItemsQuery(), func(rows *sql.Rows, item *GameObject) error {
		return rows.Scan(&item.GameRef, &item.Slug, &item.Name, &item.Ducats)
	})
}

var httpClient = &http.Client{Timeout: 15 * time.Second}
var statsBaseURL = "https://api.warframe.market"

func filterByRef[T any](owned []string, tradable []T, ref func(T) string) []T {
	m := make(map[string]T, len(tradable))
	for _, item := range tradable {
		m[ref(item)] = item
	}
	var result []T
	for _, r := range owned {
		if item, ok := m[r]; ok {
			result = append(result, item)
		}
	}
	return result
}

func FilterOwned(owned []string, tradable []GameObject) []GameObject {
	return filterByRef(owned, tradable, func(t GameObject) string { return t.GameRef })
}

func FetchItemPrice(ctx context.Context, item GameObject) (OutputItem, error) {
	avg, count, err := fetchStats(ctx, item.Slug)
	if err != nil {
		return OutputItem{}, err
	}
	return OutputItem{
		Name:       item.Name,
		GameRef:    item.GameRef,
		AvgPrice:   avg,
		Count:      count,
		Category:   item.Category,
		Syndicates: item.Syndicates,
		Ducats:     item.Ducats,
		Rotation:   item.Rotation,
		Chance:     item.Chance,
	}, nil
}

func SortResults(items []OutputItem) {
	slices.SortFunc(items, func(a, b OutputItem) int {
		if a.FetchFailed != b.FetchFailed {
			if a.FetchFailed {
				return 1
			}
			return -1
		}
		scoreA := a.AvgPrice * float64(a.Count)
		scoreB := b.AvgPrice * float64(b.Count)
		if scoreA > scoreB {
			return -1
		}
		if scoreA < scoreB {
			return 1
		}
		return 0
	})
}

func SortDucatResults(items []OutputItem) {
	slices.SortFunc(items, func(a, b OutputItem) int {
		if a.FetchFailed != b.FetchFailed {
			if a.FetchFailed {
				return 1
			}
			return -1
		}
		var scoreA, scoreB float64
		if a.AvgPrice > 0 {
			scoreA = float64(a.Ducats) / a.AvgPrice
		}
		if b.AvgPrice > 0 {
			scoreB = float64(b.Ducats) / b.AvgPrice
		}
		if scoreA > scoreB {
			return -1
		}
		if scoreA < scoreB {
			return 1
		}
		return 0
	})
}

func fetchStats(ctx context.Context, slug string) (float64, int, error) {
	if slug == "" {
		return 0, 0, fmt.Errorf("empty slug")
	}

	url := fmt.Sprintf("%s/v1/items/%s/statistics", statsBaseURL, slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("status %d", resp.StatusCode)
	}

	var stats struct {
		Payload struct {
			StatisticsClosed map[string][]struct {
				AvgPrice float64 `json:"avg_price"`
				Volume   int     `json:"volume"`
				ModRank  int     `json:"mod_rank"`
			} `json:"statistics_closed"`
		} `json:"payload"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return 0, 0, err
	}

	entries, ok := stats.Payload.StatisticsClosed["48hours"]
	if !ok {
		return 0, 0, fmt.Errorf("no 48hours stats for %s", slug)
	}

	var sumPrices float64
	var countPrices, totalVolume int
	for _, e := range entries {
		totalVolume += e.Volume
		if e.ModRank != 0 {
			continue
		}
		sumPrices += e.AvgPrice
		countPrices++
	}
	if countPrices == 0 {
		return 0, totalVolume, nil
	}
	return sumPrices / float64(countPrices), totalVolume, nil
}
