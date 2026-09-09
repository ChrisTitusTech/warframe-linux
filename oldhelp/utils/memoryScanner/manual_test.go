//go:build manual

package memoryScanner

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func requireManualAuthz(t *testing.T) (int, string) {
	t.Helper()

	pid, err := FindProcess()
	if err != nil {
		t.Fatalf("FindProcess unavailable on this host: %v", err)
	}
	if pid <= 0 {
		t.Fatalf("FindProcess() pid = %d, want > 0", pid)
	}

	authz, err := ScanAuthz(pid)
	if err != nil {
		t.Fatalf("ScanAuthz unavailable on this host: %v", err)
	}
	if !strings.HasPrefix(authz, authzPattern) {
		t.Fatalf("ScanAuthz() = %q, want prefix %q", authz, authzPattern)
	}

	return pid, authz
}

func TestFindProcessManual(t *testing.T) {
	pid, err := FindProcess()
	if err != nil {
		t.Fatalf("FindProcess unavailable on this host: %v", err)
	}
	if pid <= 0 {
		t.Fatalf("FindProcess() pid = %d, want > 0", pid)
	}
}

func TestScanAuthzManual(t *testing.T) {
	pid, authz := requireManualAuthz(t)
	if pid <= 0 || authz == "" {
		t.Fatalf("manual authz probe failed: pid=%d authz=%q", pid, authz)
	}
}

func TestFetchInventoryManual(t *testing.T) {
	_, authz := requireManualAuthz(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data, err := FetchInventory(ctx, authz)
	if err != nil {
		t.Fatalf("FetchInventory() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("FetchInventory() returned empty payload")
	}
	if data[0] != '{' && data[0] != '[' {
		t.Fatalf("FetchInventory() payload starts with %q, want JSON", data[0])
	}
}

func TestSaveInventoryManual(t *testing.T) {
	_, authz := requireManualAuthz(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data, err := FetchInventory(ctx, authz)
	if err != nil {
		t.Fatalf("FetchInventory() error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "inventory.json")
	if err := SaveInventory(data, path); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}
}
