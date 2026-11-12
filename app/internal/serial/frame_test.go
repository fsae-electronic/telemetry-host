package serial

import (
	"encoding/binary"
	"encoding/hex"
	"math"
	"strings"
	"testing"
)

func TestCRC16CheckValue(t *testing.T) {
	if got := crc16([]byte("123456789")); got != 0x29B1 {
		t.Errorf("crc16 check value = %04X, want 29B1", got)
	}
}

func TestLayoutSizes(t *testing.T) {
	if n := v3HeaderLen + v3FastLayout.size() + 2; n != v3FastLen {
		t.Errorf("fast frame %d bytes, want %d", n, v3FastLen)
	}
	if n := v3HeaderLen + v3SlowLayout.size() + 2; n != v3SlowLen {
		t.Errorf("slow frame %d bytes, want %d", n, v3SlowLen)
	}
}

// specExample is the example fast frame in the v3 spec ("Frame de
// telemetría v3", Implementación), CRC 0xD4D7.
const specExample = `
AA 55 03 01 2A 76 40 E2 01 00 FF 7F 00 00 FF 7F
00 00 05 00 00 00 FF 00 02 00 C7 01 C4 01 78 00
5F 00 67 FF 5A 18 6A 18 00 00 00 00 C7 01 C2 01
C2 01 8E 0F F0 05 57 22 46 0F 91 0F D0 07 00 00
F8 02 F3 02 BA 04 B5 04 5E 01 88 FF FA 05 29 00
FB 04 0F 00 37 06 00 00 00 00 C2 01 AE 01 2C 01
80 0C 00 00 91 0F 29 37 05 0F 00 37 06 00 00 00
00 C2 01 AE 01 2C 01 80 0C 00 00 91 0F 29 37 05
D7 D4`

func specFrame(t *testing.T) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.Join(strings.Fields(specExample), ""))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDecodeSpecExample(t *testing.T) {
	f := specFrame(t)
	fd, err := decodeV3(f)
	if err != nil {
		t.Fatal(err)
	}
	if fd.kind != KindFast || fd.seq != 42 || fd.ecuTimeMs != 123456 {
		t.Fatalf("kind %q seq %d time %d", fd.kind, fd.seq, fd.ecuTimeMs)
	}
	// Values from the spec's description of the example. Steering and the
	// currents are raw for now (steeringScale, currentScale = 1).
	want := map[string]float64{
		"seq": 42, "ecu_time": 123.456,
		"valid_mask": 0x7FFF, "fresh_mask": 0x7FFF, "ecu_state": 0b101, "ts_state": 0xFF,
		"calibration_type": 2, "tps1": 45.5, "tps2": 45.2, "front_brake": 120, "rear_brake": 95,
		"steering_angle": -153 * steeringScale, "wheel_speed_fl": 623.4, "wheel_speed_fr": 625.0,
		"torque_request": 455, "torque_cmd_l": 450, "torque_cmd_r": 450,
		"pack_voltage": 398.2, "pack_current": 152.0, "soc": 87, "cell_temp_max": 34,
		"cell_v_min": 3.910, "cell_v_max": 3.985, "bms_current_limit": 200.0, "bms_fault": 0,
		"dc_current_1": 760 * currentScale, "ac_current_2": 1205 * currentScale,
		"accel_x": 0.350, "accel_y": -0.120, "yaw_rate": 15.30, "coolant_temp": 41, "lv_voltage": 12.75,
		"drv0_status_word": 0x0637, "drv0_target_torque": 450, "drv0_torque_actual": 430,
		"drv0_current_actual": 300, "drv0_motor_velocity": 3200, "drv0_driver_dc_voltage": 398.5,
		"drv0_driver_temp": 41, "drv0_motor_temp": 55, "drv1_heartbeat_state": 5,
	}
	for k, v := range want {
		if math.Abs(fd.data[k]-v) > 1e-6 {
			t.Errorf("%s = %v, want %v", k, fd.data[k], v)
		}
	}

	// Any flipped bit must be caught by the CRC.
	bad := append([]byte(nil), f...)
	bad[40] ^= 0x01
	if _, err := decodeV3(bad); err != errCRC {
		t.Errorf("corrupted frame: err = %v, want CRC mismatch", err)
	}
}

func TestEncodeV3MatchesSpecExample(t *testing.T) {
	f := specFrame(t)
	fd, _ := decodeV3(f)
	if got := encodeV3(v3TypeFast, fd.seq, fd.ecuTimeMs, fd.data); string(got) != string(f) {
		t.Errorf("encodeV3(decodeV3(example)) differs\n got %x\nwant %x", got, f)
	}
}

