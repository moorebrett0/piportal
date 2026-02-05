# PiPortal

Self-hosted remote access for Raspberry Pi. Manage all your Pis from one dashboard — see what's online, open a terminal, run commands, and fix stuff from your browser. No VPN, no port forwarding.

## How It Works

```
Pi (localhost:8080) ──WebSocket──▸ Your PiPortal Server ◂── https://mypi.yourdomain.com
```

A lightweight Go client on the Pi opens a persistent outbound WebSocket connection to your server. The server holds a wildcard TLS cert for `*.yourdomain.com` and routes incoming HTTPS requests through the tunnel to the Pi's local port.

**You bring your own domain and server.** PiPortal is fully self-hosted.

## Features

- **HTTPS tunnels** — Each device gets a public subdomain (e.g. `mypi.yourdomain.com`)
- **Web dashboard** — See all devices, their status, and system metrics
- **In-browser terminal** — Click a device, get a shell. No SSH keys needed.
- **Group command execution** — Run a command across all devices in a tag group
- **Live monitoring** — CPU temp, memory, disk, uptime — updated in real time
- **Remote reboot** — One-click reboot from the dashboard
- **Device tagging** — Organize devices with custom tags
- **Bandwidth tracking** — Per-device usage tracking
- **Self-updating client** — `piportal upgrade` pulls the latest binary from your server

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
- A server with a domain and wildcard TLS cert (e.g. Caddy with Let's Encrypt)

## Quick Start

### 1. Build and deploy the server

```bash
# Build dashboard
cd piportal-dashboard && npm install && npm run build
cp -r dist ../piportal-server/dashboard/dist

# Build server
cd ../piportal-server
GOOS=linux GOARCH=amd64 go build -o piportal-server .

# Deploy to your server, configure with your domain
# See deploy/deploy.md for full instructions
```

### 2. Install the client on a Pi

```bash
# Download from your server
curl -fsSL https://yourdomain.com/downloads/piportal-linux-arm64 -o piportal
chmod +x piportal
sudo mv piportal /usr/local/bin/

# Set up (interactive)
piportal setup
```

The setup wizard will ask for:
- Your PiPortal server URL
- A subdomain for this device
- The local port to forward

### 3. Start the tunnel

```bash
piportal start
```

Or install as a system service:

```bash
sudo piportal service install
```

## Local Development

### Dashboard

```bash
cd piportal-dashboard
npm install
npm run dev
```

Runs on `http://localhost:5173` with hot reload.

### Server

```bash
cd piportal-server
go run . -dev -http :8080 -domain localhost
```

Dev mode uses a built-in JWT secret. The server serves:
- `/dashboard/*` — React SPA
- `/api/v1/*` — REST API
- `/tunnel` — WebSocket tunnel endpoint
- `*.yourdomain.com` — Proxies to connected devices

### Client

```bash
cd piportal-client
go run . setup
go run . start
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PIPORTAL_JWT_SECRET` | JWT signing secret (required in production) | Dev secret in `-dev` mode |
| `PIPORTAL_DOMAIN` | Base domain for tunnels | — |
| `PIPORTAL_DB` | Path to SQLite database file | `piportal.db` |

## Deploying

See [`deploy/deploy.md`](deploy/deploy.md) for full deployment docs.

Before running `deploy.sh`, create `deploy/.env`:

```bash
cp deploy/.env.example deploy/.env
# Edit with your SSH key and server
```

Then:

```bash
cd deploy
./deploy.sh
```

## Architecture

| Component | Stack |
|-----------|-------|
| Server | Go, SQLite, WebSocket, embedded SPA |
| Dashboard | React 19, TypeScript, Vite, React Router |
| Client | Go, Cobra CLI, WebSocket |
| Recommended Infra | Any VPS, Caddy (reverse proxy + wildcard TLS), systemd |

## Community

Questions, bugs, or contributions? Join the Discord:

**[discord.gg/uuYtV5Ukk7](https://discord.gg/uuYtV5Ukk7)**

## License

PiPortal is licensed under the [GNU Affero General Public License v3.0](LICENSE).
