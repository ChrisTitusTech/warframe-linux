package database

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadOnlyDSN(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "unix absolute path", path: "/tmp/warframe.db", want: "file:/tmp/warframe.db?mode=ro"},
		{name: "relative path", path: "warframe.db", want: "file:warframe.db?mode=ro"},
		{name: "windows absolute path", path: `C:\Users\runneradmin\.warframe-helper\warframe.db`, want: "file:/C:/Users/runneradmin/.warframe-helper/warframe.db?mode=ro"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := readOnlyDSN(tt.path); got != tt.want {
				t.Fatalf("readOnlyDSN() = %q, want %q", got, tt.want)
			}
		})
	}
}

func withDatabaseHTTPClient(t *testing.T, client *http.Client) {
	t.Helper()
	prev := httpClient
	httpClient = client
	t.Cleanup(func() { httpClient = prev })
}

func TestFetchBody(t *testing.T) {
	t.Run("returns body for ok response", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Fatalf("method = %s, want GET", r.Method)
			}
			_, _ = w.Write([]byte(`[{"ok":true}]`))
		}))
		defer ts.Close()
		withDatabaseHTTPClient(t, ts.Client())

		got, err := fetchBody(context.Background(), ts.URL)
		if err != nil {
			t.Fatalf("fetchBody() error = %v", err)
		}
		if string(got) != `[{"ok":true}]` {
			t.Fatalf("fetchBody() = %s", got)
		}
	})

	t.Run("returns error for non ok response", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusBadGateway)
		}))
		defer ts.Close()
		withDatabaseHTTPClient(t, ts.Client())

		_, err := fetchBody(context.Background(), ts.URL)
		if err == nil || !strings.Contains(err.Error(), "unexpected status: 502 Bad Gateway") {
			t.Fatalf("fetchBody() error = %v, want status error", err)
		}
	})

	t.Run("returns request build error", func(t *testing.T) {
		_, err := fetchBody(context.Background(), "://bad url")
		if err == nil {
			t.Fatal("fetchBody() error = nil, want url error")
		}
	})

	t.Run("honors context cancellation", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer ts.Close()
		withDatabaseHTTPClient(t, ts.Client())

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := fetchBody(ctx, ts.URL)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("fetchBody() error = %v, want context.Canceled", err)
		}
	})
}

func TestFetchRaw(t *testing.T) {
	t.Run("decodes array payload", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`[{"name":"A"},{"name":"B","n":2}]`))
		}))
		defer ts.Close()
		withDatabaseHTTPClient(t, ts.Client())

		got, err := FetchRaw(context.Background(), ts.URL)
		if err != nil {
			t.Fatalf("FetchRaw() error = %v", err)
		}
		if len(got) != 2 || got[0]["name"] != "A" || got[1]["n"] != float64(2) {
			t.Fatalf("FetchRaw() = %#v", got)
		}
	})

	t.Run("returns unmarshal error for invalid json", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"not":"an array"}`))
		}))
		defer ts.Close()
		withDatabaseHTTPClient(t, ts.Client())

		_, err := FetchRaw(context.Background(), ts.URL)
		if err == nil {
			t.Fatal("FetchRaw() error = nil, want unmarshal error")
		}
	})
}
