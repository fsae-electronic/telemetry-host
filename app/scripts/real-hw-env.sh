#!/usr/bin/env bash
# Starts telemetry-host connected to the real TMS570 Main ECU board:
#   TMS570 board (real UART) → tms570_bridge.py → socat virtual pair → telemetry host
# All processes log to this terminal. Ctrl+C shuts everything down.
#
# Usage: scripts/real-hw-env.sh [board-serial-port]
#   Defaults to /dev/ttyUSB0.

# Resolve absolute paths regardless of where the script is called from
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_DIR="$(cd "$APP_DIR/.." && pwd)"

BRIDGE="$REPO_DIR/utils/tms570_bridge.py"
BINARY="$APP_DIR/telemetry-host"
BOARD_PORT="${1:-/dev/ttyUSB0}"

# Prefer the repo's own venv (already has pyserial/msgpack/cobs) over system
# python3 -- keeps this working on systems with no pip installed globally.
PYTHON="python3"
if [ -x "$REPO_DIR/.venv/bin/python3" ]; then
  PYTHON="$REPO_DIR/.venv/bin/python3"
fi

# ── sanity checks ────────────────────────────────────────────────────────────

if [ ! -f "$BINARY" ]; then
  echo "ERROR: binary not found at $BINARY"
  echo "Build it first:"
  echo "  cd $APP_DIR && go build -o telemetry-host ./cmd/app/"
  exit 1
fi

if [ ! -f "$BRIDGE" ]; then
  echo "ERROR: bridge script not found at $BRIDGE"
  exit 1
fi

if [ ! -e "$BOARD_PORT" ]; then
  echo "ERROR: board port $BOARD_PORT does not exist."
  echo "Pass the right port as an argument, e.g.: $0 /dev/ttyUSB1"
  exit 1
fi

if ! command -v socat &>/dev/null; then
  echo "ERROR: socat not found."
  echo "  Arch:   sudo pacman -S socat"
  echo "  Debian: sudo apt install socat"
  exit 1
fi

if ! "$PYTHON" -c "import serial, msgpack, cobs" 2>/dev/null; then
  echo "ERROR: Python dependencies missing for $PYTHON."
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

# ── start the bridge ─────────────────────────────────────────────────────────

"$PYTHON" "$BRIDGE" -i "$BOARD_PORT" -o "$PTY2" &
PIDS+=($!)
sleep 0.3

if ! kill -0 "${PIDS[-1]}" 2>/dev/null; then
  echo "ERROR: bridge failed to start. Check that $BOARD_PORT is the right port"
  echo "and that this user has permission to open it (dialout/uucp group)."
  exit 1
fi

echo "Board bridge:         $BOARD_PORT -> $PTY2"

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
echo "  Dashboard:   http://localhost:8080"
echo "  Board port:  $BOARD_PORT (real TMS570 Main ECU)"
echo "  RX port:     $PTY1  (telemetry host)"
echo "─────────────────────────────────────────────────"
echo "  Press Ctrl+C to stop everything."
echo ""

wait
