#!/bin/bash
# PiPortal Server Setup Script for Ubuntu 24.04
# Run as root on your DigitalOcean droplet

set -e

DOMAIN="piportal.dev"
APP_USER="piportal"
APP_DIR="/opt/piportal"

echo "═══════════════════════════════════════════"
echo "  PiPortal Server Setup"
echo "═══════════════════════════════════════════"
echo ""

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root"
   exit 1
fi

# Get Cloudflare API token for DNS challenge
if [[ -z "$CF_API_TOKEN" ]]; then
    echo "Enter your Cloudflare API Token (for DNS-01 challenge):"
    read -s CF_API_TOKEN
    echo ""
fi

echo "[1/7] Installing dependencies..."
apt-get update -qq
apt-get install -y -qq debian-keyring debian-archive-keyring apt-transport-https curl

echo "[2/7] Installing Caddy..."
# Caddy handles TLS automatically with Let's Encrypt
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
apt-get update -qq
apt-get install -y -qq caddy

# Install Caddy with Cloudflare DNS module
echo "[2b/7] Installing Caddy with Cloudflare DNS plugin..."
caddy stop 2>/dev/null || true
curl -L "https://caddyserver.com/api/download?os=linux&arch=amd64&p=github.com%2Fcaddy-dns%2Fcloudflare" -o /usr/bin/caddy
chmod +x /usr/bin/caddy

echo "[3/7] Creating piportal user..."
id -u $APP_USER &>/dev/null || useradd --system --no-create-home --shell /usr/sbin/nologin $APP_USER

echo "[4/7] Setting up application directory..."
mkdir -p $APP_DIR
mkdir -p /var/lib/piportal

echo "[5/7] Writing Caddyfile..."
cat > /etc/caddy/Caddyfile << EOF
# PiPortal Caddy Configuration

# Global options
{
    email admin@${DOMAIN}
}

# Main site
${DOMAIN} {
    reverse_proxy localhost:8080

    tls {
        dns cloudflare {env.CF_API_TOKEN}
    }
}

# Wildcard for tunnel subdomains
*.${DOMAIN} {
    reverse_proxy localhost:8080

    tls {
        dns cloudflare {env.CF_API_TOKEN}
    }
}
EOF

echo "[6/7] Writing systemd service files..."

# Caddy environment file (for Cloudflare token)
cat > /etc/caddy/environment << EOF
CF_API_TOKEN=${CF_API_TOKEN}
EOF
chmod 600 /etc/caddy/environment

# Override Caddy service to load environment
mkdir -p /etc/systemd/system/caddy.service.d
cat > /etc/systemd/system/caddy.service.d/override.conf << EOF
[Service]
EnvironmentFile=/etc/caddy/environment
EOF

# PiPortal service
cat > /etc/systemd/system/piportal.service << EOF
[Unit]
Description=PiPortal Tunnel Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${APP_USER}
Group=${APP_USER}
WorkingDirectory=${APP_DIR}
ExecStart=${APP_DIR}/piportal-server -http :8080 -domain ${DOMAIN} -db /var/lib/piportal/piportal.db -behind-proxy
Restart=on-failure
RestartSec=5

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=/var/lib/piportal

[Install]
WantedBy=multi-user.target
EOF

# Set permissions
chown -R $APP_USER:$APP_USER /var/lib/piportal
chmod 700 /var/lib/piportal

echo "[7/7] Enabling services..."
systemctl daemon-reload
systemctl enable caddy
systemctl enable piportal

echo ""
echo "═══════════════════════════════════════════"
echo "  Setup Complete!"
echo "═══════════════════════════════════════════"
echo ""
echo "Next steps:"
echo ""
echo "  1. Upload the server binary:"
echo "     scp piportal-server-linux root@178.128.130.4:${APP_DIR}/piportal-server"
echo ""
echo "  2. Start the services:"
echo "     systemctl start piportal"
echo "     systemctl start caddy"
echo ""
echo "  3. Check status:"
echo "     systemctl status piportal"
echo "     systemctl status caddy"
echo "     curl https://${DOMAIN}/api/status"
echo ""
echo "  4. View logs:"
echo "     journalctl -u piportal -f"
echo "     journalctl -u caddy -f"
echo ""
