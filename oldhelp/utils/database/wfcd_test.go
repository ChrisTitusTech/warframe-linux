package database

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withWFCDBaseURL(t *testing.T, url string) {
	t.Helper()
	prev := wfcdBaseURL
	wfcdBaseURL = url
	t.Cleanup(func() { wfcdBaseURL = prev })
}

func withWFCDCommitAPIURL(t *testing.T, url string) {
	t.Helper()
	prev := wfcdCommitAPIURL
	wfcdCommitAPIURL = url
	t.Cleanup(func() { wfcdCommitAPIURL = prev })
}

func TestWFCDJSONURL(t *testing.T) {
	withWFCDBaseURL(t, "https://example.test/%s/files/%s")
	if got := wfcdJSONURL("sha123", "Mods.json"); got != "https://example.test/sha123/files/Mods.json" {
		t.Fatalf("wfcdJSONURL() = %q", got)
	}
}

func TestFetchWFCDRaw(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`[{"uniqueName":"/Lotus/Mods/Test"}]`))
	}))
	defer ts.Close()
	withDatabaseHTTPClient(t, ts.Client())
	withWFCDBaseURL(t, ts.URL+"/%s/data/json/%s")

	got, err := FetchWFCDRaw(context.Background(), "ref-1", "Mods.json")
	if err != nil {
		t.Fatalf("FetchWFCDRaw() error = %v", err)
	}
	if gotPath != "/ref-1/data/json/Mods.json" {
		t.Fatalf("request path = %q, want %q", gotPath, "/ref-1/data/json/Mods.json")
	}
	if len(got) != 1 || got[0]["uniqueName"] != "/Lotus/Mods/Test" {
		t.Fatalf("FetchWFCDRaw() = %#v", got)
	}
}
