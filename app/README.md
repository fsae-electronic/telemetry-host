# FSAE Telemetry Host

A self-contained telemetry dashboard for the FSAE eDrive car. Reads sensor data over serial (UART), displays it in a live configurable dashboard, and stores runs to a local database for post-session analysis.

The entire application ships as a **single binary** — no Python, no Node, no installers. Users run one file and open a browser.

---

## Architecture

```
Serial (UART)
  └─ Binary v3 frames: fast 130 B @ 10 Hz + slow 60 B @ 1 Hz (0xAA 0x55 sync, CRC-16)
        └─ Go backend  ──WebSocket──▶  Browser UI (Svelte)
              └─ SQLite (telemetry.db)       └─ uPlot charts
```

The Go binary embeds the compiled frontend at build time. On launch it serves the UI at `http://localhost:8080` and auto-opens it in the default browser.

### Protocol
The main ECU (`tms570_main_ecu`, `app/telemetry.c`) sends **telemetry frames v3** over UART (XBee, 115200 8N1). The full byte-by-byte layout is in the spec "Frame de telemetría v3"; `internal/serial/frame.go` holds the same layout as tables (`v3FastLayout`, `v3SlowLayout`).

Every frame has a 10-byte header (sync `0xAA 0x55`, version `3`, type, seq, payload_len, ecu_time_ms), a payload and a CRC-16/CCITT-FALSE over bytes 2 … end of payload, all little-endian:

| Type | Size | Rate | Contents |
|------|------|------|----------|
| 1 fast | 130 B | 10 Hz | validity masks, ECU state, torque limit reasons, tractive system, pedals, brakes, steering, wheel speeds, torque request/commands, BMS, currents, IMU, coolant, LV, 2 × 23-byte driver block |
| 2 slow | 60 B | 1 Hz | firmware version and git hash, uptime, resets, CPU load, loop overruns, ECU faults, CAN error counters, motor rated current/torque, cell IDs |

The parser hunts for the sync, checks version, payload length and CRC, and resynchronises after garbage or partial frames. It counts frames with a bad CRC and frames missing from each type's `seq` (a restart of the ECU, seen as `ecu_time` going back, starts a new count). Only v3 is accepted.

Values are published in physical units (the field scale is applied: TPS in %, wheel speeds in rpm, voltages in V, …), with these exceptions:

- `steering_angle` and the four currents are the raw CAN values (0x502 / 0x503), because the ECU does not calibrate them yet. When it sends 0.1 ° and 0.1 A, set `steeringScale` and `currentScale` in `frame.go` to `0.1`.
- `wheel_speed_rl/rr` are computed by the ECU from motor velocity / gear ratio, not measured.
- Fields the ECU has no source for yet (BMS, IMU, coolant, LV, tractive system, torque/current actual, …) arrive as 0 with their `valid_mask` bit off.

Each slow frame is published as its own packet (`"kind": "slow"`); the UI carries its values forward into every fast sample. The fast frame also publishes its header as `seq` and `ecu_time` (s). Channel keys for the driver blocks are `drv0_<field>` (left) and `drv1_<field>` (right); the currents keep the keys `dc_current_1/2` and `ac_current_1/2` (1 = left, 2 = right).

Runs recorded before v3 still open: the database stores decoded values, and a one-time migration rescaled their `driver_dc_voltage` from 0.1 V units to volts.

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
pip install pyserial
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

### Windows: desktop app

Double-click `telemetry-host.exe`. It opens in its own window (Microsoft Edge in app mode: no tabs or address bar; Chrome is used if Edge is missing) and there is no console. **Closing the window quits the program**; a recording in progress is stopped and saved. Opening the .exe again while it runs just opens another window.

- **Data:** `telemetry.db` lives **next to the .exe**, together with `telemetry-host.log`, wherever it was started from. To move the app to another PC, copy the .exe; copy `telemetry.db` too to keep the runs.
- **Requirements:** Windows 10 or 11, 64-bit. Edge comes with Windows, so nothing has to be installed.
- **Network:** it only listens on `127.0.0.1`, so Windows Firewall does not ask for permission.
- **Unsigned binary:** the first time, SmartScreen may show "Windows protected your PC"; use **More info → Run anyway**.

