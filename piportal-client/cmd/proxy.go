package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Proxy handles forwarding requests to a local HTTP service
type Proxy struct {
	targetAddr string
	client     *http.Client
}

// NewProxy creates a proxy that forwards to the given address
func NewProxy(targetAddr string) *Proxy {
	return &Proxy{
		targetAddr: targetAddr,
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Timeout: 30 * time.Second,
		},
	}
}

// ProxyResult contains the response from the local service
type ProxyResult struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// Forward sends a request to the local service
func (p *Proxy) Forward(ctx context.Context, req *RequestMessage) (*ProxyResult, error) {
	url := fmt.Sprintf("http://%s%s", p.targetAddr, req.Path)

	body, err := req.GetBody()
	if err != nil {
		return nil, fmt.Errorf("failed to decode request body: %w", err)
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range req.Headers {
		if !isHopByHopHeader(key) {
			httpReq.Header.Set(key, value)
		}
	}

	httpReq.Header.Set("X-Forwarded-Proto", "https")
	httpReq.Header.Set("X-PiPortal", "true")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to reach local service: %w", err)
	}
	defer resp.Body.Close()

	const maxBodySize = 10 * 1024 * 1024
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 && !isHopByHopHeader(key) {
			headers[key] = values[0]
		}
	}

	return &ProxyResult{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       respBody,
	}, nil
}

func isHopByHopHeader(header string) bool {
	switch header {
	case "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
		"TE", "Trailers", "Transfer-Encoding", "Upgrade":
		return true
	}
	return false
}
