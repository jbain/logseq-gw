package gateway

import (
	"context"
	"os/exec"
)

// Runner runs an external command and returns its stdout. It exists so
// resolution logic can be tested against a fake without a real logseq binary.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ExecRunner runs commands via os/exec.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	// CombinedOutput so a non-JSON failure message (e.g. binary not found,
	// crash before it could print JSON) is visible in the returned error.
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, &commandError{err: err, stderr: string(exitErr.Stderr)}
		}
		return nil, err
	}
	return out, nil
}

type commandError struct {
	err    error
	stderr string
}

func (e *commandError) Error() string {
	if e.stderr == "" {
		return e.err.Error()
	}
	return e.err.Error() + ": " + e.stderr
}

func (e *commandError) Unwrap() error { return e.err }
