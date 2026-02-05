package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for tunnel clients
	},
}

// Handler holds HTTP handlers
type Handler struct {
	config  *Config
	store   *Store
	tunnels *TunnelManager
}

// NewHandler creates a new handler
func NewHandler(config *Config, store *Store, tunnels *TunnelManager) *Handler {
	return &Handler{
		config:  config,
		store:   store,
		tunnels: tunnels,
	}
}

// ServeHTTP routes requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host

	// Remove port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Check if this is a tunnel WebSocket connection
	if r.URL.Path == "/tunnel" && websocket.IsWebSocketUpgrade(r) {
		h.handleTunnelConnect(w, r)
		return
	}

	// In dev mode, check for subdomain header/query param FIRST
	// This allows testing tunnel proxying via localhost
	if h.config.DevMode {
		if subdomain := r.Header.Get("X-PiPortal-Subdomain"); subdomain != "" {
			h.handleTunnelRequest(w, r, subdomain)
			return
		}
		if subdomain := r.URL.Query().Get("subdomain"); subdomain != "" {
			h.handleTunnelRequest(w, r, subdomain)
			return
		}
	}

	// Check if this is a subdomain request (e.g., mypi.piportal.dev)
	if strings.HasSuffix(host, "."+h.config.BaseDomain) {
		subdomain := strings.TrimSuffix(host, "."+h.config.BaseDomain)
		h.handleTunnelRequest(w, r, subdomain)
		return
	}

	// Check if this is the main domain
	isMainDomain := host == h.config.BaseDomain || host == "www."+h.config.BaseDomain
	if h.config.DevMode && (host == "localhost" || host == "127.0.0.1") {
		isMainDomain = true
	}
	if isMainDomain {
		h.handleMainSite(w, r)
		return
	}

	http.Error(w, "Not Found", http.StatusNotFound)
}

// handleTunnelConnect handles WebSocket connections from tunnel clients
func (h *Handler) handleTunnelConnect(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	log.Printf("New tunnel connection from %s", r.RemoteAddr)

	// Wait for auth message
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		log.Printf("Auth read failed: %v", err)
		conn.Close()
		return
	}

	msg, msgType, err := ParseClientMessage(data)
	if err != nil || msgType != MessageTypeAuth {
		sendError(conn, "invalid_message", "Expected auth message")
		conn.Close()
		return
	}

	authMsg := msg.(AuthMessage)

	// Validate token
	device, err := h.store.GetDeviceByToken(authMsg.Token)
	if err != nil {
		log.Printf("Token lookup error: %v", err)
		sendError(conn, "internal_error", "Token lookup failed")
		conn.Close()
		return
	}
	if device == nil {
		sendError(conn, "invalid_token", "Token not recognized")
		conn.Close()
		return
	}

	// Send success response
	sendJSON(conn, NewAuthResult(true, device.Subdomain, fmt.Sprintf("Connected as %s.%s", device.Subdomain, h.config.BaseDomain)))

	// Create and register tunnel
	tunnel := NewTunnel(device, conn, h.tunnels)
	h.tunnels.RegisterTunnel(tunnel)

	// Run the tunnel (blocks until disconnect)
	tunnel.Run()
}

