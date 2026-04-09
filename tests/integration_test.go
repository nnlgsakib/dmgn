package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/dmgn/dmgn/internal/api"
	"github.com/dmgn/dmgn/internal/config"
	"github.com/dmgn/dmgn/internal/crypto"
	"github.com/dmgn/dmgn/pkg/identity"
	"github.com/dmgn/dmgn/pkg/memory"
	"github.com/dmgn/dmgn/pkg/network"
	"github.com/dmgn/dmgn/pkg/storage"
)

type integrationEnv struct {
	id         *identity.Identity
	cryptoEng  *crypto.Engine
	store      *storage.Store
	server     *httptest.Server
	apiKey     string
	passphrase string
}

func setupIntegration(t *testing.T) *integrationEnv {
	t.Helper()

	storageDir := t.TempDir()
	identityDir := t.TempDir()
	passphrase := "integration-test-passphrase"

	id, err := identity.Generate(passphrase, identityDir)
	if err != nil {
		t.Fatalf("Generate identity failed: %v", err)
	}

	masterKey, err := id.DeriveKey("memory-encryption", 32)
	if err != nil {
		t.Fatalf("DeriveKey failed: %v", err)
	}

	eng, err := crypto.NewEngine(masterKey)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	store, err := storage.New(storage.Options{DataDir: storageDir})
	if err != nil {
		t.Fatalf("New store failed: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := config.DefaultConfig()
	cfg.DataDir = storageDir

	srv, err := api.NewServer(cfg, store, eng, id)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { ts.Close() })

	return &integrationEnv{
		id:         id,
		cryptoEng:  eng,
		store:      store,
		server:     ts,
		apiKey:     srv.APIKey(),
		passphrase: passphrase,
	}
}

// TestFullPipelineEncryptStoreQueryDecrypt tests the complete pipeline:
// create memory → encrypt → store → query → decrypt → verify plaintext matches
func TestFullPipelineEncryptStoreQueryDecrypt(t *testing.T) {
	env := setupIntegration(t)

	originalContent := "The quick brown fox jumps over the lazy dog"

	// Step 1: Create and store via API
	body, _ := json.Marshal(map[string]string{"content": originalContent, "type": "text"})
	req, _ := http.NewRequest("POST", env.server.URL+"/memory", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+env.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /memory failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 201, got %d", resp.StatusCode)
	}

	var addResp api.AddMemoryResponse
	json.NewDecoder(resp.Body).Decode(&addResp)

	if addResp.ID == "" {
		t.Fatal("Memory ID should not be empty")
	}

	// Step 2: Query via API
	req, _ = http.NewRequest("GET", env.server.URL+"/query?q=quick+brown+fox", nil)
	req.Header.Set("Authorization", "Bearer "+env.apiKey)

	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /query failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp2.StatusCode)
	}

	var queryResp api.QueryResponse
	json.NewDecoder(resp2.Body).Decode(&queryResp)

	if queryResp.Count == 0 {
		t.Fatal("Query should return at least 1 result")
	}

	// Step 3: Verify decrypted content matches original
	found := false
	for _, r := range queryResp.Results {
		if r.Content == originalContent {
			found = true
			break
		}
	}
	if !found {
		t.Error("Decrypted content should match original plaintext")
	}
}

