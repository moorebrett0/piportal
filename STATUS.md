# PiPortal — Project Status

*Last updated: 2026-02-03*

---

## What's Working

### Server (piportal.dev)
- Go server running on DigitalOcean droplet, managed by systemd
- Caddy reverse proxy with wildcard TLS (`*.piportal.dev`) via Cloudflare DNS-01
- WebSocket tunnel connections from Pi clients
- Bandwidth tracking per device (1 GB free tier, 100 GB pro tier)
- SQLite database at `/var/lib/piportal/piportal.db`
- JWT auth with `PIPORTAL_JWT_SECRET` set via systemd drop-in (`/etc/systemd/system/piportal.service.d/env.conf`)

### Dashboard (piportal.dev/dashboard)
- React + TypeScript SPA, built with Vite, embedded in Go binary
- Vision UI dark theme — glassmorphism cards, gradient accents, sidebar navigation
- Login / signup with JWT cookie auth
- Device list with online/offline status and bandwidth bars
- Device detail page with setup instructions, bandwidth, danger zone (delete)
- Add device (create new) and claim device (by token) flows
- Deployed via `deploy/deploy.sh`

### Client
- Go CLI using Cobra, builds for arm64/arm/amd64
- `piportal setup` — interactive setup, registers device, saves config
- `piportal start` — connects WebSocket tunnel, forwards HTTP to local port
- `piportal status` — shows connection info and bandwidth usage
- `piportal upgrade` — self-upgrade from server
- systemd service file for running as a daemon
- Binaries available at `piportal.dev/downloads/piportal-linux-{arm64,arm,amd64}`
- Install script at `piportal.dev/install.sh`

### Database (4 devices, 1 user)
| Subdomain | Claimed | Online |
|-----------|---------|--------|
| dashboard | Yes     | Yes    |
| test-pi   | No      | No     |
| brett      | No      | No     |
| brett1     | No      | No     |

---

## What Needs Work

### High Priority
- **Landing page** — Root domain (`piportal.dev/`) serves a bare HTML stub. Needs a proper marketing page explaining what PiPortal is, how it works, and a CTA to sign up.
- **Unclaimed devices** — 3 devices in the DB are unclaimed (no user_id). Either claim them to your account or clean them up.
- **Auth cookie Secure flag** — `SetAuthCookie` only sets `Secure: true` when NOT in dev mode. Verify this is working correctly in production (behind Caddy with HTTPS).

### Medium Priority
- **Pricing page** — The `/upgrade` route has a placeholder HTML page. Need a real pricing page, eventually with Stripe integration for Pro tier ($3/mo).
- **Landing page / marketing site** — Could be a simple static page or part of the Go server. Needs: hero, how it works, pricing summary, CTA.
- **Error handling in dashboard** — No global error boundary. API errors could be surfaced better.
- **Device online status accuracy** — `is_online` is set when a tunnel connects but may not clear reliably on disconnect (verify graceful vs. ungraceful disconnects).

### Low Priority / Future
- **CI/CD** — No automated builds or deploys. Currently manual via `deploy/deploy.sh`.
- **Rate limiting** — No rate limiting on API endpoints or tunnel connections.
- **Request size limits** — No limits on WebSocket message size in tunnel protocol.
- **Backups** — SQLite database has no backup strategy. Single file at `/var/lib/piportal/piportal.db`.
- **Monitoring/alerting** — No health checks, uptime monitoring, or alerting.
- **Multi-device per user limits** — No cap on how many devices a user can create.
- **Password reset flow** — No forgot password / email verification.
- **Custom domains** — Users can only use `*.piportal.dev` subdomains.

---

## Architecture

```
Internet
  │
  ▼
Caddy (:443) — wildcard TLS for *.piportal.dev
  │
  ▼
Go Server (:8080)
  ├── /dashboard/*        → React SPA (embedded in binary)
  ├── /api/v1/*           → Dashboard REST API (JWT auth)
  ├── /api/register       → Device registration (client CLI)
  ├── /api/status         → Server health
  ├── /api/usage          → Bandwidth usage (token auth)
  ├── /tunnel             → WebSocket tunnel endpoint
  ├── /install.sh         → Client installer script
  ├── /downloads/*        → Client binaries
  └── *.piportal.dev      → Proxy through active tunnel
        │
        ▼
  Pi Client (WebSocket) → localhost:8080 on the Pi
```

---

## Key Files

| File | Purpose |
|------|---------|
| `deploy/deploy.sh` | Build + deploy server to DigitalOcean |
| `deploy/deploy.md` | Deployment reference (SSH, paths, commands) |
| `deploy/setup-server.sh` | One-time server provisioning script |
| `piportal-server/main.go` | Server entry point |
| `piportal-server/handler.go` | HTTP routing |
| `piportal-server/dashboard.go` | Dashboard API + SPA serving |
| `piportal-server/store.go` | SQLite database layer |
| `piportal-server/config.go` | Configuration (flags + env vars) |
| `piportal-server/tunnel.go` | Tunnel connection management |
| `piportal-dashboard/src/` | React dashboard source |
| `piportal-client/cmd/` | Client CLI commands |
| `piportal-client/install.sh` | Curl-able client installer |

---

## Deploy Cheatsheet

```bash
# Build dashboard (if changed)
cd ~/Sites/PiPortal/piportal-dashboard
npm run build
cp -r dist ../piportal-server/dashboard/dist

# Deploy server
cd ~/Sites/PiPortal/deploy
./deploy.sh

# SSH into server
ssh -i ~/.ssh/brett.rsa root@piportal.dev

# Logs
ssh -i ~/.ssh/brett.rsa root@piportal.dev "journalctl -u piportal -f"
```