// handleTunnelRequest proxies a request through a tunnel
func (h *Handler) handleTunnelRequest(w http.ResponseWriter, r *http.Request, subdomain string) {
	tunnel := h.tunnels.GetTunnel(subdomain)
	if tunnel == nil {
		// Check if device exists but is offline
		device, _ := h.store.GetDeviceBySubdomain(subdomain)
		if device != nil {
			http.Error(w, fmt.Sprintf("%s.%s is currently offline", subdomain, h.config.BaseDomain), http.StatusServiceUnavailable)
		} else {
			http.Error(w, "Tunnel not found", http.StatusNotFound)
		}
		return
	}

	// Check if tunnel forwarding is enabled
	if !tunnel.Device.TunnelEnabled {
		http.Error(w, "Tunnel forwarding is disabled", http.StatusForbidden)
		return
	}

	// Check bandwidth limit
	isOver, used, limit, err := h.store.IsOverBandwidthLimit(tunnel.Device.ID)
	if err != nil {
		log.Printf("Bandwidth check error: %v", err)
	} else if isOver {
		log.Printf("Bandwidth exceeded for %s: %s / %s", subdomain, FormatBytes(used), FormatBytes(limit))
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusPaymentRequired)
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Bandwidth Limit Exceeded</title></head>
<body style="font-family: system-ui; max-width: 500px; margin: 50px auto; text-align: center;">
<h1>Bandwidth Limit Exceeded</h1>
<p>This tunnel has used <strong>%s</strong> of its <strong>%s</strong> monthly limit.</p>
<p>The limit resets on the 1st of each month.</p>
<p><a href="https://%s/upgrade">Upgrade to Pro</a> for 100GB/month.</p>
</body>
</html>`, FormatBytes(used), FormatBytes(limit), h.config.BaseDomain)
		return
	}

	// Generate request ID
	requestID := generateRequestID()

	log.Printf("Proxying %s %s -> %s", r.Method, r.URL.Path, subdomain)

	// Forward request through tunnel
	resp, err := tunnel.ForwardRequest(r, requestID)
	if err != nil {
		log.Printf("Forward error: %v", err)
		http.Error(w, fmt.Sprintf("Tunnel error: %v", err), http.StatusBadGateway)
		return
	}

	// Get response body
	body, err := resp.GetBody()
	if err != nil {
		log.Printf("Body decode error: %v", err)
		return
	}

	// Track bandwidth (request + response)
	var requestSize int64 = int64(len(r.URL.String()) + 200) // Approximate request overhead
	if r.ContentLength > 0 {
		requestSize += r.ContentLength
	}
	responseSize := int64(len(body))
	h.store.AddBandwidth(tunnel.Device.ID, requestSize, responseSize)

	// Copy response headers
	for key, value := range resp.Headers {
		w.Header().Set(key, value)
	}

	// Write status code
	w.WriteHeader(resp.StatusCode)

	// Write body
	if body != nil {
		w.Write(body)
	}
}

// handleMainSite serves the main website/API
func (h *Handler) handleMainSite(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/":
		h.handleHome(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/v1/"):
		h.handleDashboardAPI(w, r)
	case r.URL.Path == "/api/register":
		h.handleRegister(w, r)
	case r.URL.Path == "/status":
		h.handleStatusPage(w, r)
	case r.URL.Path == "/api/status":
		h.handleStatus(w, r)
	case r.URL.Path == "/api/version":
		h.handleVersion(w, r)
	case r.URL.Path == "/api/usage":
		h.handleUsage(w, r)
	case r.URL.Path == "/fleet":
		h.handleFleetPage(w, r)
	case r.URL.Path == "/terms":
		h.handleTermsPage(w, r)
	case r.URL.Path == "/sitemap.xml":
		h.handleSitemap(w, r)
	case r.URL.Path == "/upgrade":
		h.handleUpgrade(w, r)
	case strings.HasPrefix(r.URL.Path, "/dashboard"):
		h.serveDashboard(w, r)
	case r.URL.Path == "/logo.png":
		h.serveFile(w, r, "/var/www/piportal/logo.png", "image/png")
	case r.URL.Path == "/install.sh":
		h.serveFile(w, r, "/var/www/piportal/install.sh", "text/plain")
	case strings.HasPrefix(r.URL.Path, "/downloads/"):
		h.serveDownload(w, r)
	default:
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}

func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, path, contentType string) {
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Write(data)
}

func (h *Handler) serveDownload(w http.ResponseWriter, r *http.Request) {
	// Only allow specific filenames to prevent path traversal
	filename := strings.TrimPrefix(r.URL.Path, "/downloads/")
	allowed := map[string]bool{
		"piportal-linux-arm64": true,
		"piportal-linux-arm":   true,
		"piportal-linux-amd64": true,
	}
	if !allowed[filename] {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	filepath := "/var/www/piportal/downloads/" + filename
	data, err := os.ReadFile(filepath)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Write(data)
}

func (h *Handler) handleHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	domain := h.config.BaseDomain
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>PiPortal — Remote Raspberry Pi Management &amp; Monitoring Dashboard</title>
    <meta name="description" content="Manage your Raspberry Pi fleet remotely with secure tunnels, live monitoring, and a unified dashboard. No inbound ports required.">
    <link rel="canonical" href="https://%[1]s/">
    <meta property="og:title" content="PiPortal — Remote Raspberry Pi Management &amp; Monitoring Dashboard">
    <meta property="og:description" content="Secure tunnels, live monitoring, and fleet management for Raspberry Pi and IoT devices.">
    <meta property="og:type" content="website">
    <meta property="og:url" content="https://%[1]s/">
    <script type="application/ld+json">
    {
      "@context": "https://schema.org",
      "@type": "SoftwareApplication",
      "name": "PiPortal",
      "applicationCategory": "DeveloperApplication",
      "operatingSystem": "Linux",
      "description": "Remote Raspberry Pi management and monitoring dashboard with secure tunnels.",
      "url": "https://%[1]s/",
      "offers": {
        "@type": "Offer",
        "price": "0",
        "priceCurrency": "USD"
      }
    }
    </script>
    <style>
        *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: system-ui, -apple-system, sans-serif; color: #1e293b; background: #fff; line-height: 1.6; }
        a { color: #0075ff; text-decoration: none; }
        .container { max-width: 1060px; margin: 0 auto; padding: 0 24px; }

        /* Nav */
        nav { padding: 16px 0; border-bottom: 1px solid #e2e8f0; }
        nav .container { display: flex; align-items: center; justify-content: space-between; }
        nav .logo { display: flex; align-items: center; gap: 10px; font-weight: 700; font-size: 1.25rem; color: #0f172a; }
        nav .logo img { height: 32px; width: auto; }
        nav .nav-cta { background: #0075ff; color: #fff; padding: 8px 20px; border-radius: 6px; font-size: 0.875rem; font-weight: 500; }
        nav .nav-cta:hover { background: #0060d0; }

        /* Hero */
        .hero { padding: 80px 0 64px; text-align: center; background: linear-gradient(180deg, #f0f7ff 0%%, #fff 100%%); }
        .hero-logo { margin-bottom: 24px; }
        .hero-logo img { height: 80px; width: auto; }
        .hero h1 { font-size: 2.75rem; font-weight: 800; line-height: 1.15; color: #0f172a; max-width: 720px; margin: 0 auto 20px; }
        .hero h1 span { color: #0075ff; }
        .hero p { font-size: 1.2rem; color: #475569; max-width: 560px; margin: 0 auto 32px; }
        .btn { display: inline-block; background: #0075ff; color: #fff; padding: 14px 32px; border-radius: 8px; font-size: 1rem; font-weight: 600; }
        .btn:hover { background: #0060d0; }

        /* Section shared */
        section { padding: 72px 0; }
        .section-label { text-align: center; font-size: 0.8rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.08em; color: #0075ff; margin-bottom: 8px; }
        .section-title { text-align: center; font-size: 2rem; font-weight: 700; color: #0f172a; margin-bottom: 16px; }
        .section-subtitle { text-align: center; font-size: 1.05rem; color: #64748b; max-width: 600px; margin: 0 auto 40px; }
        .alt-bg { background: #f8fafc; }

        /* Use cases */
        .usecases-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 24px; }
        .usecase-card { background: #fff; border: 1px solid #e2e8f0; border-radius: 12px; padding: 28px 24px; }
        .usecase-card h3 { font-size: 1.05rem; font-weight: 600; margin-bottom: 8px; }
        .usecase-card p { font-size: 0.9rem; color: #64748b; margin-bottom: 12px; }
        .usecase-card ul { list-style: none; }
        .usecase-card ul li { font-size: 0.85rem; color: #475569; padding: 3px 0 3px 18px; position: relative; }
        .usecase-card ul li::before { content: "\2022"; position: absolute; left: 0; color: #0075ff; }

        /* Stats banner */
        .stats-banner { display: flex; justify-content: center; flex-wrap: wrap; gap: 48px; padding: 20px 0; }
        .stat { text-align: center; }
        .stat .num { font-size: 2rem; font-weight: 800; color: #0075ff; }
        .stat .label { font-size: 0.85rem; color: #64748b; margin-top: 4px; }

        /* Mission */
        .mission { text-align: center; }
        .mission-inner { max-width: 700px; margin: 0 auto; }
        .mission-inner h2 { font-size: 1.75rem; font-weight: 700; color: #0f172a; margin-bottom: 16px; }
        .mission-inner p { font-size: 1.05rem; color: #475569; margin-bottom: 14px; }
        .mission-inner .sig { font-size: 0.95rem; color: #64748b; font-style: italic; margin-top: 20px; }

        /* CTA banner */
        .cta-banner { text-align: center; padding: 72px 0; background: linear-gradient(180deg, #f0f7ff 0%%, #fff 100%%); }
        .cta-banner h2 { font-size: 2rem; font-weight: 700; color: #0f172a; margin-bottom: 12px; }
        .cta-banner p { font-size: 1.05rem; color: #64748b; max-width: 480px; margin: 0 auto 28px; }

        /* FAQ */
        .faq-grid { max-width: 720px; margin: 0 auto; }
        .faq-item { border-bottom: 1px solid #e2e8f0; padding: 20px 0; }
        .faq-item:last-child { border-bottom: none; }
        .faq-item h3 { font-size: 1rem; font-weight: 600; margin-bottom: 8px; color: #0f172a; }
        .faq-item p { font-size: 0.9rem; color: #64748b; }

        /* Security */
        .security-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 20px; max-width: 880px; margin: 0 auto; }
        .sec-item { display: flex; align-items: flex-start; gap: 12px; background: #fff; border: 1px solid #e2e8f0; border-radius: 10px; padding: 20px; }
        .sec-icon { flex-shrink: 0; width: 36px; height: 36px; background: #eef6ff; border-radius: 8px; display: flex; align-items: center; justify-content: center; color: #0075ff; font-size: 1.1rem; }
        .sec-item h3 { font-size: 0.95rem; font-weight: 600; margin-bottom: 2px; }
        .sec-item p { font-size: 0.85rem; color: #64748b; }

        /* Features */
        .features-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 24px; }
        .feature-card { background: #fff; border: 1px solid #e2e8f0; border-radius: 12px; padding: 28px 24px; box-shadow: 0 1px 3px rgba(0,0,0,0.04); }
        .feature-card .icon { width: 44px; height: 44px; background: #eef6ff; border-radius: 10px; display: flex; align-items: center; justify-content: center; font-size: 1.3rem; margin-bottom: 16px; }
        .feature-card h3 { font-size: 1.05rem; font-weight: 600; margin-bottom: 8px; }
        .feature-card p { font-size: 0.9rem; color: #64748b; }
        .feature-card .detail { font-size: 0.8rem; color: #94a3b8; margin-top: 10px; font-style: italic; }

        /* How It Works */
        .steps { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 32px; max-width: 900px; margin: 0 auto; }
        .step { text-align: center; }
        .step-num { display: inline-flex; align-items: center; justify-content: center; width: 40px; height: 40px; border-radius: 50%%; background: #0075ff; color: #fff; font-weight: 700; font-size: 1.1rem; margin-bottom: 16px; }
        .step h3 { font-size: 1.1rem; font-weight: 600; margin-bottom: 8px; }
        .step p { font-size: 0.9rem; color: #64748b; }
        .step code { display: block; margin-top: 10px; background: #0f172a; color: #e2e8f0; padding: 10px 14px; border-radius: 8px; font-size: 0.82rem; text-align: left; white-space: pre; overflow-x: auto; font-family: ui-monospace, monospace; }

        /* Pricing */
        .pricing-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 24px; max-width: 680px; margin: 0 auto; }
        .price-card { border: 1px solid #e2e8f0; border-radius: 12px; padding: 32px 28px; background: #fff; }
        .price-card.featured { border-color: #0075ff; box-shadow: 0 0 0 1px #0075ff; }
        .price-card h3 { font-size: 1.2rem; font-weight: 700; margin-bottom: 4px; }
        .price-card .price { font-size: 2rem; font-weight: 800; color: #0f172a; margin: 12px 0 4px; }
        .price-card .price-sub { font-size: 0.85rem; color: #64748b; margin-bottom: 20px; }
        .price-card ul { list-style: none; margin-bottom: 24px; }
        .price-card ul li { font-size: 0.9rem; color: #334155; padding: 6px 0; padding-left: 22px; position: relative; }
        .price-card ul li::before { content: "\2713"; position: absolute; left: 0; color: #0075ff; font-weight: 700; }
        .btn-outline { display: inline-block; border: 1px solid #0075ff; color: #0075ff; padding: 12px 28px; border-radius: 8px; font-weight: 600; font-size: 0.95rem; }
        .btn-outline:hover { background: #f0f7ff; }

        /* Footer */
        footer { padding: 48px 0 32px; border-top: 1px solid #e2e8f0; color: #94a3b8; font-size: 0.85rem; }
        .footer-grid { display: grid; grid-template-columns: 2fr 1fr; gap: 32px; margin-bottom: 32px; }
        .footer-brand p { font-size: 0.85rem; color: #64748b; margin-top: 8px; max-width: 280px; }
        .footer-col h4 { font-size: 0.8rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.06em; color: #475569; margin-bottom: 12px; }
        .footer-col a { display: block; color: #64748b; padding: 4px 0; font-size: 0.85rem; }
        .footer-col a:hover { color: #0075ff; }
        .footer-bottom { text-align: center; padding-top: 24px; border-top: 1px solid #e2e8f0; }

        @media (max-width: 640px) {
            .hero h1 { font-size: 1.85rem; }
            .hero p { font-size: 1rem; }
            section { padding: 48px 0; }
            .section-title { font-size: 1.5rem; }
            .stats-banner { gap: 24px; }
            .stats-banner .stat .num { font-size: 1.5rem; }
            .footer-grid { grid-template-columns: 1fr; }
        }
    </style>
</head>
<body>

<nav>
  <div class="container">
    <a href="/" class="logo"><img src="/logo.png" alt="PiPortal">PiPortal</a>
    <a href="https://discord.gg/uuYtV5Ukk7" class="nav-cta" target="_blank" rel="noopener">Join the Beta</a>
  </div>
</nav>

<section class="hero">
  <div class="container">
    <h1>Manage Your <span>Raspberry Pi</span> Fleet from Anywhere</h1>
    <p>Secure HTTPS tunnels, real-time device monitoring, and remote management for every Pi you own. All from a single dashboard, with no static IP, port forwarding, or VPN.</p>
    <a href="https://discord.gg/uuYtV5Ukk7" class="btn" target="_blank" rel="noopener">Join the Beta</a>
    <div class="stats-banner">
      <div class="stat"><div class="num">30s</div><div class="label">Setup time</div></div>
      <div class="stat"><div class="num">0</div><div class="label">Inbound ports needed</div></div>
    </div>
  </div>
</section>

<section class="mission">
  <div class="container">
    <div class="mission-inner">
      <div class="section-label">Why PiPortal</div>
      <h2>Security Is Not an Afterthought</h2>
      <p>PiPortal was built by a security professional who got tired of watching Pi enthusiasts and businesses alike punch holes in their networks just to access a device remotely. Open ports, weak credentials, and unencrypted tunnels are the norm. Attackers know it.</p>
      <p>This project exists to give hobbyists and production teams the same level of security: zero inbound ports, encrypted connections by default, and access controls that actually work. Whether you run one Pi in your garage or a hundred in the field, your network should never be the weak link.</p>
      <p class="sig">Built for the community, by someone who has seen what happens when remote access goes wrong.</p>
    </div>
  </div>
</section>

<section class="alt-bg">
  <div class="container">
    <div class="section-label">Security</div>
    <h2 class="section-title">Defense in Depth</h2>
    <p class="section-subtitle">Every design decision in PiPortal starts with the same question: how do we keep your devices and your network safe?</p>
    <div class="security-grid">
      <div class="sec-item">
        <div class="sec-icon">&#128274;</div>
        <div><h3>End-to-End Encrypted</h3><p>All device connections use encrypted WebSockets over TLS. Traffic between your browser and your Pi is never exposed in plain text.</p></div>
      </div>
      <div class="sec-item">
        <div class="sec-icon">&#128737;</div>
        <div><h3>No Inbound Ports</h3><p>Your Pi only makes outbound connections to the PiPortal server. No listening ports, no firewall holes, no attack surface.</p></div>
      </div>
      <div class="sec-item">
        <div class="sec-icon">&#9881;</div>
        <div><h3>Tunnels Off by Default</h3><p>Tunnel forwarding is disabled until you explicitly enable it from the dashboard. Your local services are unreachable until you say so.</p></div>
      </div>
      <div class="sec-item">
        <div class="sec-icon">&#128273;</div>
        <div><h3>Token-Based Auth</h3><p>Devices authenticate with unique tokens. No passwords stored on disk. Revoke access instantly from the dashboard if a device is compromised.</p></div>
      </div>
      <div class="sec-item">
        <div class="sec-icon">&#128272;</div>
        <div><h3>HTTPS Everywhere</h3><p>Every tunnel gets its own HTTPS subdomain with a valid TLS certificate. Your traffic is encrypted end to end, automatically.</p></div>
      </div>
    </div>
  </div>
</section>

<section>
  <div class="container">
    <div class="section-label">Features</div>
    <h2 class="section-title">Everything You Need</h2>
    <p class="section-subtitle">Remote access, monitoring, and management without the complexity of VPNs or the risk of open ports.</p>
    <div class="features-grid">
      <div class="feature-card">
        <div class="icon">&#128268;</div>
        <h3>Secure Tunnels</h3>
        <p>Expose any local port on your Pi via a unique HTTPS subdomain. Share web apps, APIs, or dev servers with a link. No port forwarding required.</p>
        <p class="detail">e.g. mypi.%[1]s routes to localhost:8080 on your device</p>
      </div>
      <div class="feature-card">
        <div class="icon">&#128200;</div>
        <h3>Live Monitoring</h3>
        <p>CPU temperature, memory usage, disk space, and uptime stream to your dashboard in real time over WebSocket. Spot problems before they become outages.</p>
        <p class="detail">Updates every few seconds with no polling or page refresh</p>
      </div>
      <div class="feature-card">
        <div class="icon">&#128260;</div>
        <h3>Remote Reboot</h3>
        <p>Restart any device from the dashboard with one click. No SSH session, no physical access needed. Ideal for headless Pis in hard-to-reach locations.</p>
        <p class="detail">Device reconnects automatically after reboot</p>
      </div>
      <div class="feature-card">
        <div class="icon">&#128421;</div>
        <h3>Fleet Dashboard</h3>
        <p>See every Pi at a glance: online/offline status, resource usage, and active tunnels. Manage your entire fleet from a single browser tab.</p>
        <p class="detail">Works on desktop and mobile browsers</p>
      </div>
    </div>
  </div>
</section>

<section class="alt-bg">
  <div class="container">
    <div class="section-label">Use Cases</div>
    <h2 class="section-title">Built for Real Projects</h2>
    <p class="section-subtitle">Whether you have one Pi or a hundred, PiPortal keeps you connected.</p>
    <div class="usecases-grid">
      <div class="usecase-card">
        <h3>Home Lab &amp; Self-Hosting</h3>
        <p>Run web apps, media servers, or home automation on a Pi at home and access them from anywhere over HTTPS.</p>
        <ul>
          <li>Access Home Assistant remotely</li>
          <li>Share a Nextcloud instance</li>
          <li>Expose a Pi-hole dashboard</li>
        </ul>
      </div>
      <div class="usecase-card">
        <h3>IoT &amp; Edge Deployments</h3>
        <p>Deploy Pis to remote sites like warehouses, farms, and retail locations, then manage them all centrally without VPNs.</p>
        <ul>
          <li>Monitor environmental sensors</li>
          <li>Update kiosk software remotely</li>
          <li>Reboot unresponsive devices</li>
        </ul>
      </div>
      <div class="usecase-card">
        <h3>Development &amp; Prototyping</h3>
        <p>Share a work-in-progress running on your Pi with teammates or clients using a real HTTPS URL.</p>
        <ul>
          <li>Demo APIs and dashboards</li>
          <li>Test webhooks from external services</li>
          <li>Pair-program on embedded projects</li>
        </ul>
      </div>
    </div>
  </div>
</section>

<section>
  <div class="container">
    <div class="section-label">Get Started</div>
    <h2 class="section-title">Up and Running in Minutes</h2>
    <p class="section-subtitle">Three commands. No dependencies, no Docker, no config files.</p>
    <div class="steps">
      <div class="step">
        <div class="step-num">1</div>
        <h3>Install</h3>
        <p>One command to install the PiPortal agent on your Pi.</p>
        <code>curl -fsSL https://%[1]s/install.sh | bash</code>
      </div>
      <div class="step">
        <div class="step-num">2</div>
        <h3>Connect</h3>
        <p>Start the agent and point it at the port you want to expose.</p>
        <code>piportal setup
piportal start --port 8080</code>
      </div>
      <div class="step">
        <div class="step-num">3</div>
        <h3>Manage</h3>
        <p>Open your dashboard to monitor, control, and tunnel into your devices.</p>
        <code>https://%[1]s/dashboard</code>
      </div>
    </div>
  </div>
</section>

<section class="alt-bg">
  <div class="container">
    <div class="section-label">Pricing</div>
    <h2 class="section-title">Simple, Per-Device Pricing</h2>
    <p class="section-subtitle">Start free with one device. Add more for a flat per-device rate with no surprises.</p>
    <div class="pricing-grid">
      <div class="price-card featured">
        <h3>Free</h3>
        <div class="price">$0</div>
        <div class="price-sub">forever</div>
        <ul>
          <li>1 device</li>
          <li>1 GB tunnel traffic / month</li>
          <li>Live monitoring</li>
          <li>Remote reboot</li>
          <li>HTTPS tunnel subdomain</li>
        </ul>
        <a href="https://discord.gg/uuYtV5Ukk7" class="btn" style="width:100%%;text-align:center;" target="_blank" rel="noopener">Join the Beta</a>
      </div>
      <div class="price-card">
        <h3>Pro</h3>
        <div class="price">$2.50<span style="font-size:1rem;font-weight:400;color:#64748b"> / device / mo</span></div>
        <div class="price-sub">coming soon</div>
        <ul>
          <li>Unlimited devices</li>
          <li>100 GB tunnel traffic / month</li>
          <li>Live monitoring</li>
          <li>Remote reboot</li>
          <li>Priority support</li>
        </ul>
        <span class="btn-outline" style="width:100%%;text-align:center;display:inline-block;cursor:default;opacity:0.6;">Coming Soon</span>
      </div>
    </div>
  </div>
</section>

<section>
  <div class="container">
    <div class="section-label">FAQ</div>
    <h2 class="section-title">Frequently Asked Questions</h2>
    <div class="faq-grid">
      <div class="faq-item">
        <h3>What devices does PiPortal support?</h3>
        <p>PiPortal works on any Raspberry Pi running a 32-bit or 64-bit Linux OS (Raspberry Pi OS, Ubuntu, DietPi, etc.). It also works on other ARM or x86 Linux single-board computers.</p>
      </div>
      <div class="faq-item">
        <h3>Do I need to open ports on my router?</h3>
        <p>No. The PiPortal agent makes outbound-only connections to the server. You never need to configure port forwarding, dynamic DNS, or firewall rules.</p>
      </div>
      <div class="faq-item">
        <h3>Is my traffic secure?</h3>
        <p>Yes. All connections are encrypted with TLS. Tunnel traffic is carried over secure WebSockets, and every tunnel subdomain gets a valid HTTPS certificate automatically.</p>
      </div>
      <div class="faq-item">
        <h3>What happens if my Pi goes offline?</h3>
        <p>The dashboard shows real-time connection status. When your Pi reconnects, the agent automatically re-establishes its connection to the server. No manual intervention needed.</p>
      </div>
      <div class="faq-item">
        <h3>Can I use PiPortal for production services?</h3>
        <p>PiPortal is great for internal tools, prototypes, and home labs. For larger deployments, the Pro plan gives you unlimited devices with 100 GB of tunnel traffic each at a flat per-device rate.</p>
      </div>
      <div class="faq-item">
        <h3>How is the free plan limited?</h3>
        <p>The free plan includes one device with 1 GB of tunnel traffic per month. Monitoring, remote terminal, and remote reboot are fully included. To add more devices, upgrade to Pro at $2.50/device/month.</p>
      </div>
    </div>
  </div>
</section>

<section class="cta-banner">
  <div class="container">
    <h2>Ready to manage your Pis securely?</h2>
    <p>PiPortal is in beta. Join the Discord to get access, share feedback, and help shape the project.</p>
    <a href="https://discord.gg/uuYtV5Ukk7" class="btn" target="_blank" rel="noopener">Join the Beta on Discord</a>
  </div>
</section>

<footer>
  <div class="container">
    <div class="footer-grid">
      <div class="footer-brand">
        <div style="display:flex;align-items:center;gap:8px;font-weight:700;font-size:1.1rem;color:#0f172a;"><img src="/logo.png" alt="PiPortal" style="height:24px;width:auto;">PiPortal</div>
        <p>Secure remote access and monitoring for Raspberry Pi and IoT devices. Built by a security professional, for the community.</p>
      </div>
      <div class="footer-col">
        <h4>Product</h4>
        <a href="https://discord.gg/uuYtV5Ukk7" target="_blank" rel="noopener">Join the Beta</a>
        <a href="/status">Service Status</a>
      </div>
    </div>
    <div class="footer-bottom">
      <p>&copy; 2025 PiPortal. All rights reserved.</p>
    </div>
  </div>
</footer>

</body>
</html>`, domain)
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Subdomain string `json:"subdomain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Subdomain == "" {
		jsonError(w, "subdomain is required", http.StatusBadRequest)
		return
	}

	device, err := h.store.CreateDevice(req.Subdomain, "")
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"token":     device.Token,
		"subdomain": device.Subdomain,
		"url":       fmt.Sprintf("https://%s.%s", device.Subdomain, h.config.BaseDomain),
	})
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"tunnels": h.tunnels.Stats(),
	})
}

func (h *Handler) handleStatusPage(w http.ResponseWriter, r *http.Request) {
	stats := h.tunnels.Stats()
	activeTunnels := stats["active_tunnels"]
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>PiPortal — Service Status</title>
    <style>
        *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: system-ui, -apple-system, sans-serif; color: #1e293b; background: #f8fafc; line-height: 1.6; }
        a { color: #0075ff; text-decoration: none; }
        nav { padding: 16px 0; border-bottom: 1px solid #e2e8f0; background: #fff; }
        nav .inner { max-width: 720px; margin: 0 auto; padding: 0 24px; display: flex; align-items: center; justify-content: space-between; }
        nav .logo { font-weight: 700; font-size: 1.25rem; color: #0f172a; }
        .container { max-width: 720px; margin: 0 auto; padding: 48px 24px; }
        .status-header { text-align: center; margin-bottom: 40px; }
        .status-header h1 { font-size: 1.75rem; font-weight: 700; margin-bottom: 12px; }
        .status-badge { display: inline-flex; align-items: center; gap: 8px; background: #f0fdf4; border: 1px solid #bbf7d0; padding: 8px 20px; border-radius: 20px; font-size: 0.95rem; font-weight: 600; color: #166534; }
        .status-badge .dot { width: 10px; height: 10px; border-radius: 50%%; background: #22c55e; }
        .cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 16px; margin-bottom: 32px; }
        .card { background: #fff; border: 1px solid #e2e8f0; border-radius: 10px; padding: 24px; text-align: center; }
        .card .value { font-size: 2rem; font-weight: 800; color: #0f172a; }
        .card .label { font-size: 0.85rem; color: #64748b; margin-top: 4px; }
        .info { background: #fff; border: 1px solid #e2e8f0; border-radius: 10px; padding: 24px; }
        .info h2 { font-size: 1.1rem; font-weight: 600; margin-bottom: 16px; }
        .info-row { display: flex; justify-content: space-between; padding: 10px 0; border-bottom: 1px solid #f1f5f9; }
        .info-row:last-child { border-bottom: none; }
        .info-row .key { color: #64748b; font-size: 0.9rem; }
        .info-row .val { font-weight: 600; font-size: 0.9rem; }
        .info-row .val.ok { color: #16a34a; }
        footer { text-align: center; padding: 32px 24px; color: #94a3b8; font-size: 0.85rem; }
    </style>
</head>
<body>
<nav><div class="inner"><a href="/" class="logo">PiPortal</a><a href="/dashboard">Dashboard</a></div></nav>
<div class="container">
  <div class="status-header">
    <h1>Service Status</h1>
    <div class="status-badge"><div class="dot"></div> All Systems Operational</div>
  </div>
  <div class="cards">
    <div class="card"><div class="value">%d</div><div class="label">Active Tunnels</div></div>
    <div class="card"><div class="value">v%s</div><div class="label">Agent Version</div></div>
    <div class="card"><div class="value">%s</div><div class="label">Region</div></div>
  </div>
  <div class="info">
    <h2>Services</h2>
    <div class="info-row"><span class="key">Tunnel Server</span><span class="val ok">Operational</span></div>
    <div class="info-row"><span class="key">Dashboard</span><span class="val ok">Operational</span></div>
    <div class="info-row"><span class="key">Device API</span><span class="val ok">Operational</span></div>
    <div class="info-row"><span class="key">WebSocket Connections</span><span class="val ok">Operational</span></div>
    <div class="info-row"><span class="key">TLS Certificates</span><span class="val ok">Operational</span></div>
  </div>
</div>
<footer><a href="/">PiPortal</a> &middot; <a href="/dashboard">Dashboard</a></footer>
</body>
</html>`, activeTunnels, ClientVersion, h.config.BaseDomain)
}

