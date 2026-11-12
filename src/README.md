# Receiver Scripts

Install dependencies (from the repo root):

```sh
python -m venv .venv && .venv/bin/pip install -r requirements.txt
```

## `v3view.py` — main-ECU telemetry v3 viewer
Live terminal view of the main ECU's v3 frames (`AA 55` sync, 10-byte header,
payload, CRC-16/CCITT-FALSE). Layouts mirror `app/internal/serial/frame.go`.
Redraws in place showing link stats, the raw hex of the latest fast and slow
frames (colour-coded sync/header/payload/CRC) and every decoded field in
physical units.
* *Options:*
    * `-p, --port`: Serial port to listen on (default: `/dev/ttyUSB0`)
    * `-b, --baud`: Baud rate (default: `115200`)
    * `--fps`: Screen refresh rate (default: `15`)

Open it in its own kitty window:

```sh
kitty --title "telemetry v3" .venv/bin/python src/v3view.py -p /dev/ttyUSB0
```

If the port gives `Permission denied`, add yourself to the device's group
(`uucp` on Arch, `dialout` on Debian/Ubuntu) and log in again, or grant access
until the adapter is unplugged: `sudo setfacl -m u:<user>:rw /dev/ttyUSB0`.

## `rx.py` — telemetry v3 receiver (scrolling log)
Same decoder as `v3view.py` (shared in `v3frame.py`), as a plain log. Every
5 s it logs link stats (frames per type, CRC errors, lost frames, discarded bytes).
* *Options:*
    * `-p, --port`: Serial port to listen on (default: `/dev/ttyUSB0`)
    * `-b, --baud`: Baud rate (default: `115200`)
    * `-v, --verbose`: Print every decoded field of each frame
    * `-d, --debug`: Enable debug logging (raw frame hex, CRC drops, resyncs, sequence gaps)

## `v3frame.py`
Shared frame layouts, CRC-16 and resyncing stream parser. A port of
`app/internal/serial/frame.go`; when the frame spec changes, update both.
