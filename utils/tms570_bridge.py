#!/usr/bin/env python3
"""
Bridge the TMS570 Main ECU's real telemetry stream into telemetry-host.

The Main ECU firmware (tms570_freertos_v2) sends a fixed-size 70-byte
binary frame over UART -- not the COBS/MessagePack protocol the telemetry
host's Go backend understands. This script sits in between: it reads and
decodes the ECU's frames, then re-encodes the fields telemetry-host cares
about as COBS-framed MessagePack dictionaries, same as utils/mock_tx.py
does for the synthetic test data.

    TMS570 board (real UART frames)
      --in-port-->  this bridge  --out-port-->  telemetry-host (COBS/msgpack)

--out-port is normally one end of a socat virtual serial pair, with
telemetry-host connected to the other end -- see scripts/real-hw-env.sh,
which wires this up automatically. --in-port is the board's actual UART
adapter (e.g. /dev/ttyUSB0).

Frame format (little-endian, 70 bytes) -- must be kept in lockstep with
vPackFrame() in tms570_freertos_v2/app/main.c (and mirrored in that
project's tools/decode_telemetry.py):

    offset  size  field
    0       2     sync (0xAA 0x55)
    2       1     version (2)
    3       2     valid_mask (bit set per message received at least once)
    5       2     tps1
    7       2     tps2
    9       2     wheel_speed_fl
    11      2     wheel_speed_fr
    13      2     steering_angle
    15      2     front_brake
    17      2     ac_current_1
    19      2     ac_current_2
    21      2     dc_current_1
    23      2     dc_current_2
    25      2     rear_brake
    27      1     calibration_type
    28      21    driver[0] (left):  control_word(H) target_torque(h)
                  status_word(H) warnings(H) error(H) motor_velocity(H)
                  motor_rated_current(I) motor_temp(B) driver_temp(B)
                  driver_dc_voltage(H) heartbeat_state(B)
    49      21    driver[1] (right), same layout as driver[0]
    70      --    end

Only fields whose source message has actually been received (per
valid_mask) are included in the outgoing packet, so the dashboard shows
gaps rather than misleading zeros for anything not yet on the bus.

Usage:
    python3 tms570_bridge.py -i /dev/ttyUSB0 -o /dev/pts/7
"""
import sys
import os
import struct
import argparse
import logging
import termios

import serial
import msgpack
from cobs import cobs

logger = logging.getLogger(__name__)

FRAME_SIZE = 70
_NODE_FMT = "HhHHHHIBBHB"  # 21 bytes: see driver[] layout above
STRUCT_FMT = "<BBBH" + "HH" + "HHHH" + "HHHH" + "H" + "B" + (_NODE_FMT * 2)
assert struct.calcsize(STRUCT_FMT) == FRAME_SIZE

VALID_TPS, VALID_FRONT, VALID_CURRENT, VALID_REAR, VALID_CAL = (1 << i for i in range(5))


def valid_rxpdo1(n):    return 1 << (5 + 5 * n)
def valid_txpdo1(n):    return 1 << (6 + 5 * n)
def valid_txpdo2(n):    return 1 << (7 + 5 * n)
def valid_txpdo3(n):    return 1 << (8 + 5 * n)
def valid_heartbeat(n): return 1 << (9 + 5 * n)


def open_in_port(port, baud):
    """Opens the real board's UART. Plain termios, not pyserial: pyserial's
    high-level read() loop plus our own byte-at-a-time resync below is more
    fiddly to get right than just doing it directly."""
    fd = os.open(port, os.O_RDWR | os.O_NOCTTY)
    attrs = termios.tcgetattr(fd)
    baud_const = getattr(termios, f"B{baud}")
    attrs[4] = baud_const
    attrs[5] = baud_const
    attrs[0] = 0
    attrs[1] = 0
    attrs[2] = termios.CS8 | termios.CREAD | termios.CLOCAL
    attrs[3] = 0
    attrs[6][termios.VMIN] = 1
    attrs[6][termios.VTIME] = 0
    termios.tcsetattr(fd, termios.TCSANOW, attrs)
    return fd


def read_exact(fd, n):
    buf = b""
    while len(buf) < n:
        chunk = os.read(fd, n - len(buf))
        if not chunk:
            raise EOFError("board serial port closed")
        buf += chunk
    return buf


def find_sync(fd):
    prev = None
    while True:
        b = read_exact(fd, 1)[0]
        if prev == 0xAA and b == 0x55:
            return
        prev = b


