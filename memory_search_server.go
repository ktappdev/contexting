package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultSearchLogQueryMax = 120

type memorySearchServer struct {
	runtimeFile string
	listener    net.Listener
	httpServer  *http.Server
}

type MemorySearchLogOptions struct {
	Enabled  bool
	QueryMax int
}

type memorySearchRequest struct {
	Query string        `json:"query"`
	Opts  SearchOptions `json:"opts"`
}

type memorySearchResponse struct {
	Results []SearchResult `json:"results"`
}

func startMemorySearchServer(ctx context.Context, manager *IndexManager, runtimeFile string, logOpts MemorySearchLogOptions) (*memorySearchServer, error) {
	if logOpts.QueryMax <= 0 {
		logOpts.QueryMax = defaultSearchLogQueryMax
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen memory server: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		start := time.Now()

		var req memorySearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if logOpts.Enabled {
				logWarnf("Search request rejected: invalid payload")
			}
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		results := manager.Search(req.Query, req.Opts)
		if logOpts.Enabled {
			logInfof("Search query \"%s\" -> %d results in %dms", formatSearchLogQuery(req.Query, logOpts.QueryMax), len(results), time.Since(start).Milliseconds())
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(memorySearchResponse{Results: results})
	})

	httpServer := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	server := &memorySearchServer{
		runtimeFile: runtimeFile,
		listener:    listener,
		httpServer:  httpServer,
	}

	runtime := RuntimeState{
		RootPath:  manager.RootPath(),
		Address:   listener.Addr().String(),
		PID:       os.Getpid(),
		StartedAt: time.Now().UTC(),
	}
	if err := SaveRuntimeState(runtimeFile, runtime); err != nil {
		_ = listener.Close()
		return nil, err
	}

	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()

	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			logErrorf("Memory search server failed: %v", err)
		}
	}()

	return server, nil
}

func formatSearchLogQuery(query string, max int) string {
	if max <= 0 {
		max = defaultSearchLogQueryMax
	}
	normalized := strings.Join(strings.Fields(strings.TrimSpace(query)), " ")
	runes := []rune(normalized)
	if len(runes) <= max {
		return normalized
	}
	return string(runes[:max]) + "..."
}

func (s *memorySearchServer) Address() string {
	if s == nil || s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *memorySearchServer) Close() error {
	if s == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.httpServer.Shutdown(ctx)
	_ = s.listener.Close()
	_ = os.Remove(s.runtimeFile)
	return nil
}
