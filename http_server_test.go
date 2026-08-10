package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestIsPortAvailableDetectsBoundPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	if IsPortAvailable(port) {
		t.Fatalf("IsPortAvailable(%d) = true, want false for bound port", port)
	}
}

func TestHTTPServiceStatusReportsStoppedByDefault(t *testing.T) {
	service := NewHTTPService(filepath.Join(t.TempDir(), "config.json"), NewAIClient("", nil))

	status := service.Status()
	if status.Running {
		t.Fatal("Running = true, want false")
	}
	if status.Port != 8765 {
		t.Fatalf("Port = %d, want 8765", status.Port)
	}
	if status.URL != "http://127.0.0.1:8765/" {
		t.Fatalf("URL = %q, want default config URL", status.URL)
	}
}

func TestHTTPServiceConfigRoundTrip(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	service := NewHTTPService(configPath, NewAIClient("", nil))
	cfg := AIConfig{
		Port:              9091,
		Model:             "alpha-free",
		SystemPrompt:      "系统",
		ConversationLimit: 8,
		EnableFriend:      true,
		EnableGroup:       false,
		EnableChannel:     true,
	}
	body, _ := json.Marshal(cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	service.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/config status = %d, body %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec = httptest.NewRecorder()
	service.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/status status = %d, body %s", rec.Code, rec.Body.String())
	}
	var status HTTPServiceStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.Config.Model != "alpha-free" || status.Config.Port != 9091 || status.Config.EnableGroup {
		t.Fatalf("status config = %+v, want saved config", status.Config)
	}
}

func TestHTTPServiceModelsEndpointReturnsFreeModels(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"alpha-free"},{"id":"beta-paid"}]}`))
	}))
	defer upstream.Close()
	service := NewHTTPService(filepath.Join(t.TempDir(), "config.json"), NewAIClient(upstream.URL, upstream.Client()))

	req := httptest.NewRequest(http.MethodGet, "/api/models", nil)
	rec := httptest.NewRecorder()
	service.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/models status = %d, body %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Models []string `json:"models"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode models: %v", err)
	}
	if len(out.Models) != 1 || out.Models[0] != "alpha-free" {
		t.Fatalf("models = %#v, want alpha-free only", out.Models)
	}
}
