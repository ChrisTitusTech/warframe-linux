package memoryScanner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

const (
	processName          = "Warframe.x64.exe"
	processNameTruncated = "Warframe.x64.ex" // Linux: /proc/[pid]/comm is capped at 15 chars
	authzPattern         = "?accountId="
	inventoryURL         = "https://mobile.warframe.com/api/inventory.php"
	authzConfidence      = 3 // must appear this many times before trusting
)

func FindProcess() (int, error) {
	pid, err := findProcess(processName)
	if err == nil {
		return pid, nil
	}
	// Fallback for Linux where process names are truncated to 15 chars.
	pid, err = findProcess(processNameTruncated)
	if err != nil {
		return 0, fmt.Errorf("warframe process not found")
	}
	return pid, nil
}

func ScanAuthz(pid int) (string, error) {
	return scanAuthz(pid)
}

func FetchInventory(ctx context.Context, authz string) ([]byte, error) {
	url := inventoryURL + authz
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}

	return pretty.Bytes(), nil
}

func SaveInventory(data []byte, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}
