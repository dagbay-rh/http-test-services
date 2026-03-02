package handlers

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"
)

// SSEHandler streams Server-Sent Events with a ping every 3 seconds.
func SSEHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	log.Println("Starting SSE stream...")
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			log.Println("Closing SSE connection...")
			return
		default:
			log.Println("Sending stream data to client...")
			fmt.Fprintf(w, "event: 'ping'\n\n")
			fmt.Fprintf(w, "data: {\"data\": \"%f\"}\n\n", rand.Float64())
			flusher.Flush()
			time.Sleep(3 * time.Second)
		}
	}
}
