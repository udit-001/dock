package gui

import (
	"bytes"
	"context"

	"github.com/udit-001/app-store/internal/exec"
)

// execOS adapts exec.OSExecutor to the Executor interface.
type execOS struct{}

func (execOS) Run(ctx context.Context, name string, args ...string) (string, error) {
	return exec.OSExecutor{}.Run(ctx, name, args...)
}

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }