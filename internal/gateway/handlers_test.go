package gateway

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testConfig(graphs ...string) Config {
	return Config{Port: 0, RootDir: "/root", Graphs: graphs, LogseqBin: "logseq"}
}

func listJSON(graph, baseURL string) string {
	return fmt.Sprintf(`{"status":"ok","data":{"servers":[{"repo":"logseq_db_x","graph":%q,"base-url":%q}]}}`, graph, baseURL)
}

func newTestProxy(runner *fakeRunner) *Proxy {
	resolver := &Resolver{Runner: runner, LogseqBin: "logseq", RootDir: "/root"}
	return &Proxy{
		Workers:        NewWorkerManager(resolver),
		HTTPClient:     &http.Client{Timeout: 2 * time.Second},
		ResolveTimeout: 2 * time.Second,
	}
}

func TestGraphsEndpointReturnsAllowlist(t *testing.T) {
	cfg := testConfig("demo", "other")
	mux := NewMux(cfg, newTestProxy(&fakeRunner{}))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/graphs", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if body != `{"graphs":["demo","other"]}`+"\n" {
		t.Fatalf("body = %q", body)
	}
}

func TestInvokeRejectsGraphNotInAllowlist(t *testing.T) {
	cfg := testConfig("demo")
	mux := NewMux(cfg, newTestProxy(&fakeRunner{}))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/graphs/not-allowed/v1/invoke", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestInvokeProxiesToResolvedWorker(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/invoke" {
			t.Errorf("upstream got path %q", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"q":1}` {
			t.Errorf("upstream got body %q", body)
		}
		w.Header().Set("X-From-Upstream", "yes")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	runner := &fakeRunner{outputs: []fakeOutput{
		{out: []byte(startOK)},
		{out: []byte(listJSON("demo", upstream.URL))},
	}}
	cfg := testConfig("demo")
	mux := NewMux(cfg, newTestProxy(runner))

	req := httptest.NewRequest("POST", "/graphs/demo/v1/invoke", strings.NewReader(`{"q":1}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != `{"ok":true}` {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if rec.Header().Get("X-From-Upstream") != "yes" {
		t.Fatalf("upstream header not forwarded")
	}
}

func TestInvokeRetriesOnceAfterConnectionFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("recovered"))
	}))
	defer upstream.Close()

	runner := &fakeRunner{outputs: []fakeOutput{
		{out: []byte(startOK)},
		{out: []byte(listJSON("demo", "http://127.0.0.1:1"))}, // unreachable: connection refused
		{out: []byte(startOK)},
		{out: []byte(listJSON("demo", upstream.URL))},
	}}
	cfg := testConfig("demo")
	mux := NewMux(cfg, newTestProxy(runner))

	req := httptest.NewRequest("POST", "/graphs/demo/v1/invoke", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "recovered" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestInvokeRetriesOnceAfter409RepoMismatch(t *testing.T) {
	stale := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"repo-mismatch"}`))
	}))
	defer stale.Close()
	fresh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fresh"))
	}))
	defer fresh.Close()

	runner := &fakeRunner{outputs: []fakeOutput{
		{out: []byte(startOK)},
		{out: []byte(listJSON("demo", stale.URL))},
		{out: []byte(startOK)},
		{out: []byte(listJSON("demo", fresh.URL))},
	}}
	cfg := testConfig("demo")
	mux := NewMux(cfg, newTestProxy(runner))

	req := httptest.NewRequest("POST", "/graphs/demo/v1/invoke", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "fresh" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestHealthz(t *testing.T) {
	mux := NewMux(testConfig("demo"), newTestProxy(&fakeRunner{}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}
