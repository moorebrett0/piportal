package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// handleTerminalWebSocket handles browser WebSocket connections for terminal access
func (h *Handler) handleTerminalWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract device ID from path: /api/v1/devices/{id}/terminal
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/devices/"), "/")
	if len(parts) < 2 {
		jsonError(w, "Invalid path", http.StatusBadRequest)
		return
	}
	deviceID := parts[0]

	// Authenticate via JWT cookie (same as AuthMiddleware but we can't use it for WS upgrades)
	var tokenStr string
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		tokenStr = strings.TrimPrefix(auth, "Bearer ")
	}
	if tokenStr == "" {
		if cookie, err := r.Cookie("token"); err == nil {
			tokenStr = cookie.Value
		}
	}
	if tokenStr == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	userID, err := ValidateJWT(tokenStr, h.config.JWTSecret)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	// Validate device ownership
	device, err := h.store.GetDeviceByID(deviceID)
	if err != nil {
		log.Printf("Terminal: device lookup error: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if device == nil || device.UserID != user.ID {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}

	// Check device is online
	tunnel := h.tunnels.GetTunnel(device.Subdomain)
	if tunnel == nil {
		http.Error(w, "Device is offline", http.StatusConflict)
		return
	}

	// Upgrade to WebSocket
	browserConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Terminal: WebSocket upgrade failed: %v", err)
		return
	}

	// Generate session ID
	sessionID := generateSessionID()
	log.Printf("Terminal session %s: opened for device %s by user %s", sessionID, device.Subdomain, user.Email)

	// Register browser connection with tunnel
	tunnel.RegisterTerminalSession(sessionID, browserConn)

	// Read initial size from browser (first message)
	rows, cols := 24, 80 // defaults
	_, initMsg, err := browserConn.ReadMessage()
	if err == nil {
		var init struct {
			Rows int `json:"rows"`
			Cols int `json:"cols"`
		}
		if json.Unmarshal(initMsg, &init) == nil && init.Rows > 0 && init.Cols > 0 {
			rows = init.Rows
			cols = init.Cols
		}
	}

	// Send terminal_open to Pi client
	openMsg := NewTerminalOpenMessage(sessionID, rows, cols)
	if err := tunnel.SendJSON(openMsg); err != nil {
		log.Printf("Terminal session %s: failed to send open to client: %v", sessionID, err)
		tunnel.UnregisterTerminalSession(sessionID)
		browserConn.Close()
		return
	}

	// Bridge: read from browser WS, forward to tunnel as terminal_data
	defer func() {
		tunnel.UnregisterTerminalSession(sessionID)
		// Tell client to close the session
		tunnel.SendJSON(NewTerminalCloseMessage(sessionID))
		browserConn.Close()
		log.Printf("Terminal session %s: closed", sessionID)
	}()

	for {
		msgType, data, err := browserConn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("Terminal session %s: browser read error: %v", sessionID, err)
			}
			return
		}

		if msgType == websocket.TextMessage {
			// Check if it's a resize message
			var base BaseMessage
			if json.Unmarshal(data, &base) == nil && base.Type == "resize" {
				var resize struct {
					Type string `json:"type"`
					Rows int    `json:"rows"`
					Cols int    `json:"cols"`
				}
				if json.Unmarshal(data, &resize) == nil {
					tunnel.SendJSON(NewTerminalResizeMessage(sessionID, resize.Rows, resize.Cols))
				}
				continue
			}

			// Otherwise it's terminal input data - forward as terminal_data
			// Browser sends base64-encoded data in a JSON envelope
			var inputMsg struct {
				Data string `json:"data"`
			}
			if json.Unmarshal(data, &inputMsg) == nil && inputMsg.Data != "" {
				tunnel.SendJSON(NewTerminalDataMessage(sessionID, []byte(inputMsg.Data)))
			}
		}
	}
}

func generateSessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "term_" + hex.EncodeToString(b)
}