def read_frame(fd):
    """Blocks until one valid (sync + version matches) frame is decoded."""
    while True:
        find_sync(fd)
        rest = read_exact(fd, FRAME_SIZE - 2)
        fields = struct.unpack(STRUCT_FMT, b"\xaa\x55" + rest)
        if fields[2] == 2:  # version
            return fields
        # else: false sync match inside frame data -- loop and resync


def to_channels(fields):
    """Maps a decoded frame to the dict telemetry-host will display,
    including only fields whose source message has actually been seen."""
    mask = fields[3]
    tps1, tps2 = fields[4:6]
    wsfl, wsfr, steer, fbrake = fields[6:10]
    ac1, ac2, dc1, dc2 = fields[10:14]
    (rbrake,) = fields[14:15]
    nodes = (fields[16:27], fields[27:38])

    out = {}
    if mask & VALID_TPS:
        out["tps1"] = tps1
        out["tps2"] = tps2
    if mask & VALID_FRONT:
        out["wheel_speed_fl"] = wsfl
        out["wheel_speed_fr"] = wsfr
        out["steering_angle"] = steer
        out["front_brake"] = fbrake
    if mask & VALID_CURRENT:
        out["ac_current_1"] = ac1
        out["ac_current_2"] = ac2
        out["dc_current_1"] = dc1
        out["dc_current_2"] = dc2
    if mask & VALID_REAR:
        out["rear_brake"] = rbrake

    for n, node in enumerate(nodes):
        (control_word, target_torque, status_word, warnings, error,
         motor_velocity, motor_rated_current, motor_temp, driver_temp,
         driver_dc_voltage, heartbeat_state) = node
        p = f"drv{n + 1}_"
        if mask & valid_rxpdo1(n):
            out[p + "control_word"] = control_word
            out[p + "target_torque"] = target_torque
        if mask & valid_txpdo1(n):
            out[p + "status_word"] = status_word
            out[p + "warnings"] = warnings
            out[p + "error"] = error
        if mask & valid_txpdo2(n):
            out[p + "motor_velocity"] = motor_velocity
            out[p + "motor_rated_current"] = motor_rated_current
            out[p + "motor_temp"] = motor_temp
        if mask & valid_txpdo3(n):
            out[p + "driver_temp"] = driver_temp
            out[p + "driver_dc_voltage"] = driver_dc_voltage
        if mask & valid_heartbeat(n):
            out[p + "heartbeat_state"] = heartbeat_state

    return out


def send_packet(ser, data_dict):
    msgpack_payload = msgpack.packb(data_dict, use_bin_type=True)
    frame = cobs.encode(msgpack_payload) + b"\x00"
    ser.write(frame)
    if logger.isEnabledFor(logging.DEBUG):
        hex_dump = " ".join(f"{b:02X}" for b in frame)
        logger.debug(f"Forwarded raw frame: {hex_dump}")


def main():
    parser = argparse.ArgumentParser(
        description="Bridge real TMS570 telemetry frames into telemetry-host's COBS/MessagePack protocol")
    parser.add_argument("-i", "--in-port", type=str, default="/dev/ttyUSB0",
                         help="TMS570 board's serial port (default: /dev/ttyUSB0)")
    parser.add_argument("-b", "--in-baud", type=int, default=115200,
                         help="Board baud rate (default: 115200)")
    parser.add_argument("-o", "--out-port", type=str, required=True,
                         help="Port telemetry-host is connected to (e.g. one end of a socat pty pair)")
    parser.add_argument("--out-baud", type=int, default=115200,
                         help="Baud rate for --out-port (default: 115200)")
    parser.add_argument("-v", "--verbose", action="store_true",
                         help="Print each forwarded packet")
    parser.add_argument("-d", "--debug", action="store_true",
                         help="Enable debug logging (raw hex frames)")
    args = parser.parse_args()

    logging.basicConfig(
        level=logging.DEBUG if args.debug else logging.INFO,
        format="%(asctime)s [%(levelname)s] %(message)s",
        datefmt="%H:%M:%S",
    )

    logger.info(f"Reading TMS570 frames from {args.in_port} @ {args.in_baud} baud...")
    logger.info(f"Forwarding to {args.out_port} @ {args.out_baud} baud (telemetry-host protocol)...")

    in_fd = open_in_port(args.in_port, args.in_baud)
    out_ser = serial.Serial(args.out_port, args.out_baud, timeout=1)

    frame_num = 0
    try:
        while True:
            fields = read_frame(in_fd)
            data = to_channels(fields)
            send_packet(out_ser, data)

            if args.verbose:
                print(f"--- frame {frame_num} ---  {data}")
            frame_num += 1
    except KeyboardInterrupt:
        pass
    except EOFError as e:
        logger.error(str(e))
    finally:
        os.close(in_fd)
        out_ser.close()


if __name__ == "__main__":
    main()
