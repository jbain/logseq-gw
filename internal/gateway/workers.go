package gateway

import (
	"context"
	"sync"
)

// WorkerManager caches graph -> base-url resolutions and knows how to
// refresh a stale or dead entry.
type WorkerManager struct {
	resolver *Resolver

	mu      sync.Mutex
	byGraph map[string]string
}

func NewWorkerManager(resolver *Resolver) *WorkerManager {
	return &WorkerManager{
		resolver: resolver,
		byGraph:  make(map[string]string),
	}
}

// BaseURL returns the cached base-url for graph, resolving and caching it if
// this is the first request for that graph.
func (m *WorkerManager) BaseURL(ctx context.Context, graph string) (string, error) {
	m.mu.Lock()
	if url, ok := m.byGraph[graph]; ok {
		m.mu.Unlock()
		return url, nil
	}
	m.mu.Unlock()

	return m.Refresh(ctx, graph)
}

// Refresh re-resolves graph's base-url, overwriting any cached entry.
// Call this after a connection failure or a 409 repo-mismatch response.
func (m *WorkerManager) Refresh(ctx context.Context, graph string) (string, error) {
	url, err := m.resolver.Resolve(ctx, graph)
	if err != nil {
		return "", err
	}

	m.mu.Lock()
	m.byGraph[graph] = url
	m.mu.Unlock()

	return url, nil
}

// Invalidate drops any cached entry for graph.
func (m *WorkerManager) Invalidate(graph string) {
	m.mu.Lock()
	delete(m.byGraph, graph)
	m.mu.Unlock()
}
