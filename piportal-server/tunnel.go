package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// TunnelManager manages all active tunnel connections
type TunnelManager struct {
	tunnels map[string]*Tunnel // subdomain -> tunnel
	mu      sync.RWMutex
	store   *Store
}

// Tunnel represents a single client connection
type Tunnel struct {
	Device           *Device
	Conn             *websocket.Conn
	Manager          *TunnelManager
	Responses        map[string]chan *ResponseMessage        // requestID -> response channel
	CommandResults   map[string]chan *CommandResultMessage    // commandID -> result channel
	TerminalSessions map[string]*websocket.Conn              // sessionID -> browser WS conn
	Metrics          *MetricsMessage
	MetricsUpdatedAt time.Time
	mu               sync.Mutex
	ctx              context.Context
	cancel           context.CancelFunc
}

// PendingRequest tracks a request waiting for a response
type PendingRequest struct {
	ResponseChan chan *ResponseMessage
	CreatedAt    time.Time
}

// NewTunnelManager creates a new tunnel manager
func NewTunnelManager(store *Store) *TunnelManager {
	return &TunnelManager{
		tunnels: make(map[string]*Tunnel),
		store:   store,
	}
}

// GetTunnel returns the tunnel for a subdomain
func (tm *TunnelManager) GetTunnel(subdomain string) *Tunnel {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.tunnels[subdomain]
}

// RegisterTunnel adds a new tunnel
func (tm *TunnelManager) RegisterTunnel(tunnel *Tunnel) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Close existing tunnel for this subdomain if any
	if existing, ok := tm.tunnels[tunnel.Device.Subdomain]; ok {
		existing.Close()
	}

	tm.tunnels[tunnel.Device.Subdomain] = tunnel
	tm.store.UpdateDeviceStatus(tunnel.Device.ID, true)

	log.Printf("Tunnel registered: %s (device: %s)", tunnel.Device.Subdomain, tunnel.Device.ID[:8])
}

// UnregisterTunnel removes a tunnel
func (tm *TunnelManager) UnregisterTunnel(tunnel *Tunnel) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Only remove if it's the same tunnel (not a replacement)
	if current, ok := tm.tunnels[tunnel.Device.Subdomain]; ok && current == tunnel {
		delete(tm.tunnels, tunnel.Device.Subdomain)
		tm.store.UpdateDeviceStatus(tunnel.Device.ID, false)
		log.Printf("Tunnel unregistered: %s", tunnel.Device.Subdomain)
	}
}

// Stats returns tunnel statistics
func (tm *TunnelManager) Stats() map[string]interface{} {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	subdomains := make([]string, 0, len(tm.tunnels))
	for subdomain := range tm.tunnels {
		subdomains = append(subdomains, subdomain)
	}

	return map[string]interface{}{
		"active_tunnels": len(tm.tunnels),
		"subdomains":     subdomains,
	}
}

// --- Tunnel methods ---

// NewTunnel creates a new tunnel
func NewTunnel(device *Device, conn *websocket.Conn, manager *TunnelManager) *Tunnel {
	ctx, cancel := context.WithCancel(context.Background())
	return &Tunnel{
		Device:           device,
		Conn:             conn,
		Manager:          manager,
		Responses:        make(map[string]chan *ResponseMessage),
		CommandResults:   make(map[string]chan *CommandResultMessage),
		TerminalSessions: make(map[string]*websocket.Conn),
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Run handles the tunnel connection
func (t *Tunnel) Run() {
	defer t.Close()
	defer t.Manager.UnregisterTunnel(t)

	// Set up ping/pong
	t.Conn.SetPongHandler(func(string) error {
		t.Conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	// Start ping loop
	go t.pingLoop()

	// Read messages
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
		}

		t.Conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		_, data, err := t.Conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("Tunnel %s: client disconnected", t.Device.Subdomain)
			} else {
				log.Printf("Tunnel %s: read error: %v", t.Device.Subdomain, err)
			}
			return
		}

		t.handleMessage(data)
	}
}

func (t *Tunnel) handleMessage(data []byte) {
	msg, msgType, err := ParseClientMessage(data)
	if err != nil {
		log.Printf("Tunnel %s: parse error: %v", t.Device.Subdomain, err)
		return
	}

	switch msgType {
	case MessageTypeResponse:
		resp := msg.(ResponseMessage)
		t.mu.Lock()
		if ch, ok := t.Responses[resp.RequestID]; ok {
			select {
			case ch <- &resp:
			default:
			}
			delete(t.Responses, resp.RequestID)
		}
		t.mu.Unlock()

	case MessageTypePing:
		t.SendJSON(NewPongMessage())

	case MessageTypeMetrics:
		metrics := msg.(MetricsMessage)
		t.mu.Lock()
		t.Metrics = &metrics
		t.MetricsUpdatedAt = time.Now()
		t.mu.Unlock()

	case MessageTypeTerminalData:
		termData := msg.(TerminalDataMessage)
		t.forwardTerminalToBrowser(termData.SessionID, data)

	case MessageTypeTerminalClose:
		termClose := msg.(TerminalCloseMessage)
		t.closeTerminalSession(termClose.SessionID)

	case MessageTypeCommandResult:
		cmdResult := msg.(CommandResultMessage)
		t.mu.Lock()
		if ch, ok := t.CommandResults[cmdResult.CommandID]; ok {
			select {
			case ch <- &cmdResult:
			default:
			}
			delete(t.CommandResults, cmdResult.CommandID)
		}
		t.mu.Unlock()
	}
}

