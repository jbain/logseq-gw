package gateway

import (
	"context"
	"testing"
)

func TestWorkerManagerCachesResolution(t *testing.T) {
	runner := &fakeRunner{outputs: []fakeOutput{
		{out: []byte(startOK)}, {out: []byte(listOK)}, // only one resolution expected
	}}
	resolver := &Resolver{Runner: runner, LogseqBin: "logseq", RootDir: "/root"}
	m := NewWorkerManager(resolver)

	url1, err := m.BaseURL(context.Background(), "demo")
	if err != nil {
		t.Fatalf("BaseURL: %v", err)
	}
	url2, err := m.BaseURL(context.Background(), "demo")
	if err != nil {
		t.Fatalf("BaseURL (cached): %v", err)
	}
	if url1 != url2 {
		t.Fatalf("cached url mismatch: %q vs %q", url1, url2)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("expected exactly 2 subprocess calls (1 resolution), got %d", len(runner.calls))
	}
}

func TestWorkerManagerInvalidateForcesReResolve(t *testing.T) {
	runner := &fakeRunner{outputs: []fakeOutput{
		{out: []byte(startOK)}, {out: []byte(listOK)},
		{out: []byte(startOK)}, {out: []byte(listOK)},
	}}
	resolver := &Resolver{Runner: runner, LogseqBin: "logseq", RootDir: "/root"}
	m := NewWorkerManager(resolver)

	if _, err := m.BaseURL(context.Background(), "demo"); err != nil {
		t.Fatalf("BaseURL: %v", err)
	}
	m.Invalidate("demo")
	if _, err := m.BaseURL(context.Background(), "demo"); err != nil {
		t.Fatalf("BaseURL after invalidate: %v", err)
	}
	if len(runner.calls) != 4 {
		t.Fatalf("expected 4 subprocess calls (2 resolutions), got %d", len(runner.calls))
	}
}
