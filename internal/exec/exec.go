// Package exec abstracts running subprocesses (version queries, daemon control)
// so the store and updater can be tested against a fake executor.
package exec

import (
	"context"
	"os/exec"
	"strings"
)

// Executor runs a command and returns trimmed stdout ("" on failure).
type Executor interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// OSExecutor runs commands on the local system.
type OSExecutor struct{}

func (OSExecutor) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// StaticExecutor always returns fixed output (for tests / placeholder).
type StaticExecutor struct {
	Out string
	Err error
}

func (s StaticExecutor) Run(context.Context, string, ...string) (string, error) {
	return s.Out, s.Err
}