func (t *Tunnel) pingLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			if err := t.Conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
				return
			}
		}
	}
}

// ForwardRequest sends an HTTP request through the tunnel and waits for response
func (t *Tunnel) ForwardRequest(req *http.Request, requestID string) (*ResponseMessage, error) {
	// Build the request message
	headers := make(map[string]string)
	for key, values := range req.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Add forwarding headers
	if clientIP := req.Header.Get("X-Forwarded-For"); clientIP != "" {
		headers["X-Forwarded-For"] = clientIP
	} else if req.RemoteAddr != "" {
		headers["X-Forwarded-For"] = req.RemoteAddr
	}

	// Read request body
	var body []byte
	if req.Body != nil {
		body = make([]byte, 0)
		buf := make([]byte, 1024)
		for {
			n, err := req.Body.Read(buf)
			if n > 0 {
				body = append(body, buf[:n]...)
			}
			if err != nil {
				break
			}
			if len(body) > 10*1024*1024 { // 10MB limit
				return nil, fmt.Errorf("request body too large")
			}
		}
	}

	// Create response channel
	respChan := make(chan *ResponseMessage, 1)
	t.mu.Lock()
	t.Responses[requestID] = respChan
	t.mu.Unlock()

	// Clean up on exit
	defer func() {
		t.mu.Lock()
		delete(t.Responses, requestID)
		t.mu.Unlock()
	}()

	// Send request to client
	reqMsg := NewRequestMessage(requestID, req.Method, req.URL.Path+"?"+req.URL.RawQuery, headers, body)
	if err := t.SendJSON(reqMsg); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response with timeout
	select {
	case resp := <-respChan:
		return resp, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request timeout")
	case <-t.ctx.Done():
		return nil, fmt.Errorf("tunnel closed")
	}
}

// SendJSON sends a JSON message to the client
func (t *Tunnel) SendJSON(msg interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	t.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return t.Conn.WriteMessage(websocket.TextMessage, data)
}

// RegisterTerminalSession registers a browser WebSocket for a terminal session
func (t *Tunnel) RegisterTerminalSession(sessionID string, browserConn *websocket.Conn) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.TerminalSessions[sessionID] = browserConn
}

// UnregisterTerminalSession removes a terminal session
func (t *Tunnel) UnregisterTerminalSession(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.TerminalSessions, sessionID)
}

// forwardTerminalToBrowser forwards raw terminal data from the client to the browser WS
func (t *Tunnel) forwardTerminalToBrowser(sessionID string, rawMsg []byte) {
	t.mu.Lock()
	browserConn, ok := t.TerminalSessions[sessionID]
	t.mu.Unlock()
	if !ok {
		return
	}
	browserConn.WriteMessage(websocket.TextMessage, rawMsg)
}

// closeTerminalSession closes the browser WS for a terminal session
func (t *Tunnel) closeTerminalSession(sessionID string) {
	t.mu.Lock()
	browserConn, ok := t.TerminalSessions[sessionID]
	if ok {
		delete(t.TerminalSessions, sessionID)
	}
	t.mu.Unlock()
	if ok {
		browserConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session closed"))
		browserConn.Close()
	}
}

// Close closes the tunnel
func (t *Tunnel) Close() {
	t.cancel()
	// Close all terminal sessions
	t.mu.Lock()
	for sid, conn := range t.TerminalSessions {
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseGoingAway, "tunnel disconnected"))
		conn.Close()
		delete(t.TerminalSessions, sid)
	}
	t.mu.Unlock()
	t.Conn.Close()
}

// SendCommand sends a command to the client
func (t *Tunnel) SendCommand(command string) error {
	cmdID := fmt.Sprintf("cmd_%d", time.Now().UnixNano())
	return t.SendJSON(NewCommandMessage(cmdID, command))
}

// SendExecCommand sends a shell command to the client and waits for the result
func (t *Tunnel) SendExecCommand(shell string, dryRun bool) (*CommandResultMessage, error) {
	cmdID := fmt.Sprintf("cmd_%d", time.Now().UnixNano())

	// Create result channel
	resultChan := make(chan *CommandResultMessage, 1)
	t.mu.Lock()
	t.CommandResults[cmdID] = resultChan
	t.mu.Unlock()

	// Clean up on exit
	defer func() {
		t.mu.Lock()
		delete(t.CommandResults, cmdID)
		t.mu.Unlock()
	}()

	// Send exec command to client
	msg := NewExecCommand(cmdID, shell, dryRun)
	if err := t.SendJSON(msg); err != nil {
		return nil, fmt.Errorf("failed to send exec command: %w", err)
	}

	// Wait for result with 90s timeout
	select {
	case result := <-resultChan:
		return result, nil
	case <-time.After(90 * time.Second):
		return nil, fmt.Errorf("command timed out")
	case <-t.ctx.Done():
		return nil, fmt.Errorf("tunnel closed")
	}
}

// GetMetrics returns a copy of the latest metrics, or nil
func (t *Tunnel) GetMetrics() *MetricsMessage {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Metrics
}
