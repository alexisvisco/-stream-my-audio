package main

import (
	_ "embed"
	"fmt"
	"github.com/alexisvisco/stream-my-audio/encode"
	"github.com/alexisvisco/stream-my-audio/record"
	"github.com/alexisvisco/stream-my-audio/stream"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"io"
	"log"
	"log/slog"
	"net/http"
)

//go:embed index.html
var indexHTML string

// StreamAudio streams audio from the specified sink input ID over HTTP.
func StreamAudio(w http.ResponseWriter, r *http.Request) {
	fmt.Print("verb", r.Method)
	w.Header().Set("Content-Type", "audio/mpeg")
	id := gonanoid.Must(8)

	slog.Info("Streaming audio", "id", id)

	audioStream := stream.NewStream(r.Context(), "firefox", encode.NewLameEncoder(), record.ParecFactory)

	<-audioStream.Ready()

	if _, err := io.Copy(w, audioStream); err != nil {
		slog.Error("Failed to stream audio", "err", err)
	}

	slog.Info("Stopped streaming audio", "id", id)

}

func main() {
	// Define the HTTP handler
	http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		StreamAudio(w, r)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(indexHTML))
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
