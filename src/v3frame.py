"""Telemetry frames v3 from the main ECU: layouts, CRC and a resyncing stream parser.

Port of app/internal/serial/frame.go -- keep the two in sync. A frame is a
10-byte header (sync AA 55, version 3, type, seq, payload_len, ecu_time_ms),
the payload and a CRC-16/CCITT-FALSE over bytes 2 .. end of payload. Type 1 is
the fast frame (130 B), type 2 the slow one (60 B). All little-endian.
"""
import struct

SYNC = b"\xaa\x55"
HDR = 10
TYPE_FAST, TYPE_SLOW = 1, 2
EXPECTED_PLEN = {TYPE_FAST: 130 - HDR - 2, TYPE_SLOW: 60 - HDR - 2}

FMT = {"u8": "B", "i8": "b", "u16": "H", "i16": "h", "u32": "I", "i32": "i"}

# (key, kind, scale) -- key None = reserved. Matches frame.go v3FastLayout.
FAST_COMMON = [
    ("valid_mask", "u32", 1), ("fresh_mask", "u32", 1), ("ecu_state", "u16", 1),
    ("torque_limit_reason", "u16", 1), ("ts_state", "u16", 1), ("calibration_type", "u8", 1),
    ("sdc_open_segment", "u8", 1), ("tps1", "u16", .1), ("tps2", "u16", .1),
    ("front_brake", "u16", 1), ("rear_brake", "u16", 1), ("steering_angle", "i16", 1),
    ("wheel_speed_fl", "u16", .1), ("wheel_speed_fr", "u16", .1),
    ("wheel_speed_rl", "u16", .1), ("wheel_speed_rr", "u16", .1),
    ("torque_request", "i16", 1), ("torque_cmd_l", "i16", 1), ("torque_cmd_r", "i16", 1),
    ("pack_voltage", "u16", .1), ("pack_current", "i16", .1), ("soc", "u8", 1),
    ("cell_temp_max", "i8", 1), ("cell_v_min", "u16", .001), ("cell_v_max", "u16", .001),
    ("bms_current_limit", "u16", .1), ("bms_fault", "u16", 1),
    ("dc_current_1", "i16", 1), ("dc_current_2", "i16", 1),
    ("ac_current_1", "i16", 1), ("ac_current_2", "i16", 1),
    ("accel_x", "i16", .001), ("accel_y", "i16", .001), ("yaw_rate", "i16", .01),
    ("coolant_temp", "i8", 1), (None, "u8", 1), ("lv_voltage", "u16", .01),
]
FAST_DRIVER = [
    ("control_word", "u16", 1), ("status_word", "u16", 1), ("warnings", "u16", 1),
    ("error", "u16", 1), ("target_torque", "i16", 1), ("torque_actual", "i16", 1),
    ("current_actual", "i16", 1), ("motor_velocity", "i32", 1),
    ("driver_dc_voltage", "u16", .1), ("driver_temp", "u8", 1), ("motor_temp", "u8", 1),
    ("heartbeat_state", "u8", 1),
]
FAST = FAST_COMMON + [(f"drv{d}_{k}", t, s) for d in (0, 1) for k, t, s in FAST_DRIVER]
SLOW = [
    ("fw_major", "u8", 1), ("fw_minor", "u8", 1), ("fw_patch", "u8", 1), ("fw_dirty", "u8", 1),
    ("git_hash", "u32", 1), ("uptime_s", "u32", 1), ("reset_count", "u16", 1),
    ("last_reset_cause", "u8", 1), ("cpu_load", "u8", 1), ("loop_overruns", "u16", 1),
    ("loop_time_max", "u16", 1), ("active_faults", "u32", 1), ("can_tx_err", "u8", 1),
    ("can_rx_err", "u8", 1), ("can_busoff_count", "u16", 1),
    ("drv0_motor_rated_current", "u32", 1), ("drv1_motor_rated_current", "u32", 1),
    ("drv0_motor_rated_torque", "u32", .001), ("drv1_motor_rated_torque", "u32", .001),
    ("cell_v_min_id", "u8", 1), ("cell_v_max_id", "u8", 1), ("cell_temp_max_id", "u8", 1),
    (None, "u8", 1),
]
HEX_KEYS = {"valid_mask", "fresh_mask", "bms_fault", "control_word", "status_word",
            "warnings", "error", "git_hash", "active_faults"}


def compile_layout(fields):
    st = struct.Struct("<" + "".join(FMT[t] for _, t, _ in fields))
    return st, fields


LAYOUTS = {TYPE_FAST: compile_layout(FAST), TYPE_SLOW: compile_layout(SLOW)}
assert LAYOUTS[TYPE_FAST][0].size == EXPECTED_PLEN[TYPE_FAST]
assert LAYOUTS[TYPE_SLOW][0].size == EXPECTED_PLEN[TYPE_SLOW]


def crc16(b):  # CRC-16/CCITT-FALSE
    crc = 0xFFFF
    for x in b:
        crc ^= x << 8
        for _ in range(8):
            crc = ((crc << 1) ^ 0x1021) & 0xFFFF if crc & 0x8000 else (crc << 1) & 0xFFFF
    return crc


def decode_payload(typ, payload):
    st, fields = LAYOUTS[typ]
    out = {}
    for (key, kind, scale), raw in zip(fields, st.unpack(payload)):
        if key:
            out[key] = (raw, raw * scale, scale)
    return out


class Parser:
    """Resyncs on AA 55 like frameParser.feed in frame.go."""

    def __init__(self):
        self.buf = bytearray()
        self.discarded = self.rejected = self.crc_errors = self.lost = 0
        self.last = {}

    def _reject(self):
        self.rejected += 1
        self.discarded += 1
        del self.buf[0]

    def feed(self, data):
        self.buf += data
        out = []
        while True:
            i = self.buf.find(SYNC)
            if i < 0:
                keep = 1 if self.buf[-1:] == b"\xaa" else 0
                self.discarded += len(self.buf) - keep
                del self.buf[:len(self.buf) - keep]
                return out
            self.discarded += i
            del self.buf[:i]
            if len(self.buf) < 3:
                return out
            if self.buf[2] != 3:
                self._reject(); continue
            if len(self.buf) < 6:
                return out
            typ, seq, plen = self.buf[3], self.buf[4], self.buf[5]
            if typ in EXPECTED_PLEN and plen != EXPECTED_PLEN[typ]:
                self._reject(); continue
            n = HDR + plen + 2
            if len(self.buf) < n:
                return out
            frame = bytes(self.buf[:n])
            if crc16(frame[2:HDR + plen]) != struct.unpack_from("<H", frame, HDR + plen)[0]:
                self.crc_errors += 1
                self._reject(); continue
            del self.buf[:n]
            t_ms = struct.unpack_from("<I", frame, 6)[0]
            if typ in self.last:
                pseq, pt = self.last[typ]
                if t_ms >= pt:
                    self.lost += (seq - pseq - 1) & 0xFF
            self.last[typ] = (seq, t_ms)
            out.append((typ, seq, t_ms, frame))


def fmt_val(key, raw, val, scale):
    """Format a decoded field: hex for masks/status words, scale-appropriate decimals otherwise."""
    if key in HEX_KEYS:
        return f"0x{raw:0{8 if raw > 0xFFFF else 4}X}"
    if scale == 1:
        return str(raw)
    digits = len(f"{scale:g}".split(".")[1]) if "." in f"{scale:g}" else 0
    return f"{val:.{digits}f}"