If the window cannot be opened, a message gives the address to open in any browser.

### Linux / macOS, or as a server

```bash
./telemetry-host
```

It logs the URL and opens it in the default browser; it runs until stopped (Ctrl+C). `--no-open` runs it as a plain server (used by Docker and `scripts/test-env.sh`), on Windows too.

```
telemetry host running at http://localhost:8080
```

### CLI flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8080` | HTTP server port |
| `--db` | `telemetry.db` (next to the .exe in the Windows app) | Path to the SQLite database file |
| `--no-open` | off | Don't open a window or browser; run as a server only |

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

## Built-in simulator

To try the app without the car, pick **SIMULATOR** in the port field (it is always listed last) and press **Connect**. It works in the Windows app and on any OS, with nothing else installed. The status bar shows a purple **SIMULATOR** tag, and runs recorded from it are named `[SIM] …`.

The simulator (`internal/serial/sim.go`) produces real v3 frames (fast at 10 Hz, slow at 1 Hz, with CRC and seq), which go through the same parser, link counters, calculated channels and event detection as data from the XBee. Unlike the real ECU today, it fills every field of the frame (BMS, tractive system, IMU, …) so every part of the UI can be tried. It plays a scripted 75 s scenario on a loop; times are the ECU's uptime:

| Time (s) | What happens |
|----------|--------------|
| 0–6 | Main ECU boot: CAN messages appear one by one in `valid_mask` (their fields are 0 until then); tractive system precharges (AIR−, precharge relay, AIR+); drivers go NMT boot-up → operational and CiA 402 switch on disabled → … → operation enabled; drive and ready-to-drive at 5.5 |
| 6–74 | 16 s laps: launch, full throttle, braking, corner, exit, coast. Launches spin the left rear wheel and traction control trims it; corner exits trim the right side; the race map falls off with speed, and a speed limit kicks in above 31 m/s |
| 15–19 | Driver R warning `0x0008` |
| 22–26 | TPS2 20 points below TPS1: host and ECU flag APPS implausibility, ECU safe mode until drive is toggled at 25.5 |
| 30–31.5 | Link lost: the ECU keeps sending, 15 fast and 2 slow frames are lost (seq gap) |
| 35 | Line noise, a frame with a wrong version and one with a bad CRC |
| 36–40 | CAN error counters climb: warning, error passive, bus-off at 39 |
| 38–46.5 | Driver L motor heats up: warning, derating at 40, fault `0x4310` at 42, fault reset and re-enable; a loop overrun at 40 |
| 50–53 | Driver R quick stop, then re-enabled |
| 55–58.5 | Driver R goes silent (its messages stop arriving), error `0x8130`, re-enabled |
| 56–59 | Cells at 58 °C: BMS fault flag and current limit down to 80 A |
| 58 / 60 | Calibration command 1 → 2; mode race → normal |
| 62–63 | Brake with the pedal at 50 %: EV.4.7 and safe mode; loop overruns at 61 |
| 64–65 | Rear sensor (0x504) timeout |
| 66–69 | Shutdown circuit opens at segment 3: AIRs open, link voltage decays, drivers disabled, precharge again |
| 69–73.5 | Driver R temperature derating, then low cell voltage derating |
| 73.5–75 | LV supply dips to 8.6 V and the ECU browns out: silence, then it boots again (seq and `ecu_time` restart, reset cause brown-out) |

`TestSimulatorScenario` checks that every channel changes and every bit and event above shows up. Which driver fields arrive in which PDO is an assumption (see `simFieldBits`).

The Python mock (`utils/mock_tx.py`, below) sends frames laid out exactly like the real ECU (unfilled fields at 0), to test the real serial path, e.g. through a pair of XBees.

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
2. Starts `mock_tx.py` on one end, transmitting v3 frames (fast at 10 Hz, slow at 1 Hz) with sine-wave data, laid out as the real main ECU sends them
3. Starts `./telemetry-host` and immediately connects it to the other end via the REST API

Set `MOCK_ARGS=--faults` to make the mock inject, every 20 s, one second of TPS implausibility and a two-second fault on driver L. Use it to try the **Health** tab and the event log: `MOCK_ARGS=--faults bash scripts/test-env.sh`, or `docker run -e MOCK_ARGS=--faults -p 8080:8080 telemetry-host` with Docker.