// TestNoPlaintextLeakage verifies that no plaintext appears in the raw stored data
func TestNoPlaintextLeakage(t *testing.T) {
	env := setupIntegration(t)

	secretContent := "SUPER_SECRET_MEMORY_CONTENT_12345"

	encryptFn := func(data []byte) ([]byte, error) {
		return env.cryptoEng.Encrypt(data)
	}

	plain := &memory.PlaintextMemory{
		Content:  secretContent,
		Type:     memory.TypeText,
		Metadata: map[string]string{"source": "test"},
	}

	mem, err := memory.Create(plain, nil, encryptFn)
	if err != nil {
		t.Fatalf("Create memory failed: %v", err)
	}

	if err := env.store.SaveMemory(mem); err != nil {
		t.Fatalf("SaveMemory failed: %v", err)
	}

	// The encrypted payload must NOT contain the plaintext
	if bytes.Contains(mem.EncryptedPayload, []byte(secretContent)) {
		t.Error("Encrypted payload contains plaintext — encryption is broken!")
	}

	// Serialized memory JSON must not contain plaintext
	memJSON, _ := mem.ToJSON()
	if bytes.Contains(memJSON, []byte(secretContent)) {
		t.Error("Serialized memory JSON contains plaintext")
	}

	// Verify we CAN decrypt it back
	decryptFn := func(data []byte) ([]byte, error) {
		return env.cryptoEng.Decrypt(data)
	}

	decrypted, err := mem.Decrypt(decryptFn)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted.Content != secretContent {
		t.Errorf("Decrypted content mismatch: got %q, want %q", decrypted.Content, secretContent)
	}
}

// TestHKDFDeterminism verifies that HKDF produces consistent keys
func TestHKDFDeterminism(t *testing.T) {
	identityDir := t.TempDir()
	passphrase := "determinism-test"

	id, err := identity.Generate(passphrase, identityDir)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	key1, err := id.DeriveKey("memory-encryption", 32)
	if err != nil {
		t.Fatalf("DeriveKey 1 failed: %v", err)
	}

	key2, err := id.DeriveKey("memory-encryption", 32)
	if err != nil {
		t.Fatalf("DeriveKey 2 failed: %v", err)
	}

	if !bytes.Equal(key1, key2) {
		t.Error("HKDF should produce identical keys for same identity + purpose")
	}

	apiKey, err := id.DeriveKey("api-key", 32)
	if err != nil {
		t.Fatalf("DeriveKey api-key failed: %v", err)
	}

	if bytes.Equal(key1, apiKey) {
		t.Error("Different purposes should produce different keys")
	}
}

// TestCryptoFramingRoundTrip tests encrypt→decrypt round-trip with various payload sizes
func TestCryptoFramingRoundTrip(t *testing.T) {
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}

	eng, err := crypto.NewEngine(masterKey)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	sizes := []int{0, 1, 16, 256, 1024, 4096, 65536}
	for _, size := range sizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			plaintext := make([]byte, size)
			for i := range plaintext {
				plaintext[i] = byte(i % 256)
			}

			ciphertext, err := eng.Encrypt(plaintext)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			decrypted, err := eng.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if !bytes.Equal(decrypted, plaintext) {
				t.Errorf("Round-trip mismatch for size %d", size)
			}
		})
	}
}

// TestRetentionIntegration verifies that retention works end-to-end via API
func TestRetentionIntegration(t *testing.T) {
	storageDir := t.TempDir()
	identityDir := t.TempDir()

	id, err := identity.Generate("retention-test", identityDir)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	masterKey, err := id.DeriveKey("memory-encryption", 32)
	if err != nil {
		t.Fatalf("DeriveKey failed: %v", err)
	}

	eng, err := crypto.NewEngine(masterKey)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	store, err := storage.New(storage.Options{
		DataDir:      storageDir,
		MaxRetention: 5,
	})
	if err != nil {
		t.Fatalf("New store failed: %v", err)
	}
	defer store.Close()

	encryptFn := func(data []byte) ([]byte, error) {
		return eng.Encrypt(data)
	}

	for i := 0; i < 10; i++ {
		plain := &memory.PlaintextMemory{
			Content:  fmt.Sprintf("retention memory %d", i),
			Type:     memory.TypeText,
			Metadata: map[string]string{},
		}
		mem, err := memory.Create(plain, nil, encryptFn)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if err := store.SaveMemory(mem); err != nil {
			t.Fatalf("SaveMemory failed: %v", err)
		}
	}

	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats["memory_count"] != 5 {
		t.Errorf("Expected 5 memories with retention=5, got %d", stats["memory_count"])
	}
}

