#!/bin/sh
# Double-clickable launcher for cron-agent on macOS.
# Finder runs binaries with $HOME as the working directory, which would send
# state to ~/data. This wrapper cds to its own folder first so ./data,
# ./.env, and --data all resolve next to the app. Keep it beside the
# cron-agent binary under the same names. Pass-through args are forwarded.
set -u
BUNDLE_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$BUNDLE_DIR" || exit 1
exec ./cron-agent "$@"
