package encode

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

type LameEncoder struct {
}

func NewLameEncoder() *LameEncoder {
	return &LameEncoder{}
}

func (e *LameEncoder) Encode(ctx context.Context, recordOut io.ReadCloser) (io.ReadCloser, error) {
	encodingCmd := exec.CommandContext(ctx, "lame",
		"-r",
		"-f",
		"--resample", "4096",
		"-", "-")
	encodingCmd.Stdin = recordOut

	encodingOut, err := encodingCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get lame stdout pipe: %w", err)
	}

	if err := encodingCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start lame process: %w", err)
	}

	return encodingOut, nil
}

func (e *LameEncoder) Name() string {
	return "lame"
}
