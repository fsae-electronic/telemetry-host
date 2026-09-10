# FSAE Telemetry Host

A self-contained telemetry dashboard for the FSAE eDrive car. Reads sensor data over serial (UART), displays it in a live configurable dashboard, and stores runs to a local database for post-session analysis.

The entire application ships as a **single binary** — no Python, no Node, no installers. Users run one file and open a browser.

---

## Architecture

```
Serial (UART)
  └─ COBS framing → MessagePack decode
        └─ Go backend  ──WebSocket──▶  Browser UI (Svelte)
              └─ SQLite (telemetry.db)       └─ uPlot charts
```

The Go binary embeds the compiled frontend at build time. On launch it serves the UI at `http://localhost:8080` and auto-opens it in the default browser.

### Protocol
Incoming frames are **COBS-encoded MessagePack** dictionaries over UART at 115200 baud. Each frame is terminated with a `0x00` byte. The receiver side in `src/rx.py` documents the same protocol. The real TMS570 Main ECU speaks a different, fixed-size binary frame natively -- `utils/tms570_bridge.py` translates between the two (see "Real hardware" below).

### Sensor channels

Channel keys, labels, and grouping live in `frontend/src/lib/channels.js` (only keys defined there are selectable in the panel editor). As currently set up for the TMS570 Main ECU:

| Key | Sensor | Group |
|-----|--------|-------|
| `tps1`, `tps2` | Throttle position sensors | TPS |
| `wheel_speed_fl`, `wheel_speed_fr` | Front wheel speed | Front Axle |
| `steering_angle` | Steering angle | Front Axle |
| `front_brake` | Front brake pressure | Front Axle |
| `ac_current_1`, `ac_current_2`, `dc_current_1`, `dc_current_2` | Driver AC/DC currents | ADC Currents |
| `rear_brake` | Rear brake pressure | Rear |
| `drv1_*`, `drv2_*` | Motor driver PDOs (velocity, current, temps, status/warnings/error, control word) | Driver 1 / Driver 2 |

The `drv1_*`/`drv2_*` fields only appear once a motor driver node is actually transmitting on the CAN bus -- the bridge omits fields for messages it hasn't seen rather than sending misleading zeros.

---

## Prerequisites

### To build from source

| Tool | Minimum version | Install |
|------|----------------|---------|
| Go | 1.21 | https://go.dev/dl |
| Node.js | 18 | https://nodejs.org |
| npm | 9 | bundled with Node |

### To run a pre-built binary

Nothing. Just the binary.

### For the test environment (virtual serial loopback)

```bash
# Arch / Manjaro
sudo pacman -S socat python python-pip

# Ubuntu / Debian
sudo apt install socat python3 python3-pip
```

Python dependencies for the mock transmitter:

```bash
pip install pyserial msgpack cobs
```

---

## Building

Both steps must be run from the `app/` directory.

**Step 1 — build the frontend:**

```bash
cd app/frontend
npm install
npm run build
```

This compiles the Svelte app into `app/cmd/app/frontend/`, where the Go compiler picks it up via `go:embed`.

**Step 2 — build the binary:**

```bash
cd app
go build -o telemetry-host ./cmd/app/
```

### Cross-compiling for Windows

From Linux, produce a Windows executable with no extra tools:

```bash
cd app
GOOS=windows GOARCH=amd64 go build -o telemetry-host.exe ./cmd/app/
```

> The frontend must be built before either `go build`. The Go build embeds the compiled JS/CSS from `cmd/app/frontend/` at compile time — if that directory is missing the build will fail.

---

## Running

```bash
./telemetry-host
```

The app will log the URL and open it automatically:

```
telemetry host running at http://localhost:8080
```

### CLI flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8080` | HTTP server port |
| `--db` | `telemetry.db` | Path to the SQLite database file |
| `--no-open` | off | Skip auto-opening the browser |

```bash
./telemetry-host --port 9090 --db /data/season2025.db
```

