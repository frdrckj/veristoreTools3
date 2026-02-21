#!/bin/bash
# =============================================================
# VeriStore Tools 3 — Offline Deploy Script
# =============================================================
# Run this on your Mac to build and package everything.
# Then transfer the "deploy-package" folder to the server.
# =============================================================

set -e

DEPLOY_DIR="deploy-package"

echo "=== Step 1: Building app image for linux/amd64 ==="
docker build --platform linux/amd64 -t veristoretools3-app:latest .

echo ""
echo "=== Step 2: Pulling dependency images for linux/amd64 ==="
docker pull --platform linux/amd64 mysql:8.0
docker pull --platform linux/amd64 redis:7-alpine

echo ""
echo "=== Step 3: Saving all images to tar.gz ==="
rm -rf "$DEPLOY_DIR"
mkdir -p "$DEPLOY_DIR"
docker save veristoretools3-app:latest mysql:8.0 redis:7-alpine | gzip > "$DEPLOY_DIR/veristoretools3-images.tar.gz"

echo ""
echo "=== Step 4: Copying config files ==="
cp docker-compose.prod.yml "$DEPLOY_DIR/docker-compose.yml"
cp config.docker.yaml "$DEPLOY_DIR/config.docker.yaml"

# Create the server setup script
cat > "$DEPLOY_DIR/setup.sh" << 'SETUP'
#!/bin/bash
# =============================================================
# Run this on the SERVER (Linux, no internet)
# =============================================================
set -e

echo "=== Loading Docker images ==="
docker load < veristoretools3-images.tar.gz

echo ""
echo "=== Starting services ==="
docker compose up -d

echo ""
echo "=== Done! ==="
echo "App is running at http://localhost:8080"
echo ""
echo "To check status:  docker compose ps"
echo "To view logs:     docker compose logs -f app"
echo "To stop:          docker compose down"
echo "To restart:       docker compose restart app"
SETUP
chmod +x "$DEPLOY_DIR/setup.sh"

echo ""
echo "=== Done! ==="
echo ""
echo "Package created in: $DEPLOY_DIR/"
ls -lh "$DEPLOY_DIR/"
echo ""
echo "Transfer the '$DEPLOY_DIR' folder to the server, then run:"
echo "  cd $DEPLOY_DIR && ./setup.sh"
echo ""
echo "IMPORTANT: Edit config.docker.yaml on the server before starting"
echo "           to set the correct database, TMS, and API credentials."