func TestSlowFrameRoundTrip(t *testing.T) {
	in := map[string]float64{
		"fw_major": 0, "fw_minor": 3, "fw_patch": 0, "fw_dirty": 1, "git_hash": 0x0885E466,
		"uptime_s": 4000000000, "reset_count": 12, "last_reset_cause": 2, "cpu_load": 41,
		"loop_overruns": 3, "loop_time_max": 950, "active_faults": 0b11000, "can_tx_err": 96,
		"can_rx_err": 127, "can_busoff_count": 1, "drv0_motor_rated_current": 200000,
		"drv1_motor_rated_current": 200000, "drv0_motor_rated_torque": 9.8, "drv1_motor_rated_torque": 9.8,
		"cell_v_min_id": 37, "cell_v_max_id": 12, "cell_temp_max_id": 64,
	}
	f := encodeV3(v3TypeSlow, 7, 5000, in)
	if len(f) != v3SlowLen {
		t.Fatalf("slow frame %d bytes", len(f))
	}
	fd, err := decodeV3(f)
	if err != nil || fd.kind != KindSlow {
		t.Fatalf("decode: %v kind %q", err, fd.kind)
	}
	for k, v := range in {
		if math.Abs(fd.data[k]-v) > 1e-6 {
			t.Errorf("%s = %v, want %v", k, fd.data[k], v)
		}
	}
}

func TestParserResync(t *testing.T) {
	f := specFrame(t)
	// garbage, a false sync with a bad version, then two frames split mid-way
	stream := append([]byte{0x01, 0xAA, 0x55, 0x09, 0x00}, f...)
	stream = append(stream, f...)

	p := &frameParser{}
	var got []frameData
	got = append(got, p.feed(stream[:40])...)
	got = append(got, p.feed(stream[40:200])...)
	got = append(got, p.feed(stream[200:])...)

	if len(got) != 2 {
		t.Fatalf("got %d frames, want 2", len(got))
	}
	for _, fd := range got {
		if fd.kind != KindFast || fd.data["tps1"] != 45.5 {
			t.Errorf("kind %q tps1 %v", fd.kind, fd.data["tps1"])
		}
	}
	if len(p.buf) != 0 {
		t.Errorf("leftover %d bytes", len(p.buf))
	}
	// 0x01, then the false sync AA 55 09 00 is rejected and its 4 bytes dropped.
	if p.discarded != 5 || p.rejected != 1 {
		t.Errorf("discarded=%d rejected=%d, want 5 and 1", p.discarded, p.rejected)
	}
}

func TestParserStream(t *testing.T) {
	fast := func(seq byte, ms uint32) []byte {
		return encodeV3(v3TypeFast, seq, ms, map[string]float64{"tps1": 10})
	}
	slow := encodeV3(v3TypeSlow, 0, 100, map[string]float64{"cpu_load": 40})
	corrupt := fast(2, 300)
	corrupt[50] ^= 0xFF
	unknown := []byte{0xAA, 0x55, 3, 9, 0, 1, 0, 0, 0, 0, 0x42}
	unknown = binary.LittleEndian.AppendUint16(unknown, crc16(unknown[2:]))

	var stream []byte
	stream = append(stream, "TMS570 MAIN ECU (rewrite) INITIALIZED\n"...) // boot text
	stream = append(stream, fast(0, 100)...)
	stream = append(stream, slow...)
	stream = append(stream, fast(1, 200)...)
	stream = append(stream, corrupt...)      // seq 2 corrupted on the link
	stream = append(stream, unknown...)      // future frame type: skipped
	stream = append(stream, fast(5, 600)...) // seq 3, 4 never arrived
	stream = append(stream, fast(0, 50)...)  // ECU restarted: seq back to 0, no loss

	p := &frameParser{}
	var got []frameData
	for i := 0; i < len(stream); i += 7 { // odd chunk size: frames split everywhere
		got = append(got, p.feed(stream[i:min(i+7, len(stream))])...)
	}

	var kinds []string
	for _, fd := range got {
		kinds = append(kinds, fd.kind)
	}
	if strings.Join(kinds, ",") != "fast,slow,fast,fast,fast" {
		t.Errorf("kinds = %v", kinds)
	}
	if p.crcErrors != 1 {
		t.Errorf("crcErrors = %d, want 1", p.crcErrors)
	}
	if p.lost != 3 { // seq 2 (corrupted), 3 and 4
		t.Errorf("lost = %d, want 3", p.lost)
	}
	if got[0].data["tps1"] != 10 || got[1].data["cpu_load"] != 40 {
		t.Errorf("values: %v / %v", got[0].data["tps1"], got[1].data["cpu_load"])
	}
}
