#!/usr/bin/env bash
set -euo pipefail

# Generate and seed an initial API key for development.
#
# Usage: bash scripts/seed-apikey.sh
#
# Requires: psql, openssl (or /dev/urandom)

DB_DSN="${SKILLBOX_DB_DSN:-postgres://skillbox:skillbox@localhost:5432/skillbox?sslmode=disable}"
TENANT_ID="${SKILLBOX_TENANT_ID:-default}"

# Generate a random API key
API_KEY="sk-$(openssl rand -hex 24)"

# SHA-256 hash for storage
KEY_HASH=$(echo -n "$API_KEY" | shasum -a 256 | cut -d' ' -f1)

# Insert into database
psql "$DB_DSN" <<SQL
INSERT INTO sandbox.api_keys (key_hash, tenant_id, name)
VALUES ('$KEY_HASH', '$TENANT_ID', 'dev-key')
ON CONFLICT (key_hash) DO NOTHING;
SQL

echo ""
echo "API key created successfully."
echo ""
echo "  Key:    $API_KEY"
echo "  Tenant: $TENANT_ID"
echo ""
echo "Export it:"
echo "  export SKILLBOX_API_KEY=$API_KEY"
echo ""
