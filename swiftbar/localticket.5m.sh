#!/bin/sh
# SwiftBar plugin shim for lt. Refresh interval is the .5m. component of the
# filename - rename the file to change it (e.g. localticket.10m.sh).
# Drop this file in ~/Library/Application Support/SwiftBar/Plugins/ and
# `chmod +x` it.

# Resolve lt: prefer PATH, fall back to common Go install locations.
if command -v lt >/dev/null 2>&1; then
  LT=$(command -v lt)
elif [ -x "$HOME/go/bin/lt" ]; then
  LT="$HOME/go/bin/lt"
elif [ -x "/usr/local/bin/lt" ]; then
  LT="/usr/local/bin/lt"
else
  echo " | sfimage=exclamationmark.triangle"
  echo "---"
  echo "lt binary not found"
  echo "Install: go install github.com/jumoel/localticket/cmd/lt@latest"
  exit 0
fi

exec "$LT" summary --swiftbar