// TestAPIAuthDerived verifies that the API key is deterministically derived from identity
func TestAPIAuthDerived(t *testing.T) {
	env := setupIntegration(t)

	// Derive key same way the server does
	apiKeyBytes, err := env.id.DeriveKey("api-key", 32)
	if err != nil {
		t.Fatalf("DeriveKey failed: %v", err)
	}

	expectedKey := api.DeriveAPIKey(apiKeyBytes)
	if env.apiKey != expectedKey {
		t.Errorf("API key mismatch: server=%s, derived=%s", env.apiKey, expectedKey)
	}

	// Verify it actually works for auth
	req, _ := http.NewRequest("GET", env.server.URL+"/status", nil)
	req.Header.Set("Authorization", "Bearer "+expectedKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}

// TestExportImportRoundTrip tests identity export/import preserves crypto capability
func TestExportImportRoundTrip(t *testing.T) {
	originalDir := t.TempDir()
	importDir := t.TempDir()
	passphrase := "export-import-test"

	// Generate identity
	origID, err := identity.Generate(passphrase, originalDir)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Derive key and encrypt something
	masterKey, err := origID.DeriveKey("memory-encryption", 32)
	if err != nil {
		t.Fatalf("DeriveKey failed: %v", err)
	}

	eng, err := crypto.NewEngine(masterKey)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	secretData := []byte("test data for export/import round trip")
	ciphertext, err := eng.Encrypt(secretData)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Export
	exported, err := identity.Export(originalDir)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Import to new location
	if err := identity.Import(exported, importDir); err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Load imported identity
	importedID, err := identity.Load(passphrase, importDir)
	if err != nil {
		t.Fatalf("Load imported failed: %v", err)
	}

	// Verify same node ID
	if importedID.ID != origID.ID {
		t.Errorf("Node ID mismatch after import: %s != %s", importedID.ID, origID.ID)
	}

	// Derive key from imported identity and decrypt
	importedKey, err := importedID.DeriveKey("memory-encryption", 32)
	if err != nil {
		t.Fatalf("DeriveKey from imported failed: %v", err)
	}

	if !bytes.Equal(masterKey, importedKey) {
		t.Error("HKDF keys should match between original and imported identity")
	}

	eng2, err := crypto.NewEngine(importedKey)
	if err != nil {
		t.Fatalf("NewEngine from imported key failed: %v", err)
	}

	decrypted, err := eng2.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt with imported identity failed: %v", err)
	}

	if !bytes.Equal(decrypted, secretData) {
		t.Error("Decrypted data from imported identity doesn't match original")
	}
}

// TestMultipleMemoriesQueryScoring verifies that query returns results in score order
func TestMultipleMemoriesQueryScoring(t *testing.T) {
	env := setupIntegration(t)

	contents := []string{
		"The weather is nice today",
		"Go programming language tutorial",
		"Go is a great programming language for systems",
		"Python is another programming language",
	}

	for _, content := range contents {
		body, _ := json.Marshal(map[string]string{"content": content})
		req, _ := http.NewRequest("POST", env.server.URL+"/memory", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+env.apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST failed: %v", err)
		}
		resp.Body.Close()
	}

	// Query for "programming" — should match 3 of 4 memories
	req, _ := http.NewRequest("GET", env.server.URL+"/query?q=programming", nil)
	req.Header.Set("Authorization", "Bearer "+env.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /query failed: %v", err)
	}
	defer resp.Body.Close()

	var result api.QueryResponse
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Count < 3 {
		t.Errorf("Expected at least 3 results for 'programming', got %d", result.Count)
	}

	// Verify all results contain "programming"
	for _, r := range result.Results {
		if !strings.Contains(strings.ToLower(r.Content), "programming") {
			t.Errorf("Unexpected result without 'programming': %s", r.Content)
		}
	}
}

// --- Phase 3: Networking Integration Tests ---