### Linux serial port permissions

On Linux, your user must be in the `dialout` group to access serial ports without `sudo`. This is a one-time setup:

```bash
sudo usermod -aG dialout $USER
# log out and back in for it to take effect
```

---

## Test environment (virtual serial loopback)

Use the provided script to spin up the entire test environment in one command. It starts socat, the mock transmitter, and the telemetry host automatically, then connects them together.

```bash
cd app
bash scripts/test-env.sh
```

The script will print the dashboard URL and the virtual port names, then stream all logs to the terminal. **Press Ctrl+C** to shut everything down cleanly.

```
Virtual serial link:  /dev/pts/5  ↔  /dev/pts/7
Mock transmitter:     running on /dev/pts/7 at 10 Hz
Serial connection:    connected to /dev/pts/5

─────────────────────────────────────────────────
  Dashboard:  http://localhost:8080
  TX port:    /dev/pts/7  (mock transmitter)
  RX port:    /dev/pts/5  (telemetry host)
─────────────────────────────────────────────────
  Press Ctrl+C to stop everything.
```

The browser opens automatically. The serial connection is made for you — data should be flowing into the dashboard immediately.

### What the script does

1. Starts `socat` to create a linked pair of virtual serial ports
2. Starts `mock_tx.py` on one end, transmitting 12 channels of sine-wave data at 10 Hz
3. Starts `./telemetry-host` and immediately connects it to the other end via the REST API

### Prerequisites check

The script will tell you if anything is missing before it starts:

| Missing | Error shown | Fix |
|---------|-------------|-----|
| Binary not built | `binary not found` | `go build -o telemetry-host ./cmd/app/` |
| socat not installed | `socat not found` | `sudo pacman -S socat` / `sudo apt install socat` |
| Python deps missing | `Python dependencies missing` | `pip install pyserial msgpack cobs` |

### Port field note

The port dropdown auto-suggests real hardware serial ports (ttyUSB*, ttyACM*). Virtual PTY paths from socat won't appear there — that's normal. The script connects automatically. If you ever need to connect manually, type the path directly into the port field (e.g. `/dev/pts/5`).

> On Windows with a real USB-serial adapter, replace the port field with `COM3` (or whichever port appears in Device Manager). The script is Linux-only; on Windows just run the binary directly and enter the COM port manually.

---

## Real hardware (TMS570 Main ECU)

The Main ECU board (tms570_freertos_v2) sends its own fixed-size binary telemetry frame, not COBS/MessagePack directly -- `utils/tms570_bridge.py` decodes those frames and re-encodes them in the protocol above. `scripts/real-hw-env.sh` wires this up the same way `test-env.sh` does for the mock transmitter, just with the real board on one end instead of synthetic data:

```bash
cd app
bash scripts/real-hw-env.sh /dev/ttyUSB0   # defaults to /dev/ttyUSB0 if omitted
```

This starts a socat virtual pair, points the bridge at the board's real serial port on one end, starts `telemetry-host`, and connects it to the other end automatically -- same log-everything-to-this-terminal, Ctrl+C-to-stop behavior as `test-env.sh`.

To run the bridge by itself (e.g. against an already-running `telemetry-host` you connected manually to a socat/virtual port):

```bash
python3 utils/tms570_bridge.py -i /dev/ttyUSB0 -o /dev/pts/7
```

See the module docstring in `tms570_bridge.py` for the exact frame layout it expects -- keep it in lockstep with `vPackFrame()` in `tms570_freertos_v2/app/main.c` if that ever changes.

---

## Using the dashboard

### Live view

The default layout shows all sensor groups:

- **Brake Sensors** — overlaid time series for both brake sensors
- **Battery Level / Steering Angle** — arc gauges
- **Accel Pedal / Brake Pedal** — horizontal bar gauges with live fill
- **Motor Currents** — overlaid time series for all four drivers

The chart scrolls a 30-second window of live data.

