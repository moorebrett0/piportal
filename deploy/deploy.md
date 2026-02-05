# PiPortal Deployment

## Infrastructure

- **Server:** DigitalOcean droplet running Ubuntu 24.04
- **Host:** `piportal.dev` / `root@piportal.dev`
- **SSH key:** `~/.ssh/brett.rsa`
- **SSH command:** `ssh -i ~/.ssh/brett.rsa root@piportal.dev`

## Server Layout

| Path | Purpose |
|------|---------|
| `/opt/piportal/piportal-server` | Go server binary |
| `/var/lib/piportal/piportal.db` | SQLite database |
| `/etc/caddy/Caddyfile` | Caddy reverse proxy config |
| `/etc/caddy/environment` | Cloudflare API token (for wildcard TLS) |
| `/etc/systemd/system/piportal.service` | systemd service |

## Services

- **caddy** — Reverse proxy, handles TLS (Let's Encrypt wildcard via Cloudflare DNS-01)
- **piportal** — Go server listening on `:8080`, Caddy proxies to it

## How It Works

Caddy handles `piportal.dev` and `*.piportal.dev`, terminates TLS, and proxies everything to the Go server on port 8080. The Go server runs with `-behind-proxy` flag so it trusts Caddy for TLS. The dashboard frontend is embedded in the server binary (built assets in `piportal-server/dashboard/dist/`).

## Server Layout (continued)

| Path | Purpose |
|------|---------|
| `/var/www/piportal/downloads/piportal-linux-arm64` | Client binary (Pi 4/5) |
| `/var/www/piportal/downloads/piportal-linux-arm` | Client binary (Pi Zero/older) |
| `/var/www/piportal/downloads/piportal-linux-amd64` | Client binary (x86) |

## Deploy (from local machine)

```bash
# 1. Build the dashboard (if changed)
cd ~/Sites/PiPortal/piportal-dashboard
npm run build
cp -r dist ../piportal-server/dashboard/dist

# 2. Deploy server + client binaries
cd ~/Sites/PiPortal/deploy
./deploy.sh
```

The deploy script:
1. Cross-compiles client binaries for linux/arm64, linux/arm, and linux/amd64
2. Builds the server binary for linux/amd64 (with embedded dashboard)
3. Stops the piportal service
4. Uploads server binary to `/opt/piportal/`
5. Uploads client binaries to `/var/www/piportal/downloads/`
6. Restarts the service

## Updating Clients

After deploying new client binaries, devices can self-upgrade:

```bash
sudo piportal upgrade
sudo systemctl restart piportal
```

The client checks `/api/version` for the latest version, downloads the correct binary for its architecture from `/downloads/piportal-linux-{arch}`, and replaces itself.

## Useful Commands

```bash
# SSH into server
ssh -i ~/.ssh/brett.rsa root@piportal.dev

# Check service status
systemctl status piportal
systemctl status caddy

# View logs
journalctl -u piportal -f
journalctl -u caddy -f

# Restart services
systemctl restart piportal
systemctl restart caddy
```

## Initial Server Setup

Only needed once (already done). The `setup-server.sh` script:
1. Installs Caddy with Cloudflare DNS plugin
2. Creates `piportal` system user
3. Sets up directories (`/opt/piportal`, `/var/lib/piportal`)
4. Writes Caddyfile and systemd service
5. Enables services

Requires `CF_API_TOKEN` environment variable for Cloudflare DNS-01 challenge.

## Architecture

```
Internet → Caddy (:443) → Go server (:8080) → Dashboard / API / WebSocket tunnels
                                                    ↕
Pi devices connect via WebSocket to /tunnel endpoint
```

## Domain & DNS

- `piportal.dev` — main site + dashboard
- `*.piportal.dev` — tunnel subdomains (each device gets `<subdomain>.piportal.dev`)
- DNS managed via Cloudflare
- Wildcard TLS certificate via Let's Encrypt DNS-01 challenge
