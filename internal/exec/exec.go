// Package exec abstracts running subprocesses (version queries, daemon control)
// so the store and updater can be tested against a fake executor.
package exec

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Executor runs a command and returns trimmed stdout ("" on failure).
type Executor interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// Stopper terminates all running processes matching a binary name. This is the
// default "stop daemon" mechanism when an app has no explicit stop command.
type Stopper func(binaryName string) error

// ProcessStop kills every process whose name equals binaryName (cross-platform).
// Used to stop self-daemonized fleet apps; returns nil if nothing matched.
func ProcessStop(binaryName string) error {
	if runtime.GOOS == "windows" {
		_, err := exec.Command("taskkill", "/IM", binaryName+".exe", "/F").CombinedOutput()
		if err != nil {
			return fmt.Errorf("taskkill %s: %w", binaryName, err)
		}
		return nil
	}
	out, err := exec.Command("pkill", "-x", binaryName).CombinedOutput()
	if err != nil {
		// pkill exits 1 when nothing matched — not an error for our purposes.
		return nil
	}
	_ = out
	return nil
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
