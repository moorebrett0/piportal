#!/bin/bash
# Quick deploy script - run from your local machine
# Usage: ./deploy.sh

set -e

SSH_KEY="~/.ssh/brett.rsa"
SERVER="root@piportal.dev"
REMOTE_DIR="/opt/piportal"
DOWNLOADS_DIR="/var/www/piportal/downloads"
SSH_OPTS="-i ${SSH_KEY}"
BASE_DIR="$(dirname "$0")/.."

echo "=== Building client binaries ==="
cd "${BASE_DIR}/piportal-client"
GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o piportal-linux-arm64 .
GOOS=linux GOARCH=arm   go build -ldflags "-s -w" -o piportal-linux-arm .
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o piportal-linux-amd64 .
echo "Client binaries built."

echo ""
echo "=== Building server binary ==="
cd "${BASE_DIR}/piportal-server"
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o piportal-server-linux .
echo "Server binary built."

echo ""
echo "=== Stopping service ==="
ssh ${SSH_OPTS} ${SERVER} "systemctl stop piportal"

echo ""
echo "=== Uploading server binary ==="
scp ${SSH_OPTS} piportal-server-linux ${SERVER}:${REMOTE_DIR}/piportal-server
ssh ${SSH_OPTS} ${SERVER} "chmod +x ${REMOTE_DIR}/piportal-server"

echo ""
echo "=== Uploading client binaries ==="
ssh ${SSH_OPTS} ${SERVER} "mkdir -p ${DOWNLOADS_DIR}"
cd "${BASE_DIR}/piportal-client"
scp ${SSH_OPTS} piportal-linux-arm64 piportal-linux-arm piportal-linux-amd64 ${SERVER}:${DOWNLOADS_DIR}/
ssh ${SSH_OPTS} ${SERVER} "chmod +x ${DOWNLOADS_DIR}/piportal-linux-*"

echo ""
echo "=== Starting service ==="
ssh ${SSH_OPTS} ${SERVER} "systemctl start piportal"

echo ""
echo "=== Checking status ==="
ssh ${SSH_OPTS} ${SERVER} "systemctl status piportal --no-pager | head -15"

echo ""
echo "Done! Server and client binaries deployed."
echo "Clients can upgrade with: piportal upgrade"
echo "View logs: ssh ${SSH_OPTS} ${SERVER} journalctl -u piportal -f"
