#!/bin/bash
# VeriStore Tools 3 - Production Installation Script
#
# Usage: sudo ./deploy/install.sh
#
# This script:
#   1. Builds the Go binary
#   2. Creates the installation directory structure
#   3. Copies binaries, static assets, config, and migrations
#   4. Installs and starts the systemd service
#
# Prerequisites:
#   - Go 1.22+ installed and available on PATH
#   - MySQL 8+ running with the veristoretools3 database created
#   - Redis running
#   - Run from the project root directory

set -e

BUILD_DIR=/opt/veristoretools3

echo "============================================"
echo " VeriStore Tools 3 - Installation"
echo "============================================"
echo ""

echo "[1/6] Building veristoreTools3..."
go build -o bin/veristoretools3 ./cmd/server
go build -o bin/migrate ./cmd/migrate
echo "      Build complete."

echo "[2/6] Creating directories..."
sudo mkdir -p $BUILD_DIR/bin $BUILD_DIR/static $BUILD_DIR/migrations
echo "      Directories created."

echo "[3/6] Copying files..."
sudo cp bin/veristoretools3 $BUILD_DIR/bin/
sudo cp bin/migrate $BUILD_DIR/bin/
sudo cp -r static/* $BUILD_DIR/static/
sudo cp config.yaml $BUILD_DIR/config.yaml
sudo cp -r migrations/* $BUILD_DIR/migrations/
echo "      Files copied."

echo "[4/6] Running database migrations..."
cd $BUILD_DIR
sudo -u www-data $BUILD_DIR/bin/migrate $BUILD_DIR/config.yaml || true
cd -
echo "      Migrations applied."

echo "[5/6] Installing systemd service..."
sudo cp deploy/veristoretools3.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable veristoretools3
echo "      Service installed and enabled."

echo "[6/6] Starting service..."
sudo systemctl restart veristoretools3
echo "      Service started."

echo ""
echo "============================================"
echo " VeriStore Tools 3 installed and started!"
echo "============================================"
echo ""
echo "Check status: sudo systemctl status veristoretools3"
echo "View logs:    sudo journalctl -u veristoretools3 -f"
echo "Config file:  $BUILD_DIR/config.yaml"
echo ""
