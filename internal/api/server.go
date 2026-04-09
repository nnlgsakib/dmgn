package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/nnlgsakib/dmgn/internal/config"
	"github.com/nnlgsakib/dmgn/internal/crypto"
	"github.com/nnlgsakib/dmgn/pkg/identity"
	"github.com/nnlgsakib/dmgn/pkg/network"
	"github.com/nnlgsakib/dmgn/pkg/query"
	"github.com/nnlgsakib/dmgn/pkg/storage"
	"github.com/nnlgsakib/dmgn/pkg/sync"
	"github.com/nnlgsakib/dmgn/pkg/vectorindex"
)

type Server struct {
	store       *storage.Store
	cryptoEng   *crypto.Engine
	identity    *identity.Identity
	config      *config.Config
	auth        *AuthMiddleware
	httpServer  *http.Server
	networkHost *network.Host
	queryEngine *query.QueryEngine
	remoteOrch  *query.RemoteQueryOrchestrator
	gossipMgr   *sync.GossipManager
	vecIndex    *vectorindex.VectorIndex
}

func NewServer(cfg *config.Config, store *storage.Store, cryptoEng *crypto.Engine, id *identity.Identity) (*Server, error) {
	apiKey, err := id.DeriveKey(APIKeyPurpose, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to derive API key: %w", err)
	}

	s := &Server{
		store:     store,
		cryptoEng: cryptoEng,
		identity:  id,
		config:    cfg,
		auth:      NewAuthMiddleware(apiKey),
	}

	mux := s.setupRoutes()

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.APIPort),
		Handler:      requestLogger(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s, nil
}

// SetNetworkHost attaches a network host to the server for live network stats.
func (s *Server) SetNetworkHost(h *network.Host) {
	s.networkHost = h
}

// SetQueryEngine attaches the query engine and remote orchestrator.
func (s *Server) SetQueryEngine(qe *query.QueryEngine, ro *query.RemoteQueryOrchestrator) {
	s.queryEngine = qe
	s.remoteOrch = ro
}

// SetGossipManager attaches the gossip manager for memory propagation.
func (s *Server) SetGossipManager(gm *sync.GossipManager) {
	s.gossipMgr = gm
}

// SetVectorIndex attaches the vector index for embedding indexing.
func (s *Server) SetVectorIndex(vi *vectorindex.VectorIndex) {
	s.vecIndex = vi
}

func (s *Server) Start() error {
	fmt.Printf("API server listening on :%d\n", s.config.APIPort)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) APIKey() string {
	apiKey, err := s.identity.DeriveKey(APIKeyPurpose, 32)
	if err != nil {
		return ""
	}
	return DeriveAPIKey(apiKey)
}

func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.Handle("POST /memory", s.auth.Authenticate(http.HandlerFunc(s.HandleAddMemory)))
	mux.Handle("GET /query", s.auth.Authenticate(http.HandlerFunc(s.HandleQuery)))
	mux.Handle("GET /status", s.auth.Authenticate(http.HandlerFunc(s.HandleStatus)))
	mux.Handle("GET /peers", s.auth.Authenticate(http.HandlerFunc(s.HandlePeers)))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	})

	return mux
}

// Handler returns the HTTP handler for testing
func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}