### Editing the layout

Click **⊹ Edit Layout** in the toolbar to enter edit mode:

- **Move** — drag a panel by its title bar to reposition it on the grid
- **Resize** — drag the **⊿** handle in the bottom-right corner
- **Edit** — click **✎** to change a panel's type, channels, range, or size
- **Remove** — click **✕** on a panel to delete it
- **Add** — click **＋ Add Panel**, then configure:
  - **Type**: Time Series, Gauge, Stat, Bar Gauge, or XY Scatter
  - **Channels**: pick individually or click a group button to select all channels in that group
  - **Range**: min/max values (used by Gauge and Bar Gauge)
  - **Size**: width and height in grid units (grid is 12 columns wide, each row is 80 px)
- **Reset** — click **↺ Reset** to restore the default layout

Layout changes are saved automatically to `localStorage` in the browser and persist between sessions.

### Panel types

| Type | Best for |
|------|----------|
| Time Series | Any channel over time; supports multiple channels overlaid |
| Gauge | Single channel with a min/max range — battery %, steering angle |
| Stat | Large numeric readout, good for quick at-a-glance values |
| Bar Gauge | Pedal positions, any 0–100% signal; multiple channels stack vertically |
| XY Scatter | Correlation between two channels (select exactly 2) |

### Recording a run

1. Connect to the serial port
2. Click **● Record** in the status bar — the button turns red and a blinking **REC** indicator appears
3. Click **■ Stop** to end the recording

Recorded runs are stored in `telemetry.db` and appear in the **Runs** tab. Click a run to load it into a chart. Runs can be deleted from the list.

---

## Project structure

```
app/
├── cmd/app/
│   ├── main.go              # Entry point — HTTP server, go:embed
│   └── frontend/            # Compiled Svelte output (gitignored, built by npm)
├── internal/
│   ├── serial/
│   │   ├── cobs.go          # COBS encode/decode
│   │   └── serial.go        # Serial port manager, read loop, packet parser
│   ├── hub/
│   │   └── hub.go           # WebSocket broadcast hub
│   ├── store/
│   │   └── store.go         # SQLite run storage (pure Go, no system deps)
│   └── api/
│       └── api.go           # REST + WebSocket HTTP handlers
├── frontend/                # Svelte source
│   └── src/
│       ├── lib/
│       │   ├── channels.js  # Sensor names, groups, colors
│       │   ├── dashboard.js # Dashboard layout store (localStorage)
│       │   └── ws.js        # WebSocket client + ring buffer
│       └── routes/
│           ├── Dashboard.svelte
│           └── Runs.svelte
├── go.mod
└── telemetry.db             # Created automatically on first run
```

---

## REST API reference

All endpoints are on `http://localhost:8080`.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/ports` | List available serial ports |
| `POST` | `/api/connect` | Connect: `{"port": "/dev/ttyUSB0", "baud": 115200}` |
| `POST` | `/api/disconnect` | Disconnect (stops any active recording) |
| `GET` | `/api/status` | Current connection + recording state |
| `GET` | `/api/runs` | List all stored runs |
| `POST` | `/api/runs` | Start recording: `{"name": "optional name"}` |
| `POST` | `/api/runs/{id}/stop` | Stop recording |
| `GET` | `/api/runs/{id}` | Get run metadata |
| `GET` | `/api/runs/{id}/samples` | Get all samples for a run (JSON array) |
| `DELETE` | `/api/runs/{id}` | Delete a run and all its samples |
| `WS` | `/ws` | WebSocket — streams live packets and status updates |

### WebSocket message format

**Incoming packet** (from car):
```json
{ "type": "packet", "ts": 1714123456.789, "data": { "s_00": 12.3, "s_01": 11.9, ... } }
```

**Status update** (on connect/disconnect/record):
```json
{ "type": "status", "data": { "connected": true, "port": "/dev/pts/2", "recording": false, "active_run_id": 0 } }
```
