package gateway

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeRunner records the args it was called with and returns canned output
// per call, in order.
type fakeRunner struct {
	calls   [][]string
	outputs []fakeOutput
	i       int
}

type fakeOutput struct {
	out []byte
	err error
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	if f.i >= len(f.outputs) {
		return nil, errors.New("fakeRunner: no more canned outputs")
	}
	o := f.outputs[f.i]
	f.i++
	return o.out, o.err
}

const startOK = `{"status":"ok","data":{"repo":"logseq_db_demo","owner-source":"cli","owned":true}}`

const listOK = `{"status":"ok","data":{"servers":[
	{"repo":"logseq_db_other","graph":"other","base-url":"http://127.0.0.1:11111"},
	{"repo":"logseq_db_demo","graph":"demo","base-url":"http://127.0.0.1:22222"}
]}}`

func TestResolverResolve(t *testing.T) {
	runner := &fakeRunner{outputs: []fakeOutput{
		{out: []byte(startOK)},
		{out: []byte(listOK)},
	}}
	r := &Resolver{Runner: runner, LogseqBin: "logseq", RootDir: "/root"}

	url, err := r.Resolve(context.Background(), "demo")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if url != "http://127.0.0.1:22222" {
		t.Fatalf("got base-url %q", url)
	}

	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 subprocess calls, got %d: %v", len(runner.calls), runner.calls)
	}
	startCall := strings.Join(runner.calls[0], " ")
	if !strings.Contains(startCall, "-g demo") || !strings.Contains(startCall, "server start") {
		t.Errorf("unexpected start call: %s", startCall)
	}
	listCall := strings.Join(runner.calls[1], " ")
	if !strings.Contains(listCall, "server list") {
		t.Errorf("unexpected list call: %s", listCall)
	}
}

func TestResolverGraphNotInList(t *testing.T) {
	runner := &fakeRunner{outputs: []fakeOutput{
		{out: []byte(startOK)},
		{out: []byte(`{"status":"ok","data":{"servers":[]}}`)},
	}}
	r := &Resolver{Runner: runner, LogseqBin: "logseq", RootDir: "/root"}

	_, err := r.Resolve(context.Background(), "demo")
	if err == nil {
		t.Fatal("expected error when graph is absent from server list")
	}
}

func TestResolverStartError(t *testing.T) {
	runner := &fakeRunner{outputs: []fakeOutput{
		{out: []byte(`{"status":"error","error":{"message":"boom"}}`)},
	}}
	r := &Resolver{Runner: runner, LogseqBin: "logseq", RootDir: "/root"}

	_, err := r.Resolve(context.Background(), "demo")
	if err == nil {
		t.Fatal("expected error when server start reports status error")
	}
	// Only one call: list should not run after start fails.
	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 subprocess call, got %d", len(runner.calls))
	}
}
