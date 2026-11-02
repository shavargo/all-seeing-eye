package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"master-server/internal/games"
	"master-server/internal/registry"
)

// Server exposes an HTTP REST API for the Next.js frontend.
type Server struct {
	addr      string
	store     *registry.Store
	reg       *games.Registry
	server    *http.Server
	startTime time.Time
}

// NewServer creates a new API server.
func NewServer(addr string, store *registry.Store, reg *games.Registry) *Server {
	return &Server{
		addr:      addr,
		store:     store,
		reg:       reg,
		startTime: time.Now(),
	}
}

// Start runs the HTTP server until context cancel.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/stats", s.handleStats)
	mux.HandleFunc("/api/v1/servers", s.handleServers)
	mux.HandleFunc("/api/v1/games", s.handleGames)

	s.server = &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("[HTTP API] Internal REST API active on %s", s.addr)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
	}()

	err := s.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http api listen: %w", err)
	}
	return nil
}

func (s *Server) handleGames(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")

	if s.reg == nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"total": 0, "games": []string{}})
		return
	}

	q := r.URL.Query().Get("q")
	cat := r.URL.Query().Get("category")
	limitStr := r.URL.Query().Get("limit")

	limit := 100
	if limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 {
			limit = val
		}
	}

	results := s.reg.Search(q, cat, limit)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"total":      s.reg.TotalCount(),
		"matchCount": len(results),
		"games":      results,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
		"uptime": time.Since(s.startTime).String(),
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	snap := s.store.GetSnapshot()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"totalServers": snap.TotalServers,
		"totalPlayers": snap.TotalPlayers,
		"activeGames":  snap.ActiveGames,
		"generatedAt":  snap.GeneratedAt,
		"uptime":       time.Since(s.startTime).String(),
	})
}

func (s *Server) handleServers(w http.ResponseWriter, r *http.Request) {
	snap := s.store.GetSnapshot()
	gameFilter := strings.ToLower(r.URL.Query().Get("game"))

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	if gameFilter == "" || gameFilter == "all" {
		_ = json.NewEncoder(w).Encode(snap)
		return
	}

	var filtered []registry.Server
	totalPlayers := 0
	for _, srv := range snap.Servers {
		if srv.Gamename == gameFilter {
			filtered = append(filtered, srv)
			totalPlayers += srv.Players
		}
	}

	_ = json.NewEncoder(w).Encode(registry.Snapshot{
		GeneratedAt:  snap.GeneratedAt,
		TotalServers: len(filtered),
		TotalPlayers: totalPlayers,
		ActiveGames:  1,
		Servers:      filtered,
	})
}
