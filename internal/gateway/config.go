package gateway

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the gateway's runtime configuration, read from the environment.
type Config struct {
	// Port the gateway listens on.
	Port int
	// RootDir is passed to the logseq CLI as --root-dir; it must match wherever
	// this container's graph files actually live.
	RootDir string
	// Graphs is the allowlist of graph names this gateway will serve. Requests
	// for any other graph are rejected with 404 before shelling out.
	Graphs []string
	// LogseqBin is the logseq CLI executable to invoke.
	LogseqBin string
	// SubprocessTimeout bounds each `logseq server start`/`server list` call.
	SubprocessTimeout time.Duration
}

// LoadConfig reads configuration from environment variables.
//
//	GATEWAY_PORT             listen port (default 8085)
//	GATEWAY_ROOT_DIR         logseq CLI root dir, required
//	GATEWAY_GRAPHS           comma-separated graph allowlist, required
//	GATEWAY_LOGSEQ_BIN       logseq CLI executable (default "logseq")
//	GATEWAY_SUBPROCESS_TIMEOUT_MS  timeout for each logseq CLI call (default 20000)
func LoadConfig() (Config, error) {
	cfg := Config{
		Port:              8085,
		LogseqBin:         "logseq",
		SubprocessTimeout: 20 * time.Second,
	}

	if v := os.Getenv("GATEWAY_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("GATEWAY_PORT: %w", err)
		}
		cfg.Port = port
	}

	cfg.RootDir = os.Getenv("GATEWAY_ROOT_DIR")
	if cfg.RootDir == "" {
		return Config{}, fmt.Errorf("GATEWAY_ROOT_DIR is required")
	}

	graphsEnv := os.Getenv("GATEWAY_GRAPHS")
	if graphsEnv == "" {
		return Config{}, fmt.Errorf("GATEWAY_GRAPHS is required (comma-separated graph allowlist)")
	}
	for _, g := range strings.Split(graphsEnv, ",") {
		g = strings.TrimSpace(g)
		if g != "" {
			cfg.Graphs = append(cfg.Graphs, g)
		}
	}
	if len(cfg.Graphs) == 0 {
		return Config{}, fmt.Errorf("GATEWAY_GRAPHS is required (comma-separated graph allowlist)")
	}

	if v := os.Getenv("GATEWAY_LOGSEQ_BIN"); v != "" {
		cfg.LogseqBin = v
	}

	if v := os.Getenv("GATEWAY_SUBPROCESS_TIMEOUT_MS"); v != "" {
		ms, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("GATEWAY_SUBPROCESS_TIMEOUT_MS: %w", err)
		}
		cfg.SubprocessTimeout = time.Duration(ms) * time.Millisecond
	}

	return cfg, nil
}

// Allowed reports whether graph is in the configured allowlist.
func (c Config) Allowed(graph string) bool {
	for _, g := range c.Graphs {
		if g == graph {
			return true
		}
	}
	return false
}
