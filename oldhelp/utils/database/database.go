package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

func OpenDB(path string) (*sql.DB, error) {
	return sql.Open("sqlite3", path)
}

func readOnlyDSN(path string) string {
	dsnPath := strings.ReplaceAll(path, `\`, "/")
	if dsnPath != "" && !strings.HasPrefix(dsnPath, "/") && len(dsnPath) >= 3 && dsnPath[1] == ':' && dsnPath[2] == '/' {
		dsnPath = "/" + dsnPath
	}
	return "file:" + dsnPath + "?mode=ro"
}

func OpenReadOnlyDB(path string) (*sql.DB, error) {
	return sql.Open("sqlite3", readOnlyDSN(path))
}

func fetchBody(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func FetchRaw(ctx context.Context, url string) ([]map[string]any, error) {
	body, err := fetchBody(ctx, url)
	if err != nil {
		return nil, err
	}
	var raw []map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
