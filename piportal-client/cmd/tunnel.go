package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os/exec"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// TunnelState represents the current connection state
type TunnelState int

const (
	StateInit TunnelState = iota
	StateConnecting
	StateConnected
	StateBackoff
)

func (s TunnelState) String() string {
	switch s {
	case StateInit:
		return "INIT"
	case StateConnecting:
		return "CONNECTING"
	case StateConnected:
		return "CONNECTED"
	case StateBackoff:
		return "BACKOFF"
	default:
		return "UNKNOWN"
	}
}

// Tunnel manages the WebSocket connection to the server
type Tunnel struct {
	config    *Config
	proxy     *Proxy
	conn      *websocket.Conn
	state     TunnelState
	subdomain string
	terminals *TerminalManager

	backoffDelay   time.Duration
	connectedSince time.Time

	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

// NewTunnel creates a new tunnel manager
func NewTunnel(config *Config) *Tunnel {
	ctx, cancel := context.WithCancel(context.Background())
	t := &Tunnel{
		config:       config,
		proxy:        NewProxy(fmt.Sprintf("%s:%d", config.LocalHost, config.LocalPort)),
		state:        StateInit,
		backoffDelay: time.Second,
		ctx:          ctx,
		cancel:       cancel,
	}
	t.terminals = NewTerminalManager(t)
	return t
}

// Run starts the tunnel and keeps it connected
func (t *Tunnel) Run() error {
	for {
		select {
		case <-t.ctx.Done():
			return nil
		default:
			t.runOnce()
		}
	}
}

func (t *Tunnel) runOnce() {
	t.setState(StateConnecting)
	log.Printf("Connecting to %s...", t.config.Server)

	conn, _, err := websocket.DefaultDialer.DialContext(t.ctx, t.config.Server, nil)
	if err != nil {
		log.Printf("Connection failed: %v", err)
		t.backoff()
		return
	}

	t.mu.Lock()
	t.conn = conn
	t.mu.Unlock()

	if err := t.authenticate(); err != nil {
		log.Printf("Authentication failed: %v", err)
		conn.Close()
		t.backoff()
		return
	}

	t.setState(StateConnected)
	t.connectedSince = time.Now()

	// Update subdomain from auth response if we got one
	if t.subdomain != "" {
		fmt.Printf("  ✓ Connected: https://%s.piportal.dev\n", t.subdomain)
	} else {
		fmt.Println("  ✓ Connected!")
	}
	fmt.Println()

	go t.pingLoop()
	t.messageLoop()
	t.terminals.CloseAll()

	if time.Since(t.connectedSince) > 5*time.Minute {
		t.backoffDelay = time.Second
	}
}

func (t *Tunnel) authenticate() error {
	authMsg := NewAuthMessage(t.config.Token, Version)
	if err := t.sendJSON(authMsg); err != nil {
		return fmt.Errorf("failed to send auth: %w", err)
	}

	_, data, err := t.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read auth result: %w", err)
	}

	msg, msgType, err := ParseMessage(data)
	if err != nil {
		return fmt.Errorf("failed to parse auth result: %w", err)
	}

	switch msgType {
	case MessageTypeAuthResult:
		result := msg.(AuthResultMessage)
		if !result.Success {
			return fmt.Errorf("auth rejected: %s", result.Message)
		}
		t.subdomain = result.Subdomain
		return nil
	case MessageTypeError:
		errMsg := msg.(ErrorMessage)
		return fmt.Errorf("server error: %s - %s", errMsg.Code, errMsg.Message)
	default:
		return fmt.Errorf("unexpected response type: %s", msgType)
	}
}

func (t *Tunnel) messageLoop() {
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
		}

		_, data, err := t.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Println("Server closed connection")
			} else {
				log.Printf("Connection lost: %v", err)
			}
			return
		}

		msg, msgType, err := ParseMessage(data)
		if err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		switch msgType {
		case MessageTypeRequest:
			req := msg.(RequestMessage)
			go t.handleRequest(&req)
		case MessageTypePong:
			// OK
		case MessageTypeCommand:
			cmd := msg.(CommandMessage)
			go t.handleCommand(&cmd)
		case MessageTypeError:
			errMsg := msg.(ErrorMessage)
			log.Printf("Server error: %s - %s", errMsg.Code, errMsg.Message)
		case MessageTypeTerminalOpen:
			m := msg.(TerminalOpenMessage)
			go t.terminals.HandleOpen(m)
		case MessageTypeTerminalData:
			m := msg.(TerminalDataMessage)
			t.terminals.HandleData(m)
		case MessageTypeTerminalResize:
			m := msg.(TerminalResizeMessage)
			t.terminals.HandleResize(m)
		case MessageTypeTerminalClose:
			m := msg.(TerminalCloseMessage)
			t.terminals.HandleClose(m)
		}
	}
}

