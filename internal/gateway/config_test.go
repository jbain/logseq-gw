package gateway

import "testing"

func TestLoadConfigGraphsOptional(t *testing.T) {
	t.Setenv("GATEWAY_ROOT_DIR", "/root")
	t.Setenv("GATEWAY_GRAPHS", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig without GATEWAY_GRAPHS: %v", err)
	}
	if len(cfg.Graphs) != 0 {
		t.Fatalf("expected no configured graphs, got %v", cfg.Graphs)
	}
	if !cfg.Allowed("anything") {
		t.Fatal("expected unset allowlist to allow any graph")
	}
}

func TestLoadConfigGraphsRestrictWhenSet(t *testing.T) {
	t.Setenv("GATEWAY_ROOT_DIR", "/root")
	t.Setenv("GATEWAY_GRAPHS", "a, b")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.Allowed("a") || !cfg.Allowed("b") {
		t.Fatal("expected configured graphs to be allowed")
	}
	if cfg.Allowed("c") {
		t.Fatal("expected graph outside allowlist to be rejected")
	}
}

func TestLoadConfigRequiresRootDir(t *testing.T) {
	t.Setenv("GATEWAY_ROOT_DIR", "")
	t.Setenv("GATEWAY_GRAPHS", "")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error when GATEWAY_ROOT_DIR is unset")
	}
}
