package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleGenerateEchartsPageAPIWithObjectInputSchema(t *testing.T) {
	staticDir := t.TempDir()
	t.Setenv("STATIC_DIR", staticDir)

	body := `{"title":"Test Chart","inputSchema":{"xAxis":{"type":"category"},"series":[{"type":"line","data":[1,2,3]}]}}`
	resp := executeGenerateEchartsPageRequest(t, body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var payload GenerateEchartsPageResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if payload.URL == "" {
		t.Fatalf("expected a non-empty URL in response")
	}

	assertChartFileExists(t, staticDir, payload.URL)
}

func TestHandleGenerateEchartsPageAPIWithStringInputSchema(t *testing.T) {
	staticDir := t.TempDir()
	t.Setenv("STATIC_DIR", staticDir)

	body := `{"title":"Test Chart","inputSchema":"{\"xAxis\":{\"type\":\"value\"},\"series\":[{\"type\":\"bar\",\"data\":[4,5,6]}]}"}`
	resp := executeGenerateEchartsPageRequest(t, body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var payload GenerateEchartsPageResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if payload.URL == "" {
		t.Fatalf("expected a non-empty URL in response")
	}

	assertChartFileExists(t, staticDir, payload.URL)
}

func executeGenerateEchartsPageRequest(t *testing.T, body string) *http.Response {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/GenerateEchartsPage", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	handleGenerateEchartsPageAPI(recorder, req)

	resp := recorder.Result()
	t.Cleanup(func() {
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("failed to close response body: %v", err)
		}
	})

	return resp
}

func assertChartFileExists(t *testing.T, staticDir, urlStr string) {
	t.Helper()

	parsed, err := url.Parse(urlStr)
	if err != nil {
		t.Fatalf("failed to parse response URL: %v", err)
	}

	if !strings.HasPrefix(parsed.Path, "/charts/") {
		t.Fatalf("unexpected chart URL path: %s", parsed.Path)
	}

	fileName := path.Base(parsed.Path)
	filePath := filepath.Join(staticDir, "charts", fileName)

	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("expected generated chart file at %s: %v", filePath, err)
	}
}
