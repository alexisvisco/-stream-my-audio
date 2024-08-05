package encode

import (
	"context"
	"io"
)

type Encoder interface {
	Encode(ctx context.Context, recordOut io.ReadCloser) (io.ReadCloser, error)
	Name() string
}
