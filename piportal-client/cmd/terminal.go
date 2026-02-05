package cmd

import (
	"encoding/base64"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// TerminalSession represents an active PTY session
type TerminalSession struct {
	ID      string
	cmd     *exec.Cmd
	ptmx    *os.File
	tunnel  *Tunnel
	closeCh chan struct{}
	once    sync.Once
}

// TerminalManager manages all active terminal sessions for a tunnel
type TerminalManager struct {
	sessions map[string]*TerminalSession
	mu       sync.Mutex
	tunnel   *Tunnel
}

// NewTerminalManager creates a new terminal manager
func NewTerminalManager(tunnel *Tunnel) *TerminalManager {
	return &TerminalManager{
		sessions: make(map[string]*TerminalSession),
		tunnel:   tunnel,
	}
}

// HandleOpen creates a new PTY session
func (tm *TerminalManager) HandleOpen(msg TerminalOpenMessage) {
	tm.mu.Lock()
	// Close existing session with same ID if any
	if existing, ok := tm.sessions[msg.SessionID]; ok {
		existing.close()
		delete(tm.sessions, msg.SessionID)
	}
	tm.mu.Unlock()

	shell := getShell()
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(msg.Rows),
		Cols: uint16(msg.Cols),
	})
	if err != nil {
		log.Printf("Terminal %s: failed to start PTY: %v", msg.SessionID, err)
		tm.tunnel.sendJSON(NewTerminalCloseMessage(msg.SessionID))
		return
	}

	session := &TerminalSession{
		ID:      msg.SessionID,
		cmd:     cmd,
		ptmx:    ptmx,
		tunnel:  tm.tunnel,
		closeCh: make(chan struct{}),
	}

	tm.mu.Lock()
	tm.sessions[msg.SessionID] = session
	tm.mu.Unlock()

	log.Printf("Terminal %s: PTY started (shell: %s, %dx%d)", msg.SessionID, shell, msg.Cols, msg.Rows)

	// Read PTY output and send to server
	go session.readLoop()

	// Wait for process to exit, then clean up
	go func() {
		_ = cmd.Wait()
		log.Printf("Terminal %s: process exited", msg.SessionID)
		session.close()
		tm.tunnel.sendJSON(NewTerminalCloseMessage(msg.SessionID))
		tm.mu.Lock()
		delete(tm.sessions, msg.SessionID)
		tm.mu.Unlock()
	}()
}

// HandleData writes incoming data to the PTY stdin
func (tm *TerminalManager) HandleData(msg TerminalDataMessage) {
	tm.mu.Lock()
	session, ok := tm.sessions[msg.SessionID]
	tm.mu.Unlock()
	if !ok {
		return
	}

	data, err := base64.StdEncoding.DecodeString(msg.DataBase64)
	if err != nil {
		log.Printf("Terminal %s: decode error: %v", msg.SessionID, err)
		return
	}

	if _, err := session.ptmx.Write(data); err != nil {
		log.Printf("Terminal %s: write error: %v", msg.SessionID, err)
	}
}

// HandleResize resizes the PTY
func (tm *TerminalManager) HandleResize(msg TerminalResizeMessage) {
	tm.mu.Lock()
	session, ok := tm.sessions[msg.SessionID]
	tm.mu.Unlock()
	if !ok {
		return
	}

	if err := pty.Setsize(session.ptmx, &pty.Winsize{
		Rows: uint16(msg.Rows),
		Cols: uint16(msg.Cols),
	}); err != nil {
		log.Printf("Terminal %s: resize error: %v", msg.SessionID, err)
	}
}

// HandleClose closes a terminal session
func (tm *TerminalManager) HandleClose(msg TerminalCloseMessage) {
	tm.mu.Lock()
	session, ok := tm.sessions[msg.SessionID]
	if ok {
		delete(tm.sessions, msg.SessionID)
	}
	tm.mu.Unlock()

	if ok {
		session.close()
		log.Printf("Terminal %s: closed by server", msg.SessionID)
	}
}

// CloseAll closes all active terminal sessions
func (tm *TerminalManager) CloseAll() {
	tm.mu.Lock()
	sessions := make([]*TerminalSession, 0, len(tm.sessions))
	for _, s := range tm.sessions {
		sessions = append(sessions, s)
	}
	tm.sessions = make(map[string]*TerminalSession)
	tm.mu.Unlock()

	for _, s := range sessions {
		s.close()
	}
}

func (s *TerminalSession) readLoop() {
	buf := make([]byte, 4096)
	for {
		select {
		case <-s.closeCh:
			return
		default:
		}

		n, err := s.ptmx.Read(buf)
		if n > 0 {
			msg := NewTerminalDataMessage(s.ID, buf[:n])
			if sendErr := s.tunnel.sendJSON(msg); sendErr != nil {
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("Terminal %s: read error: %v", s.ID, err)
			}
			return
		}
	}
}

func (s *TerminalSession) close() {
	s.once.Do(func() {
		close(s.closeCh)
		s.ptmx.Close()
		if s.cmd.Process != nil {
			s.cmd.Process.Kill()
		}
	})
}

func getShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	// Check common shells
	for _, sh := range []string{"/bin/bash", "/bin/sh"} {
		if _, err := os.Stat(sh); err == nil {
			return sh
		}
	}
	return "/bin/sh"
}
