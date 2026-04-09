package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nnlgsakib/dmgn/internal/config"
	"github.com/nnlgsakib/dmgn/internal/crypto"
	"github.com/nnlgsakib/dmgn/pkg/identity"
	"github.com/nnlgsakib/dmgn/pkg/storage"
)

type testEnv struct {
	server *Server
	ts     *httptest.Server
	apiKey string
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	tmpDir := t.TempDir()
	identityDir := t.TempDir()

	passphrase := "test-passphrase-123"
	id, err := identity.Generate(passphrase, identityDir)
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	masterKey, err := id.DeriveKey("memory-encryption", 32)
	if err != nil {
		t.Fatalf("Failed to derive master key: %v", err)
	}

	cryptoEng, err := crypto.NewEngine(masterKey)
	if err != nil {
		t.Fatalf("Failed to create crypto engine: %v", err)
	}

	store, err := storage.New(storage.Options{
		DataDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := config.DefaultConfig()
	cfg.DataDir = tmpDir

	srv, err := NewServer(cfg, store, cryptoEng, id)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { ts.Close() })

	return &testEnv{
		server: srv,
		ts:     ts,
		apiKey: srv.APIKey(),
	}
}

func (e *testEnv) doRequest(t *testing.T, method, path string, body interface{}, useAuth bool) *http.Response {
	t.Helper()

	var bodyReader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(data)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, e.ts.URL+path, bodyReader)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if useAuth {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.apiKey))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	return resp
}

func TestAddMemoryEndpoint(t *testing.T) {
	env := setupTestEnv(t)

	resp := env.doRequest(t, "POST", "/memory", AddMemoryRequest{
		Content: "Test memory content",
		Type:    "text",
	}, true)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}

	var result AddMemoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.ID == "" {
		t.Error("Response ID should not be empty")
	}
	if result.Type != "text" {
		t.Errorf("Expected type 'text', got '%s'", result.Type)
	}
}

func TestAddMemoryNoContent(t *testing.T) {
	env := setupTestEnv(t)

	resp := env.doRequest(t, "POST", "/memory", AddMemoryRequest{
		Content: "",
	}, true)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", resp.StatusCode)
	}
}

func TestQueryEndpoint(t *testing.T) {
	env := setupTestEnv(t)

	// Add a memory first
	resp := env.doRequest(t, "POST", "/memory", AddMemoryRequest{
		Content: "Hello world test query",
	}, true)
	resp.Body.Close()

	// Query for it
	resp = env.doRequest(t, "GET", "/query?q=hello", nil, true)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var result QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Count == 0 {
		t.Error("Expected at least 1 result")
	}
}

func TestQueryRecentEndpoint(t *testing.T) {
	env := setupTestEnv(t)

	// Add a memory
	resp := env.doRequest(t, "POST", "/memory", AddMemoryRequest{
		Content: "Recent memory test",
	}, true)
	resp.Body.Close()

	// Query without q param returns recent
	resp = env.doRequest(t, "GET", "/query", nil, true)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var result QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Count == 0 {
		t.Error("Expected at least 1 recent result")
	}
}

func TestStatusEndpoint(t *testing.T) {
	env := setupTestEnv(t)

	resp := env.doRequest(t, "GET", "/status", nil, true)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var result StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.NodeID == "" {
		t.Error("NodeID should not be empty")
	}
	if result.Version == "" {
		t.Error("Version should not be empty")
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", ct)
	}
}

func TestAuthRequired(t *testing.T) {
	env := setupTestEnv(t)

	resp := env.doRequest(t, "GET", "/status", nil, false)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthWrongKey(t *testing.T) {
	env := setupTestEnv(t)

	req, _ := http.NewRequest("GET", env.ts.URL+"/status", nil)
	req.Header.Set("Authorization", "Bearer deadbeef01020304050607080910111213141516171819202122232425262728293031")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthCorrectKey(t *testing.T) {
	env := setupTestEnv(t)

	resp := env.doRequest(t, "GET", "/status", nil, true)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}
