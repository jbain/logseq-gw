// Command logseq-gw is a small HTTP gateway that lets sandbox containers
// with no local graph files reach a logseq graph's db-worker-node, by
// proxying to it over a fixed port with path-based routing
// (/graphs/:graph/v1/invoke, /graphs/:graph/v1/events) and serving graph
// discovery at GET /graphs.
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/jbain/logseq-gw/internal/gateway"
)

func main() {
	cfg, err := gateway.LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	resolver := &gateway.Resolver{
		Runner:    gateway.ExecRunner{},
		LogseqBin: cfg.LogseqBin,
		RootDir:   cfg.RootDir,
	}
	workers := gateway.NewWorkerManager(resolver)
	proxy := &gateway.Proxy{
		Workers: workers,
		// No overall timeout: /v1/events is a long-lived SSE stream that
		// must survive idle periods.
		HTTPClient:     &http.Client{},
		ResolveTimeout: cfg.SubprocessTimeout,
	}

	mux := gateway.NewMux(cfg, proxy)

	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
		// Deliberately no Read/Write/Idle timeouts: /v1/events (SSE) has no
		// heartbeat and can sit idle indefinitely between events.
	}

	log.Printf("logseq-gw listening on %s (graphs: %v, root-dir: %s)", addr, cfg.Graphs, cfg.RootDir)
	log.Fatal(srv.ListenAndServe())
}
