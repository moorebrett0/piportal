# PiTunnel — Design Document

A lightweight tunneling service for Raspberry Pi and hobbyist IoT devices.

## Overview

PiTunnel consists of two components:

1. **Client** — A lightweight daemon that runs on the Pi, maintains a persistent connection to the server, and forwards traffic to local services
2. **Server** — A public-facing service that accepts tunnel connections from clients and routes incoming HTTP requests to the appropriate tunnel

```
┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
│   Browser       │ ───────▶│   PiTunnel      │ ───────▶│   Raspberry Pi  │
│                 │  HTTPS  │   Server        │  Tunnel │   (Client)      │
│                 │         │                 │         │      │          │
│                 │◀─────── │   *.pitunnel.io │◀─────── │      ▼          │
│                 │         │                 │         │   Local Service │
└─────────────────┘         └─────────────────┘         │   (port 8080)   │
                                                        └─────────────────┘
```

## Client Application

### Responsibilities

- Establish and maintain a persistent WebSocket connection to the server
- Authenticate using a device token
- Receive incoming HTTP requests from the server
- Forward requests to the configured local port
- Send responses back through the tunnel
- Handle reconnection with exponential backoff
- Run as a systemd service for reliability

### Tech Stack

- **Language:** Go
- **Dependencies:** gorilla/websocket, cobra (CLI framework)

### Configuration

```yaml
# /etc/pitunnel/config.yaml
server: wss://tunnel.pitunnel.io
token: pt_abc123def456
local_port: 8080
local_host: 127.0.0.1  # optional, defaults to localhost
```

Or via command line:
```bash
pitunnel --token pt_abc123def456 --port 8080
```

### Binary Size Target

- < 5MB for ARM64 (Pi 4, Pi 5)
- < 5MB for ARMv6 (Pi Zero W)
- Static linking, no runtime dependencies

### Memory Target

- < 10MB RSS during normal operation
- Handle up to 10 concurrent requests without significant growth

### Client State Machine

```
┌──────────┐
│          │
│  INIT    │────────────────────────────────────┐
│          │                                    │
└────┬─────┘                                    │
     │                                          │
     ▼                                          │
┌──────────┐     connect failed          ┌──────────┐
│          │ ─────────────────────────▶  │          │
│CONNECTING│                             │ BACKOFF  │
│          │ ◀─────────────────────────  │          │
└────┬─────┘     wait complete           └──────────┘
     │                                          ▲
     │ connected                                │
     ▼                                          │
┌──────────┐     connection lost                │
│          │ ───────────────────────────────────┘
│CONNECTED │
│          │
└──────────┘
```

### Reconnection Strategy

- Initial delay: 1 second
- Max delay: 60 seconds
- Multiplier: 2x
- Jitter: ±20%
- Reset delay to 1s after 5 minutes of stable connection

---

## Server Application

### Responsibilities

- Accept WebSocket connections from clients
- Validate device tokens
- Assign and manage subdomains
- Accept incoming HTTP requests on wildcard domain
- Route requests to the correct tunnel based on subdomain
- Multiplex multiple HTTP requests per tunnel connection
- Handle TLS termination
- Track connected devices and their status

### Tech Stack

- **Language:** Go (net/http + gorilla/websocket)
- **Database:** SQLite
- **Reverse Proxy:** Caddy (TLS termination, wildcard certs via Cloudflare DNS-01)
- **Dashboard:** React SPA (TypeScript + Vite), embedded in server binary via `go:embed`

### Architecture

```
                                    ┌─────────────────────────────────┐
                                    │         PiTunnel Server         │
                                    │                                 │
    HTTPS requests                  │  ┌───────────────────────────┐  │
    *.pitunnel.io                   │  │    HTTP Router            │  │
────────────────────────────────────┼─▶│                           │  │
                                    │  │  - Extract subdomain      │  │
                                    │  │  - Look up tunnel         │  │
                                    │  │  - Forward request        │  │
                                    │  └───────────┬───────────────┘  │
                                    │              │                  │
                                    │              ▼                  │
                                    │  ┌───────────────────────────┐  │
                                    │  │    Tunnel Manager         │  │
                                    │  │                           │  │
    WebSocket connections           │  │  - Track active tunnels   │  │
    (from Pi clients)               │  │  - Request/response       │  │
────────────────────────────────────┼─▶│    multiplexing           │  │
                                    │  │  - Health checks          │  │
                                    │  └───────────────────────────┘  │
                                    │                                 │
                                    │  ┌───────────────────────────┐  │
                                    │  │    Token Store            │  │
                                    │  │                           │  │
                                    │  │  - Validate tokens        │  │
                                    │  │  - Map token → subdomain  │  │
                                    │  └───────────────────────────┘  │
                                    │                                 │
                                    └─────────────────────────────────┘
```

### Database Schema (MVP)

