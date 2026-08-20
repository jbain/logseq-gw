package gateway

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"time"
)

// hopByHopHeaders are stripped before forwarding a request or response, per
// RFC 7230 6.1 — they describe this specific connection, not the resource.
var hopByHopHeaders = []string{
	"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
	"Te", "Trailers", "Transfer-Encoding", "Upgrade",
}

// Proxy forwards graph-scoped requests to the resolved db-worker-node,
// retrying once (after invalidating the cache) on a connection failure or a
// 409 repo-mismatch — both signs the cached base-url no longer points at the
// worker for this graph.
type Proxy struct {
	Workers *WorkerManager
	// HTTPClient sends the proxied request. It should have no overall
	// timeout — /v1/events is a long-lived SSE stream.
	HTTPClient *http.Client
	// ResolveTimeout bounds each worker-resolution subprocess round trip
	// (`logseq server start` + `server list`), independent of how long the
	// proxied request itself may run.
	ResolveTimeout time.Duration
}

func (p *Proxy) resolveBaseURL(ctx context.Context, graph string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, p.ResolveTimeout)
	defer cancel()
	return p.Workers.BaseURL(ctx, graph)
}

// ServeUpstream proxies r to upstreamPath on graph's worker and writes the
// response to w. It streams the response body unmodified so SSE
// (/v1/events) passes through without added buffering.
func (p *Proxy) ServeUpstream(w http.ResponseWriter, r *http.Request, graph, upstreamPath string) {
	ctx := r.Context()

	// Buffer the request body so it can be replayed on a retry.
	var body []byte
	if r.Body != nil {
		var err error
		body, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "reading request body", http.StatusBadGateway)
			return
		}
	}

	baseURL, err := p.resolveBaseURL(ctx, graph)
	if err != nil {
		log.Printf("resolve %s: %v", graph, err)
		http.Error(w, "resolving graph worker", http.StatusBadGateway)
		return
	}

	resp, err := p.doUpstream(ctx, r, baseURL+upstreamPath, body)
	if err != nil {
		// Connection-level failure: the cached worker is likely dead or the
		// container restarted. Invalidate, re-resolve, and retry once.
		log.Printf("upstream %s%s: %v; re-resolving %s", baseURL, upstreamPath, err, graph)
		p.Workers.Invalidate(graph)
		baseURL, err = p.resolveBaseURL(ctx, graph)
		if err != nil {
			http.Error(w, "resolving graph worker", http.StatusBadGateway)
			return
		}
		resp, err = p.doUpstream(ctx, r, baseURL+upstreamPath, body)
		if err != nil {
			http.Error(w, "upstream request failed", http.StatusBadGateway)
			return
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		// The cached port now serves a different graph's worker (port
		// reuse after a restart/cleanup elsewhere). Invalidate, re-resolve,
		// and retry once; fall back to the original response if that fails.
		log.Printf("upstream %s%s: 409, likely stale worker for %s; re-resolving", baseURL, upstreamPath, graph)
		p.Workers.Invalidate(graph)
		if newBaseURL, rerr := p.resolveBaseURL(ctx, graph); rerr == nil {
			if retryResp, rerr := p.doUpstream(ctx, r, newBaseURL+upstreamPath, body); rerr == nil {
				resp.Body.Close()
				resp = retryResp
				defer resp.Body.Close()
			}
		}
	}

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	flusher, _ := w.(http.Flusher)
	writer := io.Writer(w)
	if flusher != nil {
		writer = flushWriter{w: w, f: flusher}
	}
	if _, err := io.Copy(writer, resp.Body); err != nil {
		log.Printf("streaming response for %s: %v", graph, err)
	}
}

func (p *Proxy) doUpstream(ctx context.Context, orig *http.Request, url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, orig.Method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	copyHeader(req.Header, orig.Header)
	return p.HTTPClient.Do(req)
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		if isHopByHop(k) {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func isHopByHop(header string) bool {
	for _, h := range hopByHopHeaders {
		if http.CanonicalHeaderKey(header) == h {
			return true
		}
	}
	return false
}

// flushWriter flushes after every write so a streaming response (SSE) is
// forwarded to the client with no added buffering.
type flushWriter struct {
	w io.Writer
	f http.Flusher
}

func (fw flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if n > 0 {
		fw.f.Flush()
	}
	return n, err
}
