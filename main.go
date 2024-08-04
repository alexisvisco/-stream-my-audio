package main

import (
	"context"
	"fmt"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// GetSinkInputs retrieves the list of sink input IDs from PulseAudio.
func GetSinkInputs() ([]*SinkInput, error) {
	cmd := exec.Command("pactl", "list", "sink-inputs")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	pactlOutput, err := parsePactlOutput(string(output))
	if err != nil {
		return nil, err
	}

	return pactlOutput, nil
}

// StreamAudio streams audio from the specified sink input ID over HTTP.
func StreamAudio(w http.ResponseWriter, r *http.Request, input *SinkInput) {
	w.Header().Set("Content-Type", "audio/mpeg")
	id := gonanoid.Must(8)
	iteration := 0
	requestContext := r.Context()
	sinkInputChan := notifyForNewSinkID(requestContext, input)

	slog.Info("Starting audio stream", slog.String("id", id))

	var cancel context.CancelFunc
	for {
		iteration++
		select {
		case <-requestContext.Done():
			if cancel != nil {
				cancel()
			}

			slog.Info("End audio stream", slog.String("id", id))
			return
		case newInput := <-sinkInputChan:
			if cancel != nil {
				cancel() // Cancel the previous context
			}
			input = newInput
			cancelCtx, newCancel := context.WithCancel(requestContext)
			cancel = newCancel
			handleNewSinkInput(cancelCtx, w, input, id, iteration)
		}
	}

}

// handleNewSinkInput handles the new sink input ID and starts streaming audio.
func handleNewSinkInput(
	parentCtx context.Context,
	w http.ResponseWriter,
	input *SinkInput,
	id string,
	iteration int,
) {
	logContext := slog.Group("context",
		slog.Int("sink_id", input.ID),
		slog.Int("iteration", iteration),
		slog.String("id", id))

	go startAudioStreaming(parentCtx, w, input.ID, logContext)

	slog.Info("Stop audio stream", logContext)
}

// startAudioStreaming starts the audio streaming process.
func startAudioStreaming(ctx context.Context, w http.ResponseWriter, sinkID int, logContext slog.Attr) {
	recordOut, err := startRecordProcess(ctx, sinkID)
	if err != nil {
		slog.Error("Failed to start record process", logContext, slog.String("error", err.Error()))
		return
	}

	encodingOut, err := startEncodingProcess(ctx, recordOut)
	if err != nil {
		slog.Error("Failed to start encoding process", logContext, slog.String("error", err.Error()))
		return
	}

	streamAudioToResponse(ctx, w, encodingOut, logContext)
}

// startRecordProcess starts the parec process and returns the stdout pipe.
func startRecordProcess(ctx context.Context, sinkID int) (io.ReadCloser, error) {
	recordCmd := exec.CommandContext(ctx, "parec", "--monitor-stream="+strconv.Itoa(sinkID))
	recordOut, err := recordCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get parec stdout pipe: %w", err)
	}
	if err := recordCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start parec process: %w", err)
	}
	return recordOut, nil
}

// startEncodingProcess starts the lame process and returns the stdout pipe.
func startEncodingProcess(ctx context.Context, recordOut io.ReadCloser) (io.ReadCloser, error) {
	encodingCmd := exec.CommandContext(ctx, "lame", "-r", "-", "-")
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

// streamAudioToResponse streams the audio from the encoding process to the HTTP response writer.
func streamAudioToResponse(
	ctx context.Context,
	w http.ResponseWriter,
	encodingOut io.ReadCloser,
	logContext slog.Attr,
) {
	ctxReader := newContextReader(ctx, encodingOut)
	if _, err := io.Copy(w, ctxReader); err != nil {
		slog.Error("Failed to stream audio", logContext, slog.String("error", err.Error()))
	}
}

func main() {
	sinkInputs, err := GetSinkInputs()
	if err != nil {
		slog.Info("Failed to get sink input ID: %v", err)
		os.Exit(1)
	}

	if len(sinkInputs) == 0 {
		slog.Info("No sink inputs found")
		os.Exit(1)
	}

	var firefoxInput *SinkInput
	for _, s := range sinkInputs {
		if strings.ToLower(s.Properties.ApplicationName) == "firefox" {
			firefoxInput = s
			break
		}
	}

	if firefoxInput == nil {
		slog.Info("No Firefox sink input found")
		os.Exit(1)
	}

	// Define the HTTP handler
	http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		StreamAudio(w, r, firefoxInput)
	})

	srv := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  0,
		WriteTimeout: 0,
		IdleTimeout:  0,
	}

	slog.Info("Starting HTTP server on port 8080")
	// Start the HTTP server
	log.Fatal(srv.ListenAndServe())
}

func notifyForNewSinkID(ctx context.Context, current *SinkInput) <-chan *SinkInput {
	ch := make(chan *SinkInput)
	appName := strings.ToLower(current.Properties.ApplicationName)
	currentSinkInputId := current.ID
	go func() {
		ch <- current
		for {
			select {
			case <-ctx.Done():
				close(ch)
				return
			default:
				sinkInputs, err := GetSinkInputs()
				if err != nil {
					slog.Info("Failed to get sink input ID: %v", err)
					continue
				}

				for _, s := range sinkInputs {
					if strings.ToLower(s.Properties.ApplicationName) == appName && s.ID != currentSinkInputId {
						ch <- s
						currentSinkInputId = s.ID
						break
					}
				}
			}
			time.Sleep(250 * time.Millisecond)
		}
	}()

	return ch
}