### Prerequisites check

The script will tell you if anything is missing before it starts:

| Missing | Error shown | Fix |
|---------|-------------|-----|
| Binary not built | `binary not found` | `go build -o telemetry-host ./cmd/app/` |
| socat not installed | `socat not found` | `sudo pacman -S socat` / `sudo apt install socat` |
| Python deps missing | `Python dependencies missing` | `pip install pyserial` |

### Port field note

The port dropdown auto-suggests real hardware serial ports (ttyUSB*, ttyACM*). Virtual PTY paths from socat won't appear there — that's normal. The script connects automatically. If you ever need to connect manually, type the path directly into the port field (e.g. `/dev/pts/5`).

> On Windows with a real USB-serial adapter, replace the port field with `COM3` (or whichever port appears in Device Manager). The script is Linux-only; on Windows just run the binary directly and enter the COM port manually.

---

## Running with Docker (Windows / macOS)

The test environment (socat + mock transmitter + host) needs Linux PTYs, so on Windows run it inside Docker Desktop. From the **repo root**:

```bash
docker compose up --build      # first time (builds frontend + binary)
docker compose up -d           # later runs, in the background
docker compose down            # stop
```

Open http://localhost:8080 — data from the mock transmitter flows in immediately. The database is persisted in `./data/telemetry.db`.

> Docker Desktop cannot access Windows COM ports. To use a real USB-serial adapter on Windows, run `telemetry-host.exe` natively. You can build it with Docker (no Go/Node needed), from the repo root:
>
> ```bash
> docker build --target windows-exe --output type=local,dest=app .
> ```
>
> This writes `app/telemetry-host.exe`.

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
3. While recording, press **M** (or click **⚑ Marker**) to drop a marker at the current moment, and click **✎ Run info** to fill in driver, mode, track, weather, setup and notes
4. Click **■ Stop** to end the recording

A new recording starts with the metadata of the last run you edited, since driver, mode and setup rarely change between consecutive runs. Recorded runs are stored in `telemetry.db` and appear in the **Runs** tab.

### Calculated channels

Besides the channels in the frame, the host calculates these. The same code (`frontend/src/lib/derived.js`) runs on live data and on saved runs, so they also work on runs recorded before they existed. They appear in the **Calculated** group in every channel picker.

| Channel | Unit | How |
|---------|------|-----|
| `tps_diff` | % | \|TPS1 − TPS2\| |
| `tps_implausible` | 0/1 | 1 while the difference exceeds 10 points for 100 ms or more (FSAE T.4.2) |
| `power_l`, `power_r`, `power_total` | kW | `driver_dc_voltage` × 0.1 V × `dc_current_1/2` |
| `energy` | Wh | Integral of `power_total`; regen counts as negative; gaps over 1 s are skipped |
| `frame_dt` | ms | Time since the previous frame (expected 100) |

> **To confirm:** the DC current scale (assumed 1 A/bit) and the current-to-driver mapping (assumed `dc_current_1` → left, `dc_current_2` → right). Both are constants at the top of `derived.js`.

### Health tab

Live diagnostics for testing sessions:

- **Link**: time since the last frame, frame rate against the expected 10 Hz, gaps over 0.5 s, and parser counters (rejected syncs, discarded bytes, frames dropped by the host)
- **Pedal plausibility**: TPS1/TPS2, their difference and the implausibilities this session
- **CAN messages**: `valid_mask` decoded into one light per message
- **Driver L / R**: CiA 402 state from `status_word`, NMT state from `heartbeat_state`, error code with its CANopen class, warning bits, torques, voltage and temperatures
- **Events**: log of driver state changes, errors and warnings, main ECU resets (bits dropping from `valid_mask`), TPS implausibility and link gaps

When an error event happens on another tab, a red counter appears on the **Health** tab.

### Analysing a run

Click a run in the **Runs** tab to open it with the dashboard layout. The list can be searched and filtered by driver and mode.