// ClientVersion is the latest client version available for download
var ClientVersion = "0.1.3"
var ClientChangelog = "Added browser-based terminal (remote shell) support"

func (h *Handler) handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"version":      ClientVersion,
		"release_date": "2026-02-04",
		"changelog":    ClientChangelog,
	})
}

func (h *Handler) handleUsage(w http.ResponseWriter, r *http.Request) {
	// Requires token auth
	token := r.Header.Get("Authorization")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		jsonError(w, "Authorization required", http.StatusUnauthorized)
		return
	}

	// Strip "Bearer " prefix if present
	token = strings.TrimPrefix(token, "Bearer ")

	device, err := h.store.GetDeviceByToken(token)
	if err != nil {
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if device == nil {
		jsonError(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	usage, err := h.store.GetMonthlyUsage(device.ID)
	if err != nil {
		jsonError(w, "Failed to get usage", http.StatusInternalServerError)
		return
	}

	limit, err := h.store.GetBandwidthLimit(device.ID)
	if err != nil {
		jsonError(w, "Failed to get limit", http.StatusInternalServerError)
		return
	}

	totalUsed := usage.BytesIn + usage.BytesOut

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"subdomain":    device.Subdomain,
		"tier":         device.Tier,
		"month":        usage.Month,
		"bytes_in":     usage.BytesIn,
		"bytes_out":    usage.BytesOut,
		"bytes_total":  totalUsed,
		"limit":        limit,
		"limit_human":  FormatBytes(limit),
		"used_human":   FormatBytes(totalUsed),
		"percent_used": float64(totalUsed) / float64(limit) * 100,
	})
}

