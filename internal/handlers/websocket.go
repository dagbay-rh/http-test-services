package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"nhooyr.io/websocket"
)

// WebSocketHandler upgrades to WebSocket and echoes messages back.
// If the request is not a WebSocket upgrade, returns a JSON message.
func WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		log.Println("not a websocket request")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "not a websocket request"})
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.CloseNow()
	log.Println("Opening WebSocket connection...")

	ctx := r.Context()
	for {
		msgType, data, err := conn.Read(ctx)
		if err != nil {
			log.Printf("Closing WebSocket connection: %v", err)
			return
		}
		log.Printf("Sending message: %s", data)
		if err := conn.Write(ctx, msgType, data); err != nil {
			log.Printf("WebSocket write error: %v", err)
			return
		}
	}
}
