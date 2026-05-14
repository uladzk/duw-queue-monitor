#!/bin/bash

set -euo pipefail

# Auth setup (one-time per shell):
#   az acr login --name acrduwshared
#   gh auth token | crane auth login ghcr.io -u uladzk --password-stdin

readonly ACR="acrduwshared.azurecr.io"
readonly GHCR="ghcr.io/uladzk"
readonly IMAGES=(
  "queue-monitor:1.4.0"
  "telegram-bot:1.0.0"
  "queue-stats-reports:1.1.0"
  "duw-migrations:1.1.0"
)

if ! command -v crane >/dev/null 2>&1; then
  echo "❌ crane not found. Install: brew install crane (or: go install github.com/google/go-containerregistry/cmd/crane@latest)"
  exit 1
fi

for IMAGE in "${IMAGES[@]}"; do
  echo "📦 Copying $IMAGE..."
  crane copy "$ACR/$IMAGE" "$GHCR/$IMAGE"
  crane copy "$ACR/$IMAGE" "$GHCR/${IMAGE%%:*}:latest"
done

echo "✅ Done. Verify at https://github.com/uladzk?tab=packages"
