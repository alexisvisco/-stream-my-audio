package record

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

type ParecRecorder struct {
	sinkID int
}

func NewParecRecorder(sinkID int) *ParecRecorder {
	return &ParecRecorder{sinkID: sinkID}
}

func (r *ParecRecorder) Record(ctx context.Context) (io.ReadCloser, error) {
	recordCmd := exec.CommandContext(ctx, "parec",
		fmt.Sprintf("--monitor-stream=%d", r.sinkID),
		"--latency-msec=100")
	recordOut, err := recordCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get parec stdout pipe: %w", err)
	}
	if err := recordCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start parec process: %w", err)
	}
	return recordOut, nil
}

func (r *ParecRecorder) Name() string {
	return "parec"
}

func ParecFactory(sinkID int) Recorder {
	return NewParecRecorder(sinkID)
}
