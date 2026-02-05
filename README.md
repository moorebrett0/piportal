# PiPortal

Expose any Raspberry Pi to the internet with a single command. PiPortal gives each device a public HTTPS subdomain (`yourname.piportal.dev`) via a WebSocket tunnel — no port forwarding, no dynamic DNS.

## How It Works

```
Pi (localhost:8080) ──WebSocket──▸ PiPortal Server ◂── https://yourname.piportal.dev
```

A lightweight Go client on the Pi opens a persistent WebSocket connection to the server. The server holds a wildcard TLS cert for `*.piportal.dev` and routes incoming HTTPS requests through the tunnel to the Pi's local port.

## Project Structure

```
piportal-server/     Go server — API, tunnels, embedded dashboard
piportal-dashboard/  React + TypeScript frontend (Vite)
piportal-client/     Go CLI installed on each Pi
deploy/              Deployment scripts and docs
```

## Prerequisites

- **Go 1.21+**
- **Node.js 20+** and npm

## Local Development

### 1. Dashboard

```bash
cd piportal-dashboard
npm install
npm run dev
```

Runs on `http://localhost:5173` with hot reload.

### 2. Server

```bash
cd piportal-server

# Copy in the dashboard build (or skip if running dashboard separately)
cd ../piportal-dashboard && npm run build && cp -r dist ../piportal-server/dashboard/dist && cd ../piportal-server

# Run in dev mode
go run . -dev -http :8080
```

Dev mode uses a built-in JWT secret and skips TLS. The server listens on `:8080` and serves:
- `/dashboard/*` — React SPA
- `/api/v1/*` — Dashboard REST API
- `/tunnel` — WebSocket tunnel endpoint
- `*.piportal.dev` — Proxies to connected Pi devices

### 3. Client (on a Pi or any Linux machine)

```bash
cd piportal-client
go run . setup   # Interactive — registers device, saves config
go run . start   # Connects tunnel, forwards to local port
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PIPORTAL_JWT_SECRET` | JWT signing secret (required in production) | Dev secret in `-dev` mode |
| `PIPORTAL_DOMAIN` | Base domain for tunnels | `piportal.dev` |
| `PIPORTAL_DB` | Path to SQLite database file | `piportal.db` |
| `PIPORTAL_DEV` | Set to `1` for dev mode | — |

## Deploying

See [`deploy/deploy.md`](deploy/deploy.md) for full deployment docs. Quick version:

```bash
# Build dashboard
cd piportal-dashboard
npm run build
cp -r dist ../piportal-server/dashboard/dist

# Deploy everything
cd deploy
./deploy.sh
```

This cross-compiles client binaries (arm64/arm/amd64), builds the server (with the dashboard embedded), uploads to the production server, and restarts the service.

## Architecture

| Component | Stack |
|-----------|-------|
| Server | Go, SQLite, WebSocket, embedded SPA |
| Dashboard | React 19, TypeScript, Vite, React Router |
| Client | Go, Cobra CLI, WebSocket |
| Infra | DigitalOcean, Caddy (reverse proxy + wildcard TLS), systemd |

### Key Server Files

| File | Purpose |
|------|---------|
| `main.go` | Entry point |
| `handler.go` | HTTP routing and subdomain proxy |
| `dashboard.go` | Dashboard API handlers + SPA serving |
| `store.go` | SQLite data layer (users, devices, orgs, bandwidth) |
| `tunnel.go` | Tunnel connection management |
| `fleet.go` | Device fleet / metrics |
| `auth.go` | JWT + password hashing |
| `terminal.go` | WebSocket terminal relay |

## Features

- **HTTPS tunnels** — Each device gets a public `*.piportal.dev` subdomain
- **Web dashboard** — Manage devices, view system metrics, monitor bandwidth
- **In-browser terminal** — SSH-like shell access via WebSocket
- **Tunnel forwarding toggle** — Enable/disable HTTP forwarding per device
- **Bandwidth tracking** — Monthly usage with free (1 GB) and pro (100 GB) tiers
- **Device tagging** — Group devices with custom tags
- **Remote reboot** — Send reboot commands from the dashboard
- **Group command execution** — Run shell commands across all devices in a tag group
- **Self-updating client** — `piportal upgrade` pulls the latest binary
- **One-line install** — `curl -fsSL https://piportal.dev/install.sh | bash`

## Community

Have a question, found a bug, or want to contribute? Join the Discord:

**[discord.gg/uuYtV5Ukk7](https://discord.gg/uuYtV5Ukk7)**

## License

PiPortal is licensed under the [GNU Affero General Public License v3.0](LICENSE).