func (t *Tunnel) handleRequest(req *RequestMessage) {
	log.Printf("← %s %s", req.Method, req.Path)

	result, err := t.proxy.Forward(t.ctx, req)
	if err != nil {
		log.Printf("  ✗ %v", err)
		resp := NewResponseMessage(req.RequestID, 502, map[string]string{
			"Content-Type": "text/plain",
		}, []byte(fmt.Sprintf("Failed to reach local service: %v", err)))
		t.sendJSON(resp)
		return
	}

	log.Printf("→ %d %s", result.StatusCode, req.Path)

	resp := NewResponseMessage(req.RequestID, result.StatusCode, result.Headers, result.Body)
	if err := t.sendJSON(resp); err != nil {
		log.Printf("Failed to send response: %v", err)
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
			if t.state != StateConnected {
				return
			}
			if err := t.sendJSON(NewPingMessage()); err != nil {
				return
			}
			// Send system metrics alongside ping
			metrics := CollectMetrics()
			if err := t.sendJSON(metrics); err != nil {
				return
			}
		}
	}
}

func (t *Tunnel) handleCommand(cmd *CommandMessage) {
	log.Printf("Received command: %s (id: %s)", cmd.Command, cmd.CommandID)
	switch cmd.Command {
	case "reboot":
		log.Println("Reboot command received, rebooting system...")
		if err := exec.Command("sudo", "reboot").Run(); err != nil {
			log.Printf("Reboot failed: %v", err)
		}
	case "exec":
		t.handleExecCommand(cmd)
	default:
		log.Printf("Unknown command: %s", cmd.Command)
	}
}

const maxOutputBytes = 64 * 1024 // 64 KB output cap

func (t *Tunnel) handleExecCommand(cmd *CommandMessage) {
	shell := cmd.Shell
	if shell == "" {
		result := NewCommandResultMessage(cmd.CommandID, -1, "", "no shell command provided")
		t.sendJSON(result)
		return
	}

	if cmd.DryRun {
		// For apt-get/apt commands, rewrite with -s (simulate) flag
		if isAptCommand(shell) {
			shell = insertAptSimulate(shell)
		} else {
			// For non-apt commands, just echo what would run
			output := fmt.Sprintf("[dry run] would execute: %s", shell)
			result := NewCommandResultMessage(cmd.CommandID, 0, base64Encode([]byte(output)), "")
			t.sendJSON(result)
			return
		}
	}

	log.Printf("Executing shell command: %s (dry_run=%v)", shell, cmd.DryRun)

	ctx, cancel := context.WithTimeout(t.ctx, 60*time.Second)
	defer cancel()

	execCmd := exec.CommandContext(ctx, "sh", "-c", shell)
	outputBytes, err := execCmd.CombinedOutput()

	// Cap output at 64 KB
	if len(outputBytes) > maxOutputBytes {
		outputBytes = outputBytes[:maxOutputBytes]
	}

	exitCode := 0
	errMsg := ""
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			exitCode = -1
			errMsg = "command timed out after 60s"
		} else {
			exitCode = -1
			errMsg = err.Error()
		}
	}

	result := NewCommandResultMessage(cmd.CommandID, exitCode, base64Encode(outputBytes), errMsg)
	if sendErr := t.sendJSON(result); sendErr != nil {
		log.Printf("Failed to send command result: %v", sendErr)
	}
}

func isAptCommand(cmd string) bool {
	// Check if command starts with apt-get or apt
	for _, prefix := range []string{"apt-get ", "apt "} {
		if len(cmd) >= len(prefix) && cmd[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func insertAptSimulate(cmd string) string {
	// Insert -s flag after "apt-get" or "apt"
	if len(cmd) >= 8 && cmd[:8] == "apt-get " {
		return "apt-get -s " + cmd[8:]
	}
	if len(cmd) >= 4 && cmd[:4] == "apt " {
		return "apt -s " + cmd[4:]
	}
	return cmd
}

func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func (t *Tunnel) backoff() {
	t.setState(StateBackoff)

	jitter := 1.0 + (rand.Float64()*0.4 - 0.2)
	delay := time.Duration(float64(t.backoffDelay) * jitter)

	log.Printf("Reconnecting in %v...", delay.Round(time.Second))

	select {
	case <-t.ctx.Done():
		return
	case <-time.After(delay):
	}

	t.backoffDelay = time.Duration(math.Min(
		float64(t.backoffDelay*2),
		float64(60*time.Second),
	))
}

func (t *Tunnel) sendJSON(msg interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn == nil {
		return fmt.Errorf("not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return t.conn.WriteMessage(websocket.TextMessage, data)
}

func (t *Tunnel) setState(state TunnelState) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state = state
}

// Stop gracefully shuts down the tunnel
func (t *Tunnel) Stop() {
	t.cancel()
	t.mu.Lock()
	if t.conn != nil {
		t.conn.Close()
	}
	t.mu.Unlock()
}