func createTestNetworkHost(t *testing.T) (*network.Host, *identity.Identity) {
	t.Helper()
	dir := t.TempDir()
	id, err := identity.Generate("net-test-passphrase", dir)
	if err != nil {
		t.Fatalf("Generate identity failed: %v", err)
	}

	key, err := network.DeriveLibp2pKey(id)
	if err != nil {
		t.Fatalf("DeriveLibp2pKey failed: %v", err)
	}

	h, err := network.NewHost(network.HostConfig{
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
		MDNSService:  "",
		MaxPeersLow:  5,
		MaxPeersHigh: 10,
		PrivateKey:   key,
	})
	if err != nil {
		t.Fatalf("NewHost failed: %v", err)
	}
	t.Cleanup(func() { h.Stop() })
	return h, id
}

func TestStartWithNetworking(t *testing.T) {
	h, _ := createTestNetworkHost(t)

	if err := h.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if h.ID() == "" {
		t.Error("peer ID should be non-empty after start")
	}

	if h.PeerCount() != 0 {
		t.Errorf("expected 0 peers initially, got %d", h.PeerCount())
	}

	stats := h.NetworkStats()
	if stats["dht_mode"] != "active" {
		t.Errorf("expected dht_mode 'active' after Start(), got %v", stats["dht_mode"])
	}
}

func TestAPIStatusWithNetwork(t *testing.T) {
	h, id := createTestNetworkHost(t)
	if err := h.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	storageDir := t.TempDir()
	store, err := storage.New(storage.Options{DataDir: storageDir})
	if err != nil {
		t.Fatalf("New store failed: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	masterKey, _ := id.DeriveKey("memory-encryption", 32)
	eng, _ := crypto.NewEngine(masterKey)
	cfg := config.DefaultConfig()
	cfg.DataDir = storageDir

	srv, err := api.NewServer(cfg, store, eng, id)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	srv.SetNetworkHost(h)

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { ts.Close() })

	req, _ := http.NewRequest("GET", ts.URL+"/status", nil)
	req.Header.Set("Authorization", "Bearer "+srv.APIKey())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("status request failed: %v", err)
	}
	defer resp.Body.Close()

	var statusResp struct {
		Network struct {
			Status      string   `json:"status"`
			Peers       int      `json:"peers"`
			PeerID      string   `json:"peer_id"`
			ListenAddrs []string `json:"listen_addrs"`
		} `json:"network"`
	}
	json.NewDecoder(resp.Body).Decode(&statusResp)

	if statusResp.Network.Status != "running" {
		t.Errorf("expected network status 'running', got %q", statusResp.Network.Status)
	}
	if statusResp.Network.PeerID == "" {
		t.Error("expected non-empty peer_id in status response")
	}
	if statusResp.Network.Peers != 0 {
		t.Errorf("expected 0 peers, got %d", statusResp.Network.Peers)
	}
}

func TestTwoPeersDiscoverViaDirect(t *testing.T) {
	h1, _ := createTestNetworkHost(t)
	h2, _ := createTestNetworkHost(t)

	// Connect h2 to h1 directly
	h1Info := peer.AddrInfo{
		ID:    h1.ID(),
		Addrs: h1.Addrs(),
	}

	if err := h2.LibP2PHost().Connect(context.Background(), h1Info); err != nil {
		t.Fatalf("failed to connect h2 to h1: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if h1.PeerCount() != 1 {
		t.Errorf("h1 expected 1 peer, got %d", h1.PeerCount())
	}
	if h2.PeerCount() != 1 {
		t.Errorf("h2 expected 1 peer, got %d", h2.PeerCount())
	}

	peers1 := h1.ConnectedPeers()
	if len(peers1) != 1 || peers1[0].ID != h2.ID().String() {
		t.Errorf("h1 should see h2 as connected peer")
	}

	peers2 := h2.ConnectedPeers()
	if len(peers2) != 1 || peers2[0].ID != h1.ID().String() {
		t.Errorf("h2 should see h1 as connected peer")
	}
}
