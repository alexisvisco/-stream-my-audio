package stream

import (
	"context"
	"github.com/alexisvisco/stream-my-audio/encode"
	"github.com/alexisvisco/stream-my-audio/pactl"
	"github.com/alexisvisco/stream-my-audio/record"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	sinkInputPollInterval = 250 * time.Millisecond
)

type Stream struct {
	ctx context.Context

	encoder  encode.Encoder
	recorder record.RecorderFactory

	encodedAudio io.Reader

	onNewSinkInput <-chan *pactl.SinkInput

	onStreamReady chan struct{}
	ready         bool

	mutex sync.RWMutex
}

func NewStream(
	ctx context.Context,
	appName string,
	encoder encode.Encoder,
	recorder record.RecorderFactory,
) *Stream {
	s := &Stream{
		ctx:           ctx,
		encoder:       encoder,
		recorder:      recorder,
		onStreamReady: make(chan struct{}),
	}

	s.onNewSinkInput = s.initSinkInput(appName)
	go s.handle()

	return s
}

func (s *Stream) Read(p []byte) (n int, err error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.encodedAudio == nil {
		return 0, io.EOF
	}

	return s.encodedAudio.Read(p)
}

func (s *Stream) handle() {
	var (
		cancel    context.CancelFunc
		iteration int
	)

	for {
		select {
		case <-s.ctx.Done():
			if cancel != nil {
				cancel()
			}
			return
		case sinkInput, ok := <-s.onNewSinkInput:
			var ctx context.Context

			if !ok {
				// mean the channel is closed so we stop the stream
				if cancel != nil {
					cancel()
				}

				return
			}

			if cancel != nil {
				cancel()
			}

			iteration++
			ctx, cancel = context.WithCancel(s.ctx)

			go s.startAudioStreaming(ctx, sinkInput)
		}
	}
}

func (s *Stream) startAudioStreaming(ctx context.Context, sinkInput *pactl.SinkInput) {
	recorder := s.recorder(sinkInput.ID)

	recordOut, err := recorder.Record(ctx)
	if err != nil {
		slog.Error("Failed to start record process", slog.String("error", err.Error()))
		return
	}

	encodedAudio, err := s.encoder.Encode(ctx, recordOut)
	if err != nil {
		slog.Error("Failed to start encoding process", slog.String("error", err.Error()))
		return
	}

	s.setEncodedAudio(ctx, encodedAudio)
}

func (s *Stream) setEncodedAudio(ctx context.Context, encodedAudio io.ReadCloser) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.encodedAudio = newDebugReader("encoded audio", newContextReader(ctx, encodedAudio))
	if !s.ready {
		s.onStreamReady <- struct{}{}
		close(s.onStreamReady)
	}
	s.ready = true
}

func (s *Stream) Ready() <-chan struct{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.onStreamReady
}

func (s *Stream) initSinkInput(appName string) <-chan *pactl.SinkInput {
	ch := make(chan *pactl.SinkInput)

	var currentSinkInputId int

	go func() {
		for {
			select {
			case <-s.ctx.Done():
				close(ch)
				return
			default:
				sinkInputs, err := pactl.GetSinkInputs()
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
			time.Sleep(sinkInputPollInterval)
		}
	}()

	return ch
}
