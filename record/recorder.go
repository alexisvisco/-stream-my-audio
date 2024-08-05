package record

import (
	"context"
	"io"
)

type Recorder interface {
	Record(ctx context.Context) (io.ReadCloser, error)
	Name() string
}

type RecorderFactory = func(sinkID int) Recorder
