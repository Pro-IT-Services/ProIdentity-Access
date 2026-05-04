#!/bin/bash
set -e
export PATH=$PATH:/usr/local/go/bin

APP_DIR="${PROIDENTITY_DIR:-/opt/wg-manager}"
SERVICE_NAME="${PROIDENTITY_SERVICE:-wg-manager}"

cd "$APP_DIR"

if grep -q 'jwt_secret: ""' config.yaml || grep -q 'change-this-to-a-random-secret-in-production' config.yaml; then
  if ! systemctl show "$SERVICE_NAME" -p Environment | grep -q 'PROIDENTITY_JWT_SECRET='; then
    echo "ERROR: set PROIDENTITY_JWT_SECRET in the service environment or config.yaml before deploy" >&2
    exit 1
  fi
fi

echo "==> Pulling latest code..."
git pull

echo "==> Building WebUI..."
cd webui
npm install --silent
npm run build
cd ..

echo "==> Copying UI to embed target..."
rm -rf internal/api/ui/dist
cp -r webui/dist internal/api/ui/dist

echo "==> Building server binary..."
go build -o bin/proidentity ./cmd/server/

echo "==> Restarting service..."
systemctl restart "$SERVICE_NAME"
sleep 1
systemctl status "$SERVICE_NAME" --no-pager | head -8
echo "==> Deploy complete"