```sql
-- Devices registered with the service
CREATE TABLE devices (
    id TEXT PRIMARY KEY,           -- uuid
    token TEXT UNIQUE NOT NULL,    -- pt_xxxx (hashed)
    subdomain TEXT UNIQUE NOT NULL,-- mypi
    created_at TIMESTAMP,
    last_seen_at TIMESTAMP,
    is_online BOOLEAN DEFAULT FALSE
);

-- Optional: request logs for debugging
CREATE TABLE request_logs (
    id TEXT PRIMARY KEY,
    device_id TEXT REFERENCES devices(id),
    method TEXT,
    path TEXT,
    status_code INTEGER,
    duration_ms INTEGER,
    created_at TIMESTAMP
);
```

### Subdomain Assignment

For MVP, let users pick their subdomain at registration. Validate:
- 3-30 characters
- Lowercase alphanumeric and hyphens only
- No leading/trailing hyphens
- Not on reserved list (www, api, app, admin, etc.)

### TLS

Use Let's Encrypt with wildcard certificate for `*.pitunnel.io`. Renew automatically with DNS-01 challenge.

---

## Wire Protocol

Communication between client and server happens over a WebSocket connection. Messages are JSON for simplicity in MVP (can optimize to binary later if needed).

### Message Types

#### Client → Server

```json
// Authentication (sent immediately after connect)
{
    "type": "auth",
    "token": "pt_abc123def456",
    "client_version": "0.1.2"
}

// Response to a proxied request
{
    "type": "response",
    "request_id": "req_123",
    "status_code": 200,
    "headers": {
        "Content-Type": "text/html"
    },
    "body_base64": "PGh0bWw+Li4uPC9odG1sPg=="
}

// System metrics (sent every 30 seconds alongside ping)
{
    "type": "metrics",
    "cpu_temp": 42.5,
    "mem_total": 4294967296,
    "mem_free": 2147483648,
    "disk_total": 32212254720,
    "disk_free": 16106127360,
    "uptime": 86400,
    "load_avg": 0.15
}

// Heartbeat/ping
{
    "type": "ping"
}
```

#### Server → Client

```json
// Authentication result
{
    "type": "auth_result",
    "success": true,
    "subdomain": "mypi",
    "message": "Connected as mypi.piportal.dev"
}

// Incoming HTTP request to forward
{
    "type": "request",
    "request_id": "req_123",
    "method": "GET",
    "path": "/api/sensors",
    "headers": {
        "Accept": "application/json",
        "X-Forwarded-For": "203.0.113.50"
    },
    "body_base64": null
}

// Remote command (e.g., reboot)
{
    "type": "command",
    "command_id": "cmd_1706918400000000000",
    "command": "reboot"
}

// Heartbeat response
{
    "type": "pong"
}

// Error/disconnect
{
    "type": "error",
    "code": "invalid_token",
    "message": "Token not recognized"
}
```

### Request Flow

```
Browser              Server                    Client                  Local App
   │                    │                         │                        │
   │  GET /api/temp     │                         │                        │
   │───────────────────▶│                         │                        │
   │                    │                         │                        │
   │                    │  {"type":"request"...}  │                        │
   │                    │────────────────────────▶│                        │
   │                    │                         │                        │
   │                    │                         │  GET /api/temp         │
   │                    │                         │───────────────────────▶│
   │                    │                         │                        │
   │                    │                         │  {"temp": 22.5}        │
   │                    │                         │◀───────────────────────│
   │                    │                         │                        │
   │                    │  {"type":"response"...} │                        │
   │                    │◀────────────────────────│                        │
   │                    │                         │                        │
   │  {"temp": 22.5}    │                         │                        │
   │◀───────────────────│                         │                        │
   │                    │                         │                        │
```

### Timeouts

- Request timeout: 30 seconds (configurable)
- WebSocket ping interval: 30 seconds
- Connection considered dead after 3 missed pings

---

## MVP Feature Scope

### Included

- [x] Single HTTP port forwarding per device
- [x] Auto-assigned subdomain (user picks at registration)
- [x] Token-based authentication
- [x] Auto-reconnection on client
- [x] Basic request logging
- [x] ARM + x86 client builds
- [x] systemd service file
- [x] Simple web UI for token generation
- [x] Dashboard with device management (React SPA)
- [x] User accounts (signup/login with JWT)
- [x] Bandwidth tracking and monthly limits (1GB free tier)
- [x] Client self-upgrade (`piportal upgrade`)
- [x] Device metrics reporting (CPU temp, memory, disk, uptime, load avg)
- [x] Remote device reboot from dashboard
- [x] Device claim flow (register device, claim via token in dashboard)

### Not Included (Future)

- [ ] Metrics history graphs (CPU temp, memory, disk over time)
- [ ] Multiple tunnels per device
- [ ] TCP tunneling (SSH, databases)
- [ ] Custom domains
- [ ] Team/organization accounts
- [ ] Pro tier billing ($3/month for 100GB)
- [ ] Geographic edge servers
- [ ] Request inspection/replay UI
- [ ] Webhook notifications (device online/offline)

---

## Deployment

### Server Requirements (MVP)

- Single VPS (4GB RAM, 2 vCPU is plenty to start)
- Domain with wildcard DNS pointing to server
- Wildcard TLS certificate
- SQLite database (single file, easy backups)

### Suggested Providers

