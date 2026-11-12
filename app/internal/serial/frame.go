package serial

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

// Telemetry frames from the main ECU, v3 (tms570_main_ecu, app/telemetry.c;
// spec "Frame de telemetría v3"): a 10-byte header (sync 0xAA 0x55, version
// 3, type, seq, payload_len, ecu_time_ms), the payload, and a CRC-16/
// CCITT-FALSE over bytes 2 … end of payload. Type 1 is the fast frame
// (130 B, 10 Hz), type 2 the slow one (60 B, 1 Hz).
//
// All fields are little-endian. Decoded values are in physical units: each
// field's scale is applied (0.1 % → %, 0.1 V → V, …).
const (
	frameSync0 = 0xAA
	frameSync1 = 0x55

	v3HeaderLen = 10
	v3TypeFast  = 1
	v3TypeSlow  = 2
	v3FastLen   = 130
	v3SlowLen   = 60
)

// The main ECU still sends steering and the four currents uncalibrated
// (raw 0x502 / 0x503 values, see telemetry.c), so they are passed through
// as is. Set these to the spec's 0.1 once it sends degrees and amperes.
const (
	steeringScale = 1 // spec: 0.1 °/bit
	currentScale  = 1 // spec: 0.1 A/bit
)

type fieldKind int

const (
	u8 fieldKind = iota
	i8
	u16
	i16
	u32
	i32
)

func (k fieldKind) size() int {
	switch k {
	case u8, i8:
		return 1
	case u16, i16:
		return 2
	default:
		return 4
	}
}

func (k fieldKind) read(b []byte) float64 {
	switch k {
	case u8:
		return float64(b[0])
	case i8:
		return float64(int8(b[0]))
	case u16:
		return float64(binary.LittleEndian.Uint16(b))
	case i16:
		return float64(int16(binary.LittleEndian.Uint16(b)))
	case u32:
		return float64(binary.LittleEndian.Uint32(b))
	default:
		return float64(int32(binary.LittleEndian.Uint32(b)))
	}
}

// write stores v rounded and clamped to the range of the field type.
func (k fieldKind) write(b []byte, v float64) {
	clamp := func(lo, hi float64) float64 { return math.Max(lo, math.Min(hi, math.Round(v))) }
	switch k {
	case u8:
		b[0] = byte(clamp(0, math.MaxUint8))
	case i8:
		b[0] = byte(int8(clamp(math.MinInt8, math.MaxInt8)))
	case u16:
		binary.LittleEndian.PutUint16(b, uint16(clamp(0, math.MaxUint16)))
	case i16:
		binary.LittleEndian.PutUint16(b, uint16(int16(clamp(math.MinInt16, math.MaxInt16))))
	case u32:
		binary.LittleEndian.PutUint32(b, uint32(clamp(0, math.MaxUint32)))
	default:
		binary.LittleEndian.PutUint32(b, uint32(int32(clamp(math.MinInt32, math.MaxInt32))))
	}
}

// field is one wire field. An empty key marks a reserved field (skipped);
// scale converts the wire integer to physical units (0 means 1).
type field struct {
	key   string
	kind  fieldKind
	scale float64
}

func (f field) mul() float64 {
	if f.scale == 0 {
		return 1
	}
	return f.scale
}

// section is a run of fields; a driver section repeats once per driver with
// the keys prefixed drv0_ / drv1_.
type section struct {
	fields  []field
	drivers bool
}

type layout []section

func (l layout) size() int {
	n := 0
	for _, s := range l {
		reps := 1
		if s.drivers {
			reps = 2
		}
		for _, f := range s.fields {
			n += reps * f.kind.size()
		}
	}
	return n
}

// each calls fn for every field in wire order with its full key and offset
// relative to the start of the layout.
func (l layout) each(fn func(key string, f field, off int)) {
	off := 0
	for _, s := range l {
		reps := 1
		if s.drivers {
			reps = 2
		}
		for d := 0; d < reps; d++ {
			prefix := ""
			if s.drivers {
				prefix = fmt.Sprintf("drv%d_", d)
			}
			for _, f := range s.fields {
				key := ""
				if f.key != "" {
					key = prefix + f.key
				}
				fn(key, f, off)
				off += f.kind.size()
			}
		}
	}
}

func (l layout) decode(b []byte, data map[string]float64) {
	l.each(func(key string, f field, off int) {
		if key != "" {
			data[key] = f.kind.read(b[off:]) * f.mul()
		}
	})
}

func (l layout) encode(b []byte, data map[string]float64) {
	l.each(func(key string, f field, off int) {
		if key != "" {
			f.kind.write(b[off:], data[key]/f.mul())
		}
	})
}

