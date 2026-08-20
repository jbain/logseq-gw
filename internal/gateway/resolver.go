package gateway

import (
	"context"
	"encoding/json"
	"fmt"
)

// Resolver finds the base-url of a graph's db-worker-node by shelling out to
// the logseq CLI: `server start` to ensure/reuse a worker, then `server list`
// to read back its address (server start's own JSON payload has no base-url).
type Resolver struct {
	Runner    Runner
	LogseqBin string
	RootDir   string
}

type envelope struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
	Error  json.RawMessage `json:"error"`
}

type serverListData struct {
	Servers []struct {
		Graph   string `json:"graph"`
		BaseURL string `json:"base-url"`
	} `json:"servers"`
}

// Resolve ensures a worker is running for graph and returns its base-url.
func (r *Resolver) Resolve(ctx context.Context, graph string) (string, error) {
	if _, err := r.run(ctx, "-g", graph, "server", "start", "-o", "json"); err != nil {
		return "", fmt.Errorf("server start for graph %q: %w", graph, err)
	}

	listOut, err := r.run(ctx, "server", "list", "-o", "json")
	if err != nil {
		return "", fmt.Errorf("server list: %w", err)
	}

	var data serverListData
	if err := json.Unmarshal(listOut, &data); err != nil {
		return "", fmt.Errorf("server list: parsing servers: %w", err)
	}

	for _, s := range data.Servers {
		if s.Graph == graph {
			if s.BaseURL == "" {
				return "", fmt.Errorf("server list: graph %q has no base-url", graph)
			}
			return s.BaseURL, nil
		}
	}
	return "", fmt.Errorf("server list: no running server found for graph %q", graph)
}

// run invokes the logseq CLI with --root-dir and the given args, and unwraps
// the {"status":"ok","data":...} / {"status":"error","error":...} envelope
// that every `-o json` command emits.
func (r *Resolver) run(ctx context.Context, args ...string) (json.RawMessage, error) {
	fullArgs := append([]string{"--root-dir", r.RootDir}, args...)
	out, err := r.Runner.Run(ctx, r.LogseqBin, fullArgs...)
	if err != nil {
		return nil, err
	}

	var env envelope
	if err := json.Unmarshal(out, &env); err != nil {
		return nil, fmt.Errorf("parsing logseq output: %w", err)
	}
	if env.Status != "ok" {
		return nil, fmt.Errorf("logseq error: %s", string(env.Error))
	}
	return env.Data, nil
}
