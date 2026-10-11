package contexting

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Server-side timeouts
const defaultReadHeaderTimeout = 3 * time.Second // Timeout for reading HTTP request headers (prevents slowloris)
const defaultShutdownTimeout = 2 * time.Second   // Timeout for graceful HTTP server shutdown

const defaultSearchLogQueryMax = 120

type memorySearchServer struct {
	listener     net.Listener
	httpServer   *http.Server
	registration *runtimeRegistration
	closeOnce    sync.Once
	closed       chan struct{}
	closeErr     error
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
	Results     []SearchResult `json:"results"`
	GeneratedAt time.Time      `json:"generated_at"`
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
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		if r.Header.Get("Origin") != "" {
			http.Error(w, "browser origins are not allowed", http.StatusForbidden)
			return
		}
		start := time.Now()

		var req memorySearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if logOpts.Enabled {
				LogWarnf("Search request rejected: invalid payload")
			}
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		results := manager.Search(req.Query, req.Opts)
		if logOpts.Enabled {
			LogInfof("Search query \"%s\" -> %d results in %dms", formatSearchLogQuery(req.Query, logOpts.QueryMax), len(results), time.Since(start).Milliseconds())
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(memorySearchResponse{Results: results, GeneratedAt: manager.IndexGeneratedAt()})
	})

	httpServer := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	server := &memorySearchServer{
		listener:   listener,
		httpServer: httpServer,
		closed:     make(chan struct{}),
	}

	runtime := RuntimeState{
		RootPath:  manager.RootPath(),
		Address:   listener.Addr().String(),
		PID:       os.Getpid(),
		StartedAt: time.Now().UTC(),
	}
	server.registration, err = registerRuntimeState(runtimeFile, runtime)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	if server.registration == nil {
		LogInfof("Another HTTP server owns discovery at %s; leaving its registration unchanged.", runtimeFile)
	}

	go func() {
		select {
		case <-ctx.Done():
			_ = server.Close()
		case <-server.closed:
		}
	}()

	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			LogErrorf("Memory search server failed: %v", err)
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
	s.closeOnce.Do(func() {
		defer close(s.closed)
		ctx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
		defer cancel()
		s.closeErr = s.httpServer.Shutdown(ctx)
		if s.closeErr != nil {
			_ = s.httpServer.Close()
		}
		_ = s.listener.Close()
		s.registration.close()
	})
	return s.closeErr
}