// v3FastLayout is the fast frame payload (offsets 10–127).
var v3FastLayout = layout{
	{fields: []field{
		{"valid_mask", u32, 0},
		{"fresh_mask", u32, 0},
		{"ecu_state", u16, 0},
		{"torque_limit_reason", u16, 0},
		{"ts_state", u16, 0},
		{"calibration_type", u8, 0},
		{"sdc_open_segment", u8, 0},
		{"tps1", u16, 0.1},
		{"tps2", u16, 0.1},
		{"front_brake", u16, 0},
		{"rear_brake", u16, 0},
		{"steering_angle", i16, steeringScale},
		{"wheel_speed_fl", u16, 0.1},
		{"wheel_speed_fr", u16, 0.1},
		{"wheel_speed_rl", u16, 0.1},
		{"wheel_speed_rr", u16, 0.1},
		{"torque_request", i16, 0},
		{"torque_cmd_l", i16, 0},
		{"torque_cmd_r", i16, 0},
		{"pack_voltage", u16, 0.1},
		{"pack_current", i16, 0.1},
		{"soc", u8, 0},
		{"cell_temp_max", i8, 0},
		{"cell_v_min", u16, 0.001},
		{"cell_v_max", u16, 0.001},
		{"bms_current_limit", u16, 0.1},
		{"bms_fault", u16, 0},
		// keys as in older runs: 1 = left, 2 = right
		{"dc_current_1", i16, currentScale},
		{"dc_current_2", i16, currentScale},
		{"ac_current_1", i16, currentScale},
		{"ac_current_2", i16, currentScale},
		{"accel_x", i16, 0.001},
		{"accel_y", i16, 0.001},
		{"yaw_rate", i16, 0.01},
		{"coolant_temp", i8, 0},
		{"", u8, 0}, // reserved
		{"lv_voltage", u16, 0.01},
	}},
	{drivers: true, fields: []field{
		{"control_word", u16, 0},
		{"status_word", u16, 0},
		{"warnings", u16, 0},
		{"error", u16, 0},
		{"target_torque", i16, 0},
		{"torque_actual", i16, 0},
		{"current_actual", i16, 0},
		{"motor_velocity", i32, 0},
		{"driver_dc_voltage", u16, 0.1},
		{"driver_temp", u8, 0},
		{"motor_temp", u8, 0},
		{"heartbeat_state", u8, 0},
	}},
}

// v3SlowLayout is the slow frame payload (offsets 10–57).
var v3SlowLayout = layout{
	{fields: []field{
		{"fw_major", u8, 0},
		{"fw_minor", u8, 0},
		{"fw_patch", u8, 0},
		{"fw_dirty", u8, 0},
		{"git_hash", u32, 0},
		{"uptime_s", u32, 0},
		{"reset_count", u16, 0},
		{"last_reset_cause", u8, 0},
		{"cpu_load", u8, 0},
		{"loop_overruns", u16, 0},
		{"loop_time_max", u16, 0},
		{"active_faults", u32, 0},
		{"can_tx_err", u8, 0},
		{"can_rx_err", u8, 0},
		{"can_busoff_count", u16, 0},
		{"drv0_motor_rated_current", u32, 0},
		{"drv1_motor_rated_current", u32, 0},
		{"drv0_motor_rated_torque", u32, 0.001},
		{"drv1_motor_rated_torque", u32, 0.001},
		{"cell_v_min_id", u8, 0},
		{"cell_v_max_id", u8, 0},
		{"cell_temp_max_id", u8, 0},
		{"", u8, 0}, // reserved
	}},
}

// v3Header is the decoded common header of a v3 frame.
type v3Header struct {
	typ, seq   byte
	payloadLen int
	ecuTimeMs  uint32
}

func readV3Header(f []byte) v3Header {
	return v3Header{
		typ:        f[3],
		seq:        f[4],
		payloadLen: int(f[5]),
		ecuTimeMs:  binary.LittleEndian.Uint32(f[6:]),
	}
}

// v3ExpectedLen is the payload length of each known type; the parser rejects
// a known type with any other length without waiting for its bytes.
var v3ExpectedLen = map[byte]int{
	v3TypeFast: v3FastLen - v3HeaderLen - 2,
	v3TypeSlow: v3SlowLen - v3HeaderLen - 2,
}