- **Cursor**: hovering a chart moves the cursor on every panel. The sidebar's **Values** tab lists every channel at that instant.
- **Zoom**: drag on any time-series panel to zoom; all panels follow. **↺ Reset** zooms back out.
- **Markers**: shown on every chart and on the scrubber. In the sidebar's **Markers** tab, click a time to jump there, edit the label, or delete it. **M** adds one at the cursor.
- **Events**: the same detector as the Health tab, run over the whole recording. Click one to jump there.
- **Info**: edit the run's metadata.
- **Keyboard**: **Space** play/pause, **←/→** one sample (with Shift, ten), **M** marker.
- **⤓ Export CSV**: whole run, current zoom or between two markers; all channels, only the car's, or the ones on the dashboard. Comma format for Python/MATLAB, or `;` with decimal comma for Excel in Spanish. Columns are `time_s` (since run start), `timestamp` (Unix seconds), `marker`, then one per channel.

### Comparing runs

Tick 2–4 runs in **Runs** and press **Compare**, or pick them in the **Compare** tab. Each selected channel gets a chart with one line per run. Runs can be aligned on their start, on a marker with the same label (e.g. "launch") or on the first time a channel reaches a value (e.g. `tps1 ≥ 50`). Zooming one chart zooms all of them. The table shows each run's values at the cursor, and min/avg/max over the zoomed range.

---

## Project structure

```
app/
├── cmd/app/
│   ├── main.go              # Entry point — HTTP server, go:embed
│   └── frontend/            # Compiled Svelte output (gitignored, built by npm)
├── internal/
│   ├── serial/
│   │   ├── frame.go         # v3 frame decoder (CRC, seq) + resync parser
│   │   ├── sim.go           # built-in SIMULATOR port
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
│       │   ├── channels.js    # Sensor names, groups, colors
│       │   ├── derived.js     # Calculated channels (TPS plausibility, power, energy…)
│       │   ├── diagnostics.js # valid_mask, CiA 402 and CANopen decoders
│       │   ├── events.js      # Event detection, live and on saved runs
│       │   ├── runs.js        # Run loading, markers and metadata API
│       │   ├── compare.js     # Run alignment for Compare
│       │   ├── csv.js         # CSV export
│       │   ├── dashboard.js   # Dashboard layout store (localStorage)
│       │   └── ws.js          # WebSocket client + ring buffer
│       └── routes/
│           ├── Dashboard.svelte
│           ├── Health.svelte
│           ├── Runs.svelte
│           ├── RunDashboard.svelte
│           └── Compare.svelte
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
| `POST` | `/api/runs` | Start recording: `{"name", "driver", "mode", "track", "weather", "setup", "notes"}`, all optional |
| `POST` | `/api/runs/{id}/stop` | Stop recording |
| `GET` | `/api/runs/{id}` | Get run metadata (includes the marker count) |
| `PATCH` | `/api/runs/{id}` | Update metadata: any subset of the fields above |
| `GET` | `/api/runs/{id}/samples` | Get all samples for a run (JSON array) |
| `DELETE` | `/api/runs/{id}` | Delete a run with its samples and markers |
| `GET` | `/api/runs/{id}/markers` | List markers, sorted by time |
| `POST` | `/api/runs/{id}/markers` | Add a marker: `{"label": "…", "ts": 1714123456.7}`; `ts` defaults to now |
| `PATCH` | `/api/runs/{id}/markers/{markerID}` | Change a marker's `label` and/or `ts` |
| `DELETE` | `/api/runs/{id}/markers/{markerID}` | Delete a marker |
| `WS` | `/ws` | WebSocket — streams live packets, status and link updates |

### WebSocket message format

**Incoming packet** (from car):
```json
{ "type": "packet", "ts": 1714123456.789, "data": { "tps1": 512, "steering_angle": -120, "drv0_motor_temp": 61, ... } }
```

**Status update** (on connect/disconnect/record):
```json
{ "type": "status", "data": { "connected": true, "port": "/dev/pts/2", "recording": false, "active_run_id": 0 } }
```

**Link counters** (every second; counters reset on connect):
```json
{ "type": "link", "data": { "connected": true, "rate_hz": 10, "frames": 1234, "discarded_bytes": 0, "rejected": 0, "dropped": 0 } }
```