func (h *Handler) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Upgrade to PiPortal Pro</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; }
        h1 { color: #333; }
        .plan { border: 2px solid #ddd; border-radius: 10px; padding: 20px; margin: 20px 0; }
        .plan.pro { border-color: #4CAF50; }
        .plan h2 { margin-top: 0; }
        .price { font-size: 2em; color: #4CAF50; }
        .features { list-style: none; padding: 0; }
        .features li { padding: 8px 0; }
        .features li::before { content: "✓ "; color: #4CAF50; }
        .btn { display: inline-block; background: #4CAF50; color: white; padding: 12px 24px;
               text-decoration: none; border-radius: 5px; font-size: 1.1em; }
        .btn:hover { background: #45a049; }
        .current { background: #f0f0f0; padding: 5px 10px; border-radius: 3px; font-size: 0.9em; }
    </style>
</head>
<body>
    <h1>PiPortal Pro</h1>

    <div class="plan">
        <h2>Free Tier <span class="current">Current</span></h2>
        <ul class="features">
            <li>1 GB bandwidth per month</li>
            <li>1 tunnel per device</li>
            <li>Community support</li>
        </ul>
    </div>

    <div class="plan pro">
        <h2>Pro</h2>
        <p class="price">$3/month</p>
        <ul class="features">
            <li>100 GB bandwidth per month</li>
            <li>Priority routing</li>
            <li>Email support</li>
            <li>Support indie development</li>
        </ul>
        <p><em>Coming soon! Email <a href="mailto:hello@piportal.dev">hello@piportal.dev</a> to get notified.</em></p>
    </div>

    <p style="color: #666; font-size: 0.9em;">
        Bandwidth resets on the 1st of each month. Check your usage with <code>piportal status</code>.
    </p>
</body>
</html>`)
}

// --- Helpers ---

func sendJSON(conn *websocket.Conn, msg interface{}) error {
	data, _ := json.Marshal(msg)
	return conn.WriteMessage(websocket.TextMessage, data)
}

func sendError(conn *websocket.Conn, code, message string) {
	sendJSON(conn, NewErrorMessage(code, message))
}

func jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

func generateRequestID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "req_" + hex.EncodeToString(b)
}