// crc16 is CRC-16/CCITT-FALSE: poly 0x1021, init 0xFFFF, no reflection.
func crc16(b []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, x := range b {
		crc ^= uint16(x) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// errCRC marks a v3 frame whose CRC does not match.
var errCRC = fmt.Errorf("CRC mismatch")

// decodeV3 checks the CRC of one complete v3 frame and decodes it. Frames of
// an unknown type decode to a nil map. The fast frame also carries its
// header as the channels seq and ecu_time (s).
func decodeV3(f []byte) (frameData, error) {
	if len(f) < v3HeaderLen+2 || f[0] != frameSync0 || f[1] != frameSync1 || f[2] != 3 {
		return frameData{}, fmt.Errorf("not a v3 frame")
	}
	h := readV3Header(f)
	if len(f) != v3HeaderLen+h.payloadLen+2 {
		return frameData{}, fmt.Errorf("frame length %d, payload_len %d", len(f), h.payloadLen)
	}
	end := v3HeaderLen + h.payloadLen
	if crc16(f[2:end]) != binary.LittleEndian.Uint16(f[end:]) {
		return frameData{}, errCRC
	}

	payload := f[v3HeaderLen:end]
	fd := frameData{typ: h.typ, seq: h.seq, ecuTimeMs: h.ecuTimeMs}
	switch {
	case h.typ == v3TypeFast && h.payloadLen == v3FastLayout.size():
		fd.kind = KindFast
		fd.data = make(map[string]float64, 80)
		v3FastLayout.decode(payload, fd.data)
		fd.data["seq"] = float64(h.seq)
		fd.data["ecu_time"] = float64(h.ecuTimeMs) / 1000
	case h.typ == v3TypeSlow && h.payloadLen == v3SlowLayout.size():
		fd.kind = KindSlow
		fd.data = make(map[string]float64, 30)
		v3SlowLayout.decode(payload, fd.data)
	}
	return fd, nil
}

// encodeV3 builds a v3 frame of the given type from channel values.
func encodeV3(typ, seq byte, ecuTimeMs uint32, data map[string]float64) []byte {
	l := v3FastLayout
	if typ == v3TypeSlow {
		l = v3SlowLayout
	}
	n := l.size()
	f := make([]byte, v3HeaderLen+n+2)
	f[0], f[1], f[2], f[3], f[4], f[5] = frameSync0, frameSync1, 3, typ, seq, byte(n)
	binary.LittleEndian.PutUint32(f[6:], ecuTimeMs)
	l.encode(f[v3HeaderLen:], data)
	binary.LittleEndian.PutUint16(f[v3HeaderLen+n:], crc16(f[2:v3HeaderLen+n]))
	return f
}

// ── stream parser ───────────────────────────────────────────────────────────

// Frame kinds, as published in Packet.Kind.
const (
	KindFast = "fast"
	KindSlow = "slow"
)

// frameData is one decoded frame.
type frameData struct {
	kind      string // "" for a frame of an unknown type
	data      map[string]float64
	typ, seq  byte
	ecuTimeMs uint32
}

// frameParser accumulates a byte stream and extracts v3 frames,
// resynchronising on the 0xAA 0x55 sync after garbage or partial frames.
type frameParser struct {
	buf []byte

	// Link-quality counters, cumulative since the parser was created.
	discarded int // bytes thrown away while hunting for a sync
	rejected  int // syncs that did not lead to a valid frame
	crcErrors int // v3 frames with a bad CRC (also counted in rejected)
	lost      int // v3 frames missing according to their seq

	last map[byte]v3Header // last frame of each v3 type, for seq gaps
}

// feed appends raw bytes and returns every complete frame decoded so far.
func (p *frameParser) feed(b []byte) []frameData {
	p.buf = append(p.buf, b...)
	var out []frameData

	for {
		i := bytes.Index(p.buf, []byte{frameSync0, frameSync1})
		if i < 0 {
			// Keep a trailing 0xAA: it may be the first half of the next sync.
			if n := len(p.buf); n > 0 && p.buf[n-1] == frameSync0 {
				p.discarded += n - 1
				p.buf = p.buf[n-1:]
			} else {
				p.discarded += n
				p.buf = p.buf[:0]
			}
			return out
		}
		p.discarded += i
		p.buf = p.buf[i:]
		if len(p.buf) < 3 {
			return out
		}
		if p.buf[2] != 3 {
			p.reject() // wrong version: a false sync
			continue
		}
		if len(p.buf) < 6 {
			return out
		}
		typ, plen := p.buf[3], int(p.buf[5])
		if want, known := v3ExpectedLen[typ]; known && plen != want {
			p.reject()
			continue
		}
		n := v3HeaderLen + plen + 2
		if len(p.buf) < n {
			return out
		}
		fd, err := decodeV3(p.buf[:n])
		if err != nil {
			if err == errCRC {
				p.crcErrors++
			}
			p.reject()
			continue
		}
		p.buf = p.buf[n:]
		p.trackSeq(fd)
		if fd.kind != "" {
			out = append(out, fd)
		}
	}
}

// reject drops the first byte of a false or corrupt sync and rescans.
func (p *frameParser) reject() {
	p.rejected++
	p.discarded++
	p.buf = p.buf[1:]
}

// trackSeq counts frames missing between consecutive seq values of a type.
// A restart of the main ECU (its clock goes back) starts a new count.
func (p *frameParser) trackSeq(fd frameData) {
	if p.last == nil {
		p.last = map[byte]v3Header{}
	}
	if prev, ok := p.last[fd.typ]; ok && fd.ecuTimeMs >= prev.ecuTimeMs {
		p.lost += int(fd.seq - prev.seq - 1)
	}
	p.last[fd.typ] = v3Header{typ: fd.typ, seq: fd.seq, ecuTimeMs: fd.ecuTimeMs}
}