- Hetzner (cheap, reliable): ~$5/month for CX21
- DigitalOcean: $6/month droplet
- Vultr: Similar pricing

### DNS Setup

```
pitunnel.io         A       203.0.113.10
*.pitunnel.io       A       203.0.113.10
```

---

## Development Phases

### Phase 1: Core Tunneling — DONE

1. ~~WebSocket server + client in Go~~
2. ~~Token-based auth, subdomain routing~~
3. ~~HTTP request forwarding end-to-end~~
4. ~~Auto-reconnection with exponential backoff~~
5. ~~TLS via Caddy + Let's Encrypt wildcard~~
6. ~~systemd service + install script~~

### Phase 2: Dashboard & Accounts — DONE

1. ~~React SPA dashboard (embedded in server binary)~~
2. ~~User signup/login with JWT auth~~
3. ~~Device CRUD (create, claim, delete)~~
4. ~~Bandwidth tracking with monthly limits (1GB free)~~
5. ~~Client self-upgrade command~~

### Phase 3: Device Management — DONE (v0.1.2)

1. ~~Real-time device metrics (CPU temp, memory, disk, uptime, load avg)~~
2. ~~Metrics displayed on dashboard (detail page + device cards)~~
3. ~~Remote reboot from dashboard~~

### Phase 4: Next Up

1. Metrics history graphs (time-series charts for CPU temp, memory, disk)
2. Pro tier billing ($3/month for 100GB)
3. Alerting / notifications (device offline, high temp, disk full)
4. Custom domains
5. TCP tunneling (SSH, databases)

---

## File Structure

### Actual Project Structure

```
PiPortal/
├── piportal-client/           # Go client (runs on Pi devices)
│   ├── main.go
│   └── cmd/
│       ├── root.go            # CLI root, version string
│       ├── start.go           # `piportal start` command
│       ├── setup.go           # `piportal setup` interactive config
│       ├── tunnel.go          # WebSocket connection management
│       ├── proxy.go           # Local HTTP forwarding
│       ├── protocol.go        # Message types (auth, request, response, metrics, command)
│       ├── metrics.go         # System metrics collection (/proc, /sys)
│       ├── upgrade.go         # `piportal upgrade` self-update
│       └── config.go          # Config file parsing
│
├── piportal-server/           # Go server (runs on DigitalOcean)
│   ├── main.go
│   ├── handler.go             # HTTP routing, tunnel proxy, main site
│   ├── dashboard.go           # Dashboard API routes, embedded SPA serving
│   ├── tunnel.go              # Tunnel manager, WebSocket tunnel handling
│   ├── protocol.go            # Wire protocol message types
│   ├── auth.go                # JWT, bcrypt, auth middleware
│   ├── store.go               # SQLite database operations
│   ├── bandwidth.go           # Bandwidth tracking and limits
│   └── dashboard/dist/        # Embedded React dashboard build
│
├── piportal-dashboard/        # React SPA (TypeScript + Vite)
│   ├── src/
│   │   ├── api.ts             # API client and types
│   │   ├── App.tsx            # Router and layout
│   │   ├── index.css          # Global styles (dark theme)
│   │   ├── pages/
│   │   │   ├── DeviceDetailPage.tsx   # Device detail with metrics + reboot
│   │   │   ├── DevicesPage.tsx        # Device list
│   │   │   ├── AddDevicePage.tsx      # Create/claim device
│   │   │   ├── LoginPage.tsx
│   │   │   └── SignupPage.tsx
│   │   └── components/
│   │       ├── DeviceCard.tsx         # Device card with compact metrics
│   │       ├── StatusBadge.tsx        # Online/offline badge
│   │       └── BandwidthBar.tsx       # Usage bar
│   └── dist/                  # Build output → copied to server
│
└── deploy/
    ├── deploy.sh              # Full deploy (server + client binaries)
    ├── deploy.md              # Deployment documentation
    └── setup-server.sh        # One-time server provisioning
```

---

## Decisions Made

1. **Language:** Go for both client and server. Binary sizes ~8MB, acceptable tradeoff for development speed.
2. **Protocol:** JSON over WebSocket. Good enough for the traffic levels we see.
3. **Free tier:** 1GB bandwidth/month. Pro tier planned at $3/month for 100GB.
4. **Name:** PiPortal (`piportal.dev`).
5. **Metrics storage:** In-memory only (on the Tunnel struct). Ephemeral — only available while device is connected. No database changes needed.

## Open Questions

1. **Metrics history:** Need to decide on storage strategy for time-series data (SQLite? Separate table? Retention policy?).
2. **Binary protocol later?** JSON is fine for now but could optimize if high-throughput tunnels become common.
3. **Billing integration:** Stripe? LemonSqueezy? Need to evaluate before Pro tier launch.

---

## Resources

- [ngrok architecture blog post](https://inconshreveable.com/software/)
- [frp](https://github.com/fatedier/frp) — open source alternative, good reference
- [bore](https://github.com/ekzhang/bore) — minimal tunnel in Rust, excellent code reference
- [rathole](https://github.com/rapiz1/rathole) — another lightweight Rust option
