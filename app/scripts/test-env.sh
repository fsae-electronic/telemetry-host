#!/usr/bin/env bash
# Starts the full test environment in one shot:
#   socat virtual serial pair → mock transmitter → telemetry host
# All three processes log to this terminal. Ctrl+C shuts everything down.

# Resolve absolute paths regardless of where the script is called from
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_DIR="$(cd "$APP_DIR/.." && pwd)"

MOCK_TX="$REPO_DIR/utils/mock_tx.py"
BINARY="$APP_DIR/telemetry-host"
RATE=10

# ── sanity checks ────────────────────────────────────────────────────────────

if [ ! -f "$BINARY" ]; then
  echo "ERROR: binary not found at $BINARY"
  echo "Build it first:"
  echo "  cd $APP_DIR && go build -o telemetry-host ./cmd/app/"
  exit 1
fi

if [ ! -f "$MOCK_TX" ]; then
  echo "ERROR: mock transmitter not found at $MOCK_TX"
  exit 1
fi

if ! command -v socat &>/dev/null; then
  echo "ERROR: socat not found."
  echo "  Arch:   sudo pacman -S socat"
  echo "  Debian: sudo apt install socat"
  exit 1
fi

if ! python3 -c "import serial, msgpack, cobs" 2>/dev/null; then
  echo "ERROR: Python dependencies missing."
  echo "  pip install pyserial msgpack cobs"
  exit 1
fi

# ── cleanup ──────────────────────────────────────────────────────────────────
# Only trap EXIT so cleanup runs exactly once — whether we exit normally,
# via Ctrl+C (INT → exit 0 → EXIT), or via TERM.

PIDS=()

cleanup() {
  echo ""
  echo "Shutting down..."
  for pid in "${PIDS[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
  wait 2>/dev/null || true
  echo "Done."
}

trap cleanup EXIT
trap 'exit 0' INT TERM

# ── start socat ──────────────────────────────────────────────────────────────

SOCAT_LOG=$(mktemp)
socat -d -d pty,raw,echo=0 pty,raw,echo=0 2>"$SOCAT_LOG" &
PIDS+=($!)

for _ in $(seq 20); do
  sleep 0.2
  [ "$(grep -c "N PTY is" "$SOCAT_LOG" 2>/dev/null || echo 0)" -ge 2 ] && break
done

PTY1=$(sed -n '1s/.*PTY is //p' "$SOCAT_LOG")
PTY2=$(sed -n '2s/.*PTY is //p' "$SOCAT_LOG")
rm -f "$SOCAT_LOG"

if [ -z "$PTY1" ] || [ -z "$PTY2" ]; then
  echo "ERROR: socat did not create PTY pair in time."
  exit 1
fi

echo "Virtual serial link:  $PTY1  ↔  $PTY2"

# ── start mock transmitter ───────────────────────────────────────────────────

python3 "$MOCK_TX" -p "$PTY2" -r "$RATE" &
PIDS+=($!)
sleep 0.3

if ! kill -0 "${PIDS[-1]}" 2>/dev/null; then
  echo "ERROR: mock transmitter failed to start."
  exit 1
fi

echo "Mock transmitter:     running on $PTY2 at ${RATE} Hz"

# ── start telemetry host ─────────────────────────────────────────────────────

"$BINARY" --no-open &
PIDS+=($!)
sleep 0.5

PORT_RESULT=$(curl -sf -X POST http://localhost:8080/api/connect \
  -H 'Content-Type: application/json' \
  -d "{\"port\":\"$PTY1\",\"baud\":115200}" 2>/dev/null || true)

if echo "$PORT_RESULT" | grep -q '"connected":true'; then
  echo "Serial connection:    connected to $PTY1"
else
  echo "WARNING: auto-connect failed ($PORT_RESULT)"
  echo "Open the browser and connect manually to $PTY1"
fi

xdg-open "http://localhost:8080" 2>/dev/null || true

echo ""
echo "─────────────────────────────────────────────────"
echo "  Dashboard:  http://localhost:8080"
echo "  TX port:    $PTY2  (mock transmitter)"
echo "  RX port:    $PTY1  (telemetry host)"
echo "─────────────────────────────────────────────────"
echo "  Press Ctrl+C to stop everything."
echo ""

wait
