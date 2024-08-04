package main

import (
	"context"
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

	var cancel context.CancelFunc
	iteration := 0
	requestContext := r.Context()
	sinkInputChan := notifyForNewSinkID(requestContext, input)

	slog.Info("Starting audio stream", slog.String("id", id))

	for {
		iteration++
		select {
		case <-requestContext.Done():
			if cancel != nil {
				cancel()
			}
			return
		case newInput := <-sinkInputChan:
			if iteration > 1 && input.ID == newInput.ID {
				// when it's above the first iteration, and the new input ID is the same as the current one,
				// we skip the iteration
				continue
			}

			if cancel != nil {
				cancel()
			}

			input = newInput

			slog.Info("Using sink input", slog.String("id", id), slog.Int("sinkID", input.ID))

			// Start a new context for the new sink input ID
			var newCtx context.Context
			newCtx, cancel = context.WithCancel(requestContext)

			go func(sinkID int, iteration int, ctx context.Context) {
				logContext := slog.GroupValue(
					slog.Int("sinkID", sinkID),
					slog.Int("iteration", iteration),
					slog.String("id", id))

				parecCmd := exec.CommandContext(ctx, "parec", "--monitor-stream="+strconv.Itoa(sinkID))
				parecOut, err := parecCmd.StdoutPipe()
				if err != nil {
					slog.Info("Failed to start parec: %v",
						logContext,
						slog.String("error", err.Error()))

					return
				}

				if err := parecCmd.Start(); err != nil {
					slog.Info("Failed to start parec: %v",
						logContext,
						slog.String("error", err.Error()))
					return
				}

				// Start lame process
				lameCmd := exec.CommandContext(ctx, "lame", "-r", "-", "-")
				lameCmd.Stdin = parecOut
				lameOut, err := lameCmd.StdoutPipe()
				if err != nil {
					slog.Info("Failed to start lame: %v",
						logContext,
						slog.String("error", err.Error()))
					return
				}

				if err := lameCmd.Start(); err != nil {
					slog.Info("Failed to start lame: %v",
						logContext,
						slog.String("error", err.Error()))
					return
				}

				ctxReader := newContextReader(ctx, lameOut)

				// Stream the audio to the HTTP response writer
				_, err = io.Copy(w, ctxReader)
				if err != nil {
					slog.Info("Failed to stream audio: %v",
						logContext,
						slog.String("error", err.Error()))
				}

				// Clean up the processes
				parecCmd.Process.Kill()
				lameCmd.Process.Kill()
			}(input.ID, iteration, newCtx)

		}
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
