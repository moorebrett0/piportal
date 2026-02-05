package main

import (
	"encoding/base64"
	"encoding/json"
)

// Message type constants
const (
	MessageTypeAuth       = "auth"
	MessageTypePing       = "ping"
	MessageTypeResponse   = "response"
	MessageTypeAuthResult = "auth_result"
	MessageTypeRequest    = "request"
	MessageTypePong       = "pong"
	MessageTypeError      = "error"
	MessageTypeMetrics    = "metrics"
	MessageTypeCommand    = "command"

	// Terminal message types
	MessageTypeTerminalOpen   = "terminal_open"
	MessageTypeTerminalData   = "terminal_data"
	MessageTypeTerminalResize = "terminal_resize"
	MessageTypeTerminalClose  = "terminal_close"
)

// BaseMessage is used to peek at the "type" field
type BaseMessage struct {
	Type string `json:"type"`
}

// --- Client -> Server Messages ---

// AuthMessage is sent by the client to authenticate
type AuthMessage struct {
	Type          string `json:"type"`
	Token         string `json:"token"`
	ClientVersion string `json:"client_version"`
}

// ResponseMessage is the client's response to a proxied request
type ResponseMessage struct {
	Type       string            `json:"type"`
	RequestID  string            `json:"request_id"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	BodyBase64 string            `json:"body_base64,omitempty"`
}

// GetBody decodes the base64 body
func (r *ResponseMessage) GetBody() ([]byte, error) {
	if r.BodyBase64 == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(r.BodyBase64)
}

// PingMessage is a client heartbeat
type PingMessage struct {
	Type string `json:"type"`
}

// --- Server -> Client Messages ---

// AuthResultMessage tells the client if auth succeeded
type AuthResultMessage struct {
	Type      string `json:"type"`
	Success   bool   `json:"success"`
	Subdomain string `json:"subdomain,omitempty"`
	Message   string `json:"message,omitempty"`
}

func NewAuthResult(success bool, subdomain, message string) AuthResultMessage {
	return AuthResultMessage{
		Type:      MessageTypeAuthResult,
		Success:   success,
		Subdomain: subdomain,
		Message:   message,
	}
}

// RequestMessage sends an HTTP request to the client
type RequestMessage struct {
	Type       string            `json:"type"`
	RequestID  string            `json:"request_id"`
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Headers    map[string]string `json:"headers"`
	BodyBase64 string            `json:"body_base64,omitempty"`
}

func NewRequestMessage(requestID, method, path string, headers map[string]string, body []byte) RequestMessage {
	var bodyBase64 string
	if len(body) > 0 {
		bodyBase64 = base64.StdEncoding.EncodeToString(body)
	}
	return RequestMessage{
		Type:       MessageTypeRequest,
		RequestID:  requestID,
		Method:     method,
		Path:       path,
		Headers:    headers,
		BodyBase64: bodyBase64,
	}
}

// PongMessage responds to client ping
type PongMessage struct {
	Type string `json:"type"`
}

func NewPongMessage() PongMessage {
	return PongMessage{Type: MessageTypePong}
}

// ErrorMessage indicates an error
type ErrorMessage struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewErrorMessage(code, message string) ErrorMessage {
	return ErrorMessage{
		Type:    MessageTypeError,
		Code:    code,
		Message: message,
	}
}

// MetricsMessage contains system metrics from the client
type MetricsMessage struct {
	Type      string  `json:"type"`
	CPUTemp   float64 `json:"cpu_temp"`
	MemTotal  uint64  `json:"mem_total"`
	MemFree   uint64  `json:"mem_free"`
	DiskTotal uint64  `json:"disk_total"`
	DiskFree  uint64  `json:"disk_free"`
	Uptime    int64   `json:"uptime"`
	LoadAvg   float64 `json:"load_avg"`
}

// CommandMessage sends a command to the client
type CommandMessage struct {
	Type      string `json:"type"`
	CommandID string `json:"command_id"`
	Command   string `json:"command"`
}

func NewCommandMessage(commandID, command string) CommandMessage {
	return CommandMessage{
		Type:      MessageTypeCommand,
		CommandID: commandID,
		Command:   command,
	}
}

// --- Terminal Messages (Server <-> Client) ---

// TerminalOpenMessage tells the client to open a PTY session
type TerminalOpenMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
	Rows      int    `json:"rows"`
	Cols      int    `json:"cols"`
}

func NewTerminalOpenMessage(sessionID string, rows, cols int) TerminalOpenMessage {
	return TerminalOpenMessage{
		Type:      MessageTypeTerminalOpen,
		SessionID: sessionID,
		Rows:      rows,
		Cols:      cols,
	}
}

// TerminalDataMessage carries terminal I/O data (bidirectional)
type TerminalDataMessage struct {
	Type       string `json:"type"`
	SessionID  string `json:"session_id"`
	DataBase64 string `json:"data_base64"`
}

func NewTerminalDataMessage(sessionID string, data []byte) TerminalDataMessage {
	return TerminalDataMessage{
		Type:       MessageTypeTerminalData,
		SessionID:  sessionID,
		DataBase64: base64.StdEncoding.EncodeToString(data),
	}
}

// TerminalResizeMessage tells the client to resize the PTY
type TerminalResizeMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
	Rows      int    `json:"rows"`
	Cols      int    `json:"cols"`
}

func NewTerminalResizeMessage(sessionID string, rows, cols int) TerminalResizeMessage {
	return TerminalResizeMessage{
		Type:      MessageTypeTerminalResize,
		SessionID: sessionID,
		Rows:      rows,
		Cols:      cols,
	}
}

// TerminalCloseMessage closes a terminal session (bidirectional)
type TerminalCloseMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
}

func NewTerminalCloseMessage(sessionID string) TerminalCloseMessage {
	return TerminalCloseMessage{
		Type:      MessageTypeTerminalClose,
		SessionID: sessionID,
	}
}

// ParseClientMessage parses a message from the client
func ParseClientMessage(data []byte) (interface{}, string, error) {
	var base BaseMessage
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, "", err
	}

	var msg interface{}
	var err error

	switch base.Type {
	case MessageTypeAuth:
		var m AuthMessage
		err = json.Unmarshal(data, &m)
		msg = m
	case MessageTypeResponse:
		var m ResponseMessage
		err = json.Unmarshal(data, &m)
		msg = m
	case MessageTypePing:
		msg = PingMessage{Type: MessageTypePing}
	case MessageTypeMetrics:
		var m MetricsMessage
		err = json.Unmarshal(data, &m)
		msg = m
	case MessageTypeTerminalData:
		var m TerminalDataMessage
		err = json.Unmarshal(data, &m)
		msg = m
	case MessageTypeTerminalClose:
		var m TerminalCloseMessage
		err = json.Unmarshal(data, &m)
		msg = m
	default:
		msg = base
	}

	return msg, base.Type, err
}
