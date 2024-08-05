package stream

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"
)

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func newContextReader(ctx context.Context, r io.Reader) *contextReader {
	return &contextReader{ctx, r}
}

func (r *contextReader) Read(p []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, fmt.Errorf("context reader canceled: %w", r.ctx.Err())
	default:
		return r.r.Read(p)
	}
}

type debugReader struct {
	kind     string
	r        io.Reader
	lastCall time.Time
}

func newDebugReader(kind string, r io.Reader) *debugReader {
	return &debugReader{kind, r, time.Now()}
}

func (r *debugReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	slog.Info("read", "kind", r.kind, "bytes", n, "duration", time.Since(r.lastCall))
	return n, err
}
