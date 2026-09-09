package memoryScanner

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestFetchInventory(t *testing.T) {
	originalClient := httpClient
	t.Cleanup(func() {
		httpClient = originalClient
	})

	t.Run("pretty prints inventory json", func(t *testing.T) {
		const authz = "?accountId=abc&nonce=123"
		httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("method = %s, want GET", req.Method)
			}
			if got := req.URL.String(); got != inventoryURL+authz {
				t.Fatalf("url = %q, want %q", got, inventoryURL+authz)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(`{"items":[1,2]}`)),
				Header:     make(http.Header),
			}, nil
		})}

		got, err := FetchInventory(context.Background(), authz)
		if err != nil {
			t.Fatalf("FetchInventory() error = %v", err)
		}

		want := "{\n  \"items\": [\n    1,\n    2\n  ]\n}"
		if string(got) != want {
			t.Fatalf("FetchInventory() = %q, want %q", got, want)
		}
	})

	t.Run("request error wrapped", func(t *testing.T) {
		wantErr := errors.New("network down")
		httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, wantErr
		})}

		_, err := FetchInventory(context.Background(), "?accountId=x&nonce=1")
		if !errors.Is(err, wantErr) {
			t.Fatalf("FetchInventory() error = %v, want wrapped %v", err, wantErr)
		}
	})

	t.Run("non-200 status fails", func(t *testing.T) {
		httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Status:     "403 Forbidden",
				Body:       io.NopCloser(strings.NewReader(`{"error":"forbidden"}`)),
				Header:     make(http.Header),
			}, nil
		})}

		_, err := FetchInventory(context.Background(), "?accountId=x&nonce=1")
		if err == nil || err.Error() != "unexpected status: 403 Forbidden" {
			t.Fatalf("FetchInventory() error = %v, want %q", err, "unexpected status: 403 Forbidden")
		}
	})

	t.Run("invalid json fails", func(t *testing.T) {
		httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(`not-json`)),
				Header:     make(http.Header),
			}, nil
		})}

		_, err := FetchInventory(context.Background(), "?accountId=x&nonce=1")
		if err == nil || !strings.Contains(err.Error(), "invalid JSON response") {
			t.Fatalf("FetchInventory() error = %v, want invalid JSON response", err)
		}
	})
}

func TestSaveInventory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "inventory.json")
	data := []byte("inventory-data")

	if err := SaveInventory(data, path); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(got) != string(data) {
		t.Fatalf("saved data = %q, want %q", got, data)
	}
}
