package serial

import (
	"fmt"
	"testing"
)

// Runs two simulator cycles through the parser, without real-time pacing.
func TestSimulatorScenario(t *testing.T) {
	sc := newSimScenario(1)
	p := &frameParser{}
	var fast, slow []map[string]float64
	const slots = int(2 * simCycle * 10)
	for k := 0; k < slots; k++ {
		for _, fd := range p.feed(sc.chunk(float64(k) * simPeriod.Seconds())) {
			if fd.kind == KindSlow {
				slow = append(slow, fd.data)
			} else {
				fast = append(fast, fd.data)
			}
		}
	}

	// Per cycle: 750 slots, 15 lost on the link and 10 with the ECU off;
	// slow frames at 0…73 s, 2 of them lost on the link.
	if want := 2 * (750 - 15 - 10); len(fast) != want {
		t.Errorf("got %d fast frames, want %d", len(fast), want)
	}
	if want := 2 * (74 - 2); len(slow) != want {
		t.Errorf("got %d slow frames, want %d", len(slow), want)
	}
	// The ECU reboot restarts seq and ecu_time, which must not count as loss.
	if p.lost != 2*17 {
		t.Errorf("lost = %d, want %d", p.lost, 2*17)
	}
	// Per cycle: 40 noise bytes, a wrong-version frame and a bad-CRC frame.
	if p.rejected != 4 || p.crcErrors != 2 || p.discarded != 2*(40+2*v3FastLen) {
		t.Errorf("rejected=%d crcErrors=%d discarded=%d", p.rejected, p.crcErrors, p.discarded)
	}

	// Every channel must change at some point, except the slow frame's
	// identity fields, which are constant on a real car too.
	constant := map[string]bool{
		"fw_major": true, "fw_minor": true, "fw_patch": true, "fw_dirty": true, "git_hash": true,
		"drv0_motor_rated_current": true, "drv1_motor_rated_current": true,
		"drv0_motor_rated_torque": true, "drv1_motor_rated_torque": true,
		"cell_v_min_id": true, "cell_v_max_id": true, "cell_temp_max_id": true,
	}
	seen := map[string]map[float64]bool{}
	for _, f := range append(append([]map[string]float64{}, fast...), slow...) {
		for k, v := range f {
			if seen[k] == nil {
				seen[k] = map[float64]bool{}
			}
			seen[k][v] = true
		}
	}
	// fast: 36 header fields + 2 × 12 driver fields + seq, ecu_time; slow: 22
	if len(seen) != 62+22 {
		t.Errorf("got %d distinct channels, want %d", len(seen), 62+22)
	}
	for k, vals := range seen {
		if len(vals) < 2 && !constant[k] {
			t.Errorf("channel %s never changes", k)
		}
	}

	has := func(frames []map[string]float64, what string, ok func(f map[string]float64) bool) {
		t.Helper()
		for _, f := range frames {
			if ok(f) {
				return
			}
		}
		t.Errorf("scenario never shows %s", what)
	}
	bit := func(f map[string]float64, key string, b int) bool { return uint32(f[key])&(1<<b) != 0 }

	has(fast, "all CAN messages valid", func(f map[string]float64) bool { return f["valid_mask"] == 0x1FFFFF })
	has(fast, "ECU boot (valid_mask 0)", func(f map[string]float64) bool { return f["valid_mask"] == 0 && f["tps1"] == 0 })
	has(fast, "a stale message", func(f map[string]float64) bool { return f["fresh_mask"] != f["valid_mask"] })
	has(fast, "both drivers enabled", func(f map[string]float64) bool {
		return f["drv0_status_word"] == cia402Enabled && f["drv1_status_word"] == cia402Enabled
	})
	has(fast, "driver L temperature fault", func(f map[string]float64) bool {
		return f["drv0_status_word"] == cia402Fault && f["drv0_error"] == 0x4310
	})
	has(fast, "driver R quick stop", func(f map[string]float64) bool { return f["drv1_status_word"] == cia402QuickStop })
	has(fast, "driver R heartbeat lost", func(f map[string]float64) bool {
		return f["drv1_heartbeat_state"] == 127 && f["drv1_error"] == 0x8130
	})
	has(fast, "TPS implausibility", func(f map[string]float64) bool { return f["tps1"]-f["tps2"] >= 19 })
	has(fast, "ECU safe mode", func(f map[string]float64) bool { return bit(f, "ecu_state", 10) })
	has(fast, "normal mode", func(f map[string]float64) bool {
		return f["ecu_state"] != 0 && !bit(f, "ecu_state", 0) && bit(f, "ecu_state", 2)
	})
	for b := 0; b <= 11; b++ {
		has(fast, fmt.Sprintf("torque_limit_reason bit %d", b), func(f map[string]float64) bool {
			return bit(f, "torque_limit_reason", b)
		})
	}
	for b := 2; b <= 10; b++ { // bits 0–1 are the mode
		has(fast, fmt.Sprintf("ecu_state bit %d", b), func(f map[string]float64) bool { return bit(f, "ecu_state", b) })
	}
	for b := 0; b <= 9; b++ {
		has(fast, fmt.Sprintf("ts_state bit %d", b), func(f map[string]float64) bool { return bit(f, "ts_state", b) })
	}
	has(fast, "shutdown circuit open", func(f map[string]float64) bool { return f["sdc_open_segment"] == 3 && !bit(f, "ts_state", 0) })
	has(fast, "traction control trimming the left side", func(f map[string]float64) bool {
		return bit(f, "ecu_state", 5) && f["torque_cmd_l"] < f["torque_request"]
	})
	has(fast, "speed over 80 km/h", func(f map[string]float64) bool {
		return f["wheel_speed_fl"] > 80/3.6/(2*3.14159*simWheelRadius)*60
	})
	has(fast, "low-voltage dip", func(f map[string]float64) bool { return f["lv_voltage"] > 0 && f["lv_voltage"] < 9 })
	has(slow, "a loop overrun", func(f map[string]float64) bool { return f["loop_overruns"] > 0 })
	has(slow, "CAN error passive", func(f map[string]float64) bool { return f["can_tx_err"] > 127 })
	has(slow, "bus-off", func(f map[string]float64) bool { return f["can_busoff_count"] > 0 })
	has(slow, "a brown-out reset", func(f map[string]float64) bool { return f["last_reset_cause"] == 7 })
	for b := 2; b <= 4; b++ {
		has(slow, "active fault", func(f map[string]float64) bool { return bit(f, "active_faults", b) })
	}
}
