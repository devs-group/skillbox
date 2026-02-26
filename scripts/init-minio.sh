#!/usr/bin/env bash
set -euo pipefail

# Initialize MinIO buckets for Skillbox.
# This script is used by the minio-init container in Docker Compose.

MINIO_ENDPOINT="${MINIO_ENDPOINT:-http://minio:9000}"
MINIO_ACCESS_KEY="${MINIO_ACCESS_KEY:-minioadmin}"
MINIO_SECRET_KEY="${MINIO_SECRET_KEY:-minioadmin}"

mc alias set skillbox "$MINIO_ENDPOINT" "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY"
mc mb --ignore-existing skillbox/skills
mc mb --ignore-existing skillbox/executions
echo "MinIO buckets initialized."
