package cmd

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
	MessageTypeCommand       = "command"
	MessageTypeCommandResult = "command_result"

	// Terminal message types
	MessageTypeTerminalOpen   = "terminal_open"
	MessageTypeTerminalData   = "terminal_data"
	MessageTypeTerminalResize = "terminal_resize"
	MessageTypeTerminalClose  = "terminal_close"
)

// BaseMessage is used to peek at the "type" field before full parsing
type BaseMessage struct {
	Type string `json:"type"`
}

// AuthMessage is sent immediately after connecting
type AuthMessage struct {
	Type          string `json:"type"`
	Token         string `json:"token"`
	ClientVersion string `json:"client_version"`
}

func NewAuthMessage(token, version string) AuthMessage {
	return AuthMessage{
		Type:          MessageTypeAuth,
		Token:         token,
		ClientVersion: version,
	}
}

// ResponseMessage sends the result of a proxied request back to the server
type ResponseMessage struct {
	Type       string            `json:"type"`
	RequestID  string            `json:"request_id"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	BodyBase64 string            `json:"body_base64,omitempty"`
}

func NewResponseMessage(requestID string, statusCode int, headers map[string]string, body []byte) ResponseMessage {
	var bodyBase64 string
	if len(body) > 0 {
		bodyBase64 = base64.StdEncoding.EncodeToString(body)
	}
	return ResponseMessage{
		Type:       MessageTypeResponse,
		RequestID:  requestID,
		StatusCode: statusCode,
		Headers:    headers,
		BodyBase64: bodyBase64,
	}
}

// PingMessage is a heartbeat
type PingMessage struct {
	Type string `json:"type"`
}

func NewPingMessage() PingMessage {
	return PingMessage{Type: MessageTypePing}
}

// AuthResultMessage is the server's response to authentication
type AuthResultMessage struct {
	Type      string `json:"type"`
	Success   bool   `json:"success"`
	Subdomain string `json:"subdomain,omitempty"`
	Message   string `json:"message,omitempty"`
}

// RequestMessage is an incoming HTTP request to forward
type RequestMessage struct {
	Type       string            `json:"type"`
	RequestID  string            `json:"request_id"`
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Headers    map[string]string `json:"headers"`
	BodyBase64 string            `json:"body_base64,omitempty"`
}

func (r *RequestMessage) GetBody() ([]byte, error) {
	if r.BodyBase64 == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(r.BodyBase64)
}

// ErrorMessage indicates something went wrong
type ErrorMessage struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// MetricsMessage reports system metrics to the server
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

// CommandMessage is a command sent from the server to the client
type CommandMessage struct {
	Type      string `json:"type"`
	CommandID string `json:"command_id"`
	Command   string `json:"command"`
	Shell     string `json:"shell,omitempty"`
	DryRun    bool   `json:"dry_run,omitempty"`
}

// CommandResultMessage is sent back to the server after executing a command
type CommandResultMessage struct {
	Type      string `json:"type"`
	CommandID string `json:"command_id"`
	ExitCode  int    `json:"exit_code"`
	Output    string `json:"output"`
	Error     string `json:"error,omitempty"`
}

// NewCommandResultMessage creates a new command result message
func NewCommandResultMessage(commandID string, exitCode int, output, errMsg string) CommandResultMessage {
	return CommandResultMessage{
		Type:      MessageTypeCommandResult,
		CommandID: commandID,
		ExitCode:  exitCode,
		Output:    output,
		Error:     errMsg,
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

// ParseMessage parses a raw JSON message
func ParseMessage(data []byte) (interface{}, string, error) {
	var base BaseMessage
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, "", err
	}

	var msg interface{}
	var err error

	switch base.Type {
	case MessageTypeAuthResult:
		var m AuthResultMessage
		err = json.Unmarshal(data, &m)
		msg = m
	case MessageTypeRequest:
		var m RequestMessage
		err = json.Unmarshal(data, &m)
		msg = m
	case MessageTypePong:
		msg = base
	case MessageTypeError:
		var m ErrorMessage
		err = json.Unmarshal(data, &m)
		msg = m
	case MessageTypeCommand:
		var m CommandMessage
		err = json.Unmarshal(data, &m)
		msg = m
	case MessageTypeTerminalOpen:
		var m TerminalOpenMessage
		err = json.Unmarshal(data, &m)
		msg = m
	case MessageTypeTerminalData:
		var m TerminalDataMessage
		err = json.Unmarshal(data, &m)
		msg = m
	case MessageTypeTerminalResize:
		var m TerminalResizeMessage
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
