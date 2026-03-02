package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

type memorySearchServer struct {
	runtimeFile string
	listener    net.Listener
	httpServer  *http.Server
}

type memorySearchRequest struct {
	Query string        `json:"query"`
	Opts  SearchOptions `json:"opts"`
}

type memorySearchResponse struct {
	Results []SearchResult `json:"results"`
}

func startMemorySearchServer(ctx context.Context, manager *IndexManager, runtimeFile string) (*memorySearchServer, error) {
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

		var req memorySearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		results := manager.Search(req.Query, req.Opts)
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
