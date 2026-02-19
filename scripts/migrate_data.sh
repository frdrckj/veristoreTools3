#!/bin/bash
# Data migration script from veristoretools2 to veristoretools3
# Usage: ./scripts/migrate_data.sh [mysql_user] [mysql_password]
#
# This script:
#   1. Dumps row data (no schema) from veristoretools2
#   2. Imports the data into veristoretools3
#   3. Runs the verify subcommand to compare row counts
#
# Prerequisites:
#   - Both databases must exist and the v3 schema must already be applied
#     (run: go run ./cmd/migrate)
#   - MySQL client tools (mysql, mysqldump) must be available on PATH

set -e

MYSQL_USER="${1:-root}"
MYSQL_PASS="${2:-}"
V2_DB="veristoretools2"
V3_DB="veristoretools3"
DUMP_FILE="/tmp/v2_data.sql"

# Build password flag only if a password was provided.
PASS_FLAG=""
if [ -n "$MYSQL_PASS" ]; then
    PASS_FLAG="-p${MYSQL_PASS}"
fi

echo "============================================"
echo " VeriStore Tools: Data Migration v2 -> v3"
echo "============================================"
echo ""

echo "[1/3] Dumping data from ${V2_DB}..."
mysqldump -u "$MYSQL_USER" $PASS_FLAG "$V2_DB" \
    --no-create-info \
    --complete-insert \
    --skip-triggers \
    --set-gtid-purged=OFF \
    > "$DUMP_FILE"
echo "      Dump saved to ${DUMP_FILE}"

echo "[2/3] Importing into ${V3_DB}..."
mysql -u "$MYSQL_USER" $PASS_FLAG "$V3_DB" < "$DUMP_FILE"
echo "      Import complete."

echo "[3/3] Verifying row counts..."
go run ./cmd/migrate verify
echo ""

echo "============================================"
echo " Migration complete!"
echo "============================================"
