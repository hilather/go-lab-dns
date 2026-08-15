package testutil

import (
	"context"
	"io"
	"testing"
)

// Cleanup registers fn on t so helpers can attach teardown that still runs on failure.
func Cleanup(t testing.TB, fn func()) {
	t.Helper()
	t.Cleanup(fn)
}

// MustClose registers c.Close on cleanup and fails the test if Close errors.
func MustClose(t testing.TB, c io.Closer) {
	t.Helper()
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
}

// Context returns a context canceled when the test ends.
func Context(t testing.TB) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}
