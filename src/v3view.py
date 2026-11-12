#!/usr/bin/env python3
"""Live viewer for main-ECU telemetry frames v3 (port of app/internal/serial/frame.go).

Shows the latest raw frame (hex, header/payload/CRC coloured) and every decoded
field in physical units, plus link-quality counters. Redraws in place.
"""
import argparse
import shutil
import struct
import sys
import time

import serial

from v3frame import HDR, LAYOUTS, TYPE_FAST, TYPE_SLOW, Parser, decode_payload, fmt_val

# ── rendering ────────────────────────────────────────────────────────────────
DIM, RST, BOLD = "\x1b[2m", "\x1b[0m", "\x1b[1m"
C_SYNC, C_HDR, C_PAY, C_CRC, C_ZERO = "\x1b[35m", "\x1b[36m", "\x1b[37m", "\x1b[33m", "\x1b[90m"


def hexdump(frame):
    lines = []
    plen = frame[5]
    for off in range(0, len(frame), 16):
        cells = []
        for j, b in enumerate(frame[off:off + 16], off):
            c = (C_SYNC if j < 2 else C_HDR if j < HDR else
                 C_CRC if j >= HDR + plen else C_ZERO if b == 0 else C_PAY)
            cells.append(f"{c}{b:02X}{RST}")
        lines.append(f"{DIM}{off:04X}{RST}  " + " ".join(cells))
    return lines


def columns(items, width):
    cells = [f"{k:<26}{v:>12}" for k, v in items]
    cw = 40
    ncol = max(1, width // cw)
    rows = (len(cells) + ncol - 1) // ncol
    return ["".join(f"{cells[c * rows + r]:<{cw}}" for c in range(ncol) if c * rows + r < len(cells))
            for r in range(rows)]


def render(state, p, port, baud, t0):
    w = shutil.get_terminal_size((120, 40)).columns
    o = ["\x1b[H"]
    el = time.monotonic() - t0
    o.append(f"{BOLD}Telemetry v3 · {port} @ {baud}{RST}   up {el:6.1f}s   "
             f"fast {state['n'][1]} ({state['n'][1] / max(el, 1e-3):.1f} Hz)  slow {state['n'][2]}   "
             f"crc_err {p.crc_errors}  lost {p.lost}  rejected {p.rejected}  discarded {p.discarded}B")
    o.append(f"{DIM}legend: {C_SYNC}sync{RST}{DIM} {C_HDR}header{RST}{DIM} {C_PAY}payload{RST}{DIM} "
             f"{C_ZERO}00{RST}{DIM} {C_CRC}crc{RST}")
    for typ, name in ((TYPE_FAST, "FAST"), (TYPE_SLOW, "SLOW")):
        f = state["last"].get(typ)
        o.append("")
        if not f:
            o.append(f"{BOLD}── {name} frame ──{RST} {DIM}(none yet){RST}")
            continue
        seq, t_ms, frame, dec = f
        o.append(f"{BOLD}── {name} frame ──{RST} seq {seq:3d}  ecu_time {t_ms / 1000:.3f}s  "
                 f"len {len(frame)}  crc 0x{struct.unpack_from('<H', frame, len(frame) - 2)[0]:04X} ✓")
        o.append(f"{BOLD}raw{RST}")
        o += hexdump(frame)
        o.append(f"{BOLD}decoded{RST}")
        o += columns([(k, fmt_val(k, r, v, s)) for k, (r, v, s) in dec.items()], w)
    sys.stdout.write("\x1b[K\n".join(o) + "\x1b[K\x1b[J")
    sys.stdout.flush()


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("-p", "--port", default="/dev/ttyUSB0")
    ap.add_argument("-b", "--baud", type=int, default=115200)
    ap.add_argument("--fps", type=float, default=15)
    a = ap.parse_args()

    p = Parser()
    state = {"last": {}, "n": {1: 0, 2: 0}}
    sys.stdout.write("\x1b[?25l\x1b[2J")
    try:
        with serial.Serial(a.port, a.baud, timeout=0.05) as ser:
            t0 = last_draw = time.monotonic()
            while True:
                data = ser.read(ser.in_waiting or 1)
                for typ, seq, t_ms, frame in p.feed(data) if data else []:
                    if typ in LAYOUTS:
                        state["n"][typ] += 1
                        state["last"][typ] = (seq, t_ms, frame,
                                              decode_payload(typ, frame[HDR:-2]))
                now = time.monotonic()
                if now - last_draw >= 1 / a.fps:
                    render(state, p, a.port, a.baud, t0)
                    last_draw = now
    except serial.SerialException as e:
        sys.stdout.write(f"\x1b[?25h\nserial error: {e}\n")
        input("press Enter to close")
    except KeyboardInterrupt:
        pass
    finally:
        sys.stdout.write("\x1b[?25h\n")


if __name__ == "__main__":
    main()
