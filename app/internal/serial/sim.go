package serial

import (
	"fmt"
	"io"
	"math"
	"math/rand"
	"sync"
	"time"
)

// SimulatorPort is the pseudo port that connects to the built-in simulator
// instead of a serial device. ListPorts offers it after the real ports.
const SimulatorPort = "SIMULATOR"

const simPeriod = 100 * time.Millisecond

// simulator is an io.ReadWriteCloser that produces the byte stream of a car
// running the scripted scenario in simScenario: v3 fast frames at 10 Hz and
// slow frames at 1 Hz, through the same parser as a real XBee link.
type simulator struct {
	start     time.Time
	closed    chan struct{}
	closeOnce sync.Once
	slot      int // 100 ms slots produced so far
	pending   []byte
	sc        *simScenario
}

func newSimulator() *simulator {
	return &simulator{start: time.Now(), closed: make(chan struct{}), sc: newSimScenario(1)}
}

func (s *simulator) Read(p []byte) (int, error) {
	for len(s.pending) == 0 {
		next := s.start.Add(time.Duration(s.slot) * simPeriod)
		select {
		case <-s.closed:
			return 0, io.EOF
		case <-time.After(time.Until(next)):
		}
		s.pending = s.sc.chunk(float64(s.slot) * simPeriod.Seconds())
		s.slot++
	}
	n := copy(p, s.pending)
	s.pending = s.pending[n:]
	return n, nil
}

func (s *simulator) Write(p []byte) (int, error) { return len(p), nil }

func (s *simulator) Close() error {
	s.closeOnce.Do(func() { close(s.closed) })
	return nil
}

// simScenario scripts one 75 s cycle that exercises every channel and every
// kind of event the host detects, then repeats. Times are seconds since the
// main ECU booted (ecu_time):
//
//	 0– 6    boot: CAN messages appear one by one in valid_mask (fields are 0
//	         until their message arrives); tractive system precharges (AIR−,
//	         precharge relay, AIR+); drivers go NMT boot-up → pre-operational →
//	         operational and CiA 402 switch on disabled → … → operation
//	         enabled; drive and ready-to-drive at 5.5
//	 6–74    16 s laps: launch, full throttle, braking, corner, exit, coast.
//	         Launches spin the left rear wheel and traction control trims it;
//	         corner exits trim the right side; the race map falls off with
//	         speed and a speed limit kicks in above 31 m/s
//	15–19    driver R warning 0x0008
//	22–22.6  TPS2 20 points below TPS1; the ECU latches an APPS fault (safe
//	         mode, torque 0) until the driver toggles drive at 25.5–26
//	30–31.5  link lost: the ECU keeps sending, 15 fast + 2 slow frames are lost
//	35       line noise, a frame with a wrong version and one with a bad CRC
//	36–40    CAN error counters climb, error passive, bus-off at 39
//	38–46.5  driver L motor heats up: warning, derating at 40, fault 0x4310 at
//	         42, fault reset and re-enable; a loop overrun at 40
//	50–53    driver R quick stop, then re-enabled
//	55–58.5  driver R silent (heartbeat lost, its messages go stale), error
//	         0x8130, re-enabled
//	56–59    cells at 58 °C: BMS lowers its current limit to 80 A (cell
//	         temperature derating, BMS current limit on full throttle)
//	58       calibration command 1 → 2
//	60       mode race → normal
//	62–63    brake with the pedal at 50 %: EV.4.7, safe mode; loop overruns at 61
//	64–65    rear sensor (0x504) timeout
//	66–68    shutdown circuit opens at segment 3: AIRs open, link voltage
//	         decays, drivers disabled; precharge again 68–69.2
//	69–70    driver R temperature derating
//	72–73.5  low cell voltage derating
//	73.5–74  low-voltage supply dips to 8.6 V and the main ECU browns out:
//	         silence, then it boots again (seq and ecu_time restart)
type simScenario struct {
	rng     *rand.Rand
	lastT   float64 // cycle time of the previous chunk
	cycle   int     // ECU boots so far
	fastSeq byte
	slowSeq byte
	speed   float64    // m/s
	soc     float64    // %
	tqAct   [2]float64 // torque the drivers actually apply, ‰
	rear    float64    // last rear brake reading, frozen while 0x504 times out
}

const (
	simCycle       = 75.0 // s
	simWheelRadius = 0.23 // m
	simGearRatio   = 4.0
	simCells       = 96

	cia402Disabled   = 0x0250 // switch on disabled
	cia402Ready      = 0x0231 // ready to switch on
	cia402SwitchedOn = 0x0233
	cia402Enabled    = 0x0637 // operation enabled, remote, target reached
	cia402QuickStop  = 0x0217
	cia402Fault      = 0x0218
)

func newSimScenario(seed int64) *simScenario {
	return &simScenario{rng: rand.New(rand.NewSource(seed)), lastT: -1, soc: 87}
}

// chunk returns the bytes received during the 100 ms slot starting at t
// (seconds since connect): a fast frame, plus a slow frame every second.
func (s *simScenario) chunk(t float64) []byte {
	ct := math.Mod(t+1e-9, simCycle) // nudge so k*0.1 lands on the slot it names
	if ct < s.lastT {
		// The ECU booted again: counters restart, the car is stopped.
		s.cycle++
		s.fastSeq, s.slowSeq, s.speed, s.tqAct = 0, 0, 0, [2]float64{}
		if s.soc < 30 {
			s.soc = 87 // "recharged"
		}
	}
	s.lastT = ct
	if ct >= 74 {
		return nil // browned out
	}

	ms := uint32(math.Round(ct * 1000))
	frame := encodeV3(v3TypeFast, s.fastSeq, ms, s.values(ct))
	s.fastSeq++
	out := frame
	if int(math.Round(ct*10))%10 == 0 {
		out = append(out, encodeV3(v3TypeSlow, s.slowSeq, ms, s.slowValues(ct))...)
		s.slowSeq++
	}

	if between(ct, 30, 31.5) {
		return nil // sent by the ECU, lost on the link
	}
	if between(ct, 35, 35.1) {
		noise := make([]byte, 40)
		s.rng.Read(noise)
		for i, b := range noise {
			if b == frameSync0 {
				noise[i] = 0 // no accidental sync: the noise must only be discarded
			}
		}
		badVersion := append([]byte(nil), frame...)
		badVersion[2] = 9
		badCRC := append([]byte(nil), frame...)
		badCRC[60] ^= 0x10
		out = append(append(append(noise, badVersion...), badCRC...), out...)
	}
	return out
}

// values returns the fast frame channels, in physical units, at cycle time ct.
func (s *simScenario) values(ct float64) map[string]float64 {
	const dt = 0.1
	d := map[string]float64{}
	noise := func(sd float64) float64 { return s.rng.NormFloat64() * sd }

	pedal, brake, steer := simInputs(ct)
	driveOn := ct >= 5.5 && !between(ct, 25.5, 26)
	appsFault := between(ct, 22.2, 25.5) // latched until drive is toggled
	ev47 := between(ct, 62.1, 63)
	race := ct < 60
	drv := [2]simDriver{simDriverState(0, ct), simDriverState(1, ct)}

	// Pedals, brakes, steering.
	tps1 := clamp(pedal+noise(0.3), 0, 100)
	tps2 := tps1 - 0.8 + noise(0.3)
	if between(ct, 22, 22.6) {
		tps2 = tps1 - 20
	}
	d["tps1"], d["tps2"] = tps1, clamp(tps2, 0, 100)
	d["steering_angle"] = steer*700 + noise(3)
	d["front_brake"] = 50 + 650*brake + noise(4) // PSI, ~50 at rest
	if !between(ct, 64, 65) {
		s.rear = 40 + 0.7*650*brake + noise(4)
	}
	d["rear_brake"] = s.rear
	d["calibration_type"] = simCalibration(ct)

	// Torque: mode map, then limits, then traction control.
	var reason uint16
	gain := 10.0 // race map: pedal % → ‰ of rated torque
	if !race {
		gain = 3.7
	}
	request := pedal * gain
	cmd := [2]float64{request, request}
	scale := func(bit int, f float64, sides ...int) {
		reason |= 1 << bit
		for _, i := range sides {
			cmd[i] *= f
		}
	}
	if !driveOn {
		scale(0, 0, 0, 1)
	}
	if appsFault {
		scale(1, 0, 0, 1)
	}
	if ev47 {
		scale(2, 0, 0, 1)
	}
	if race && s.speed > 22 && request > 0 {
		scale(4, 22/s.speed, 0, 1) // race map fall-off with speed
	}
	if between(ct, 40, 42) {
		scale(6, 0.7, 0) // motor L temperature
	}
	if between(ct, 69, 70) {
		scale(7, 0.8, 1) // driver R temperature
	}
	bmsLimit := 250.0
	if between(ct, 56, 59) {
		bmsLimit = 80
		scale(8, 1, 0, 1) // cell temperature: the limit itself does the work
	}
	if between(ct, 72, 73.5) {
		scale(9, 0.6, 0, 1) // low cell voltage
	}
	if s.speed > 31 {
		scale(10, 0.5, 0, 1)
	}
	slip, tc := [2]float64{1, 1}, [2]float64{1, 1}
	if ct >= 6 {
		lap := math.Mod(ct-6, 16)
		if between(lap, 0.5, 2) {
			slip[0], tc[0] = 1.3, 0.55 // launch: left rear spins
		}
		if between(lap, 12.6, 13.2) {
			slip[1], tc[1] = 1.12, 0.75 // corner exit: right rear slips
		}
	}
	for i := range cmd {
		if tc[i] < 1 && cmd[i] > 0 {
			scale(3, tc[i], i)
		}
		if driveOn && !drv[i].enabled() {
			reason |= 1 << 11
		}
	}

	// Tractive system: precharge at boot and after the shutdown circuit opens.
	sdcOpen := between(ct, 66, 68)
	prechargeRelay := between(ct, 2, 3.2) || between(ct, 68, 69.2)
	airPlus := ct >= 3.2 && !between(ct, 66, 69.2)
	cellAvg := 3.3 + 0.009*s.soc
	packOCV := simCells * cellAvg
	var link float64
	switch {
	case ct < 2:
		link = 0
	case prechargeRelay:
		start := 2.0
		if ct >= 68 {
			start = 68
		}
		link = 0.95 * packOCV * (ct - start) / 1.2
	case sdcOpen:
		link = packOCV * math.Exp(-(ct-66)/0.3)
	default:
		link = packOCV
	}

	// Drivers and currents; the BMS current limit scales both sides.
	wheel := s.speed / (2 * math.Pi * simWheelRadius) * 60 // rpm
	var target, dcI [2]float64
	current := func() float64 {
		for i := range drv {
			target[i] = 0
			if drv[i].enabled() && airPlus {
				target[i] = cmd[i]
			}
			rpm := wheel * simGearRatio * slip[i]
			dcI[i] = target[i] / 1000 * 140 * (0.15 + rpm/5500)
		}
		return dcI[0] + dcI[1]
	}
	if packI := current(); packI > bmsLimit {
		scale(5, bmsLimit/packI, 0, 1)
		current()
	}
	packI := dcI[0] + dcI[1]
	s.soc = math.Max(0, s.soc-packI*dt/3600/13*100) // 13 Ah pack

	// Longitudinal model: only what the drivers apply pushes the car.
	accel := 9*(target[0]+target[1])/2000 - 0.0012*s.speed*s.speed - 12*brake
	if s.speed > 0 {
		accel -= 0.15
	}
	s.speed = math.Max(0, s.speed+accel*dt)
	d["wheel_speed_fl"] = math.Max(0, wheel*(1-0.04*steer)+noise(1))
	d["wheel_speed_fr"] = math.Max(0, wheel*(1+0.04*steer)+noise(1))

	var state uint16
	if race {
		state |= 1
	}
	if driveOn {
		state |= 1<<2 | 1<<3 // drive, ready-to-drive
	}
	if ct >= 1.2 {
		state |= 1 << 4 // traction control enabled
	}
	if tc[0] < 1 && cmd[0] > 0 {
		state |= 1 << 5
	}
	if tc[1] < 1 && cmd[1] > 0 {
		state |= 1 << 6
	}
	if appsFault {
		state |= 1<<7 | 1<<10
	}
	if ev47 {
		state |= 1<<8 | 1<<10
	}
	if brake > 0.15 {
		state |= 1 << 9
	}
	d["ecu_state"] = float64(state)
	d["torque_limit_reason"] = float64(reason)
	d["torque_request"] = math.Round(request)
	d["torque_cmd_l"], d["torque_cmd_r"] = math.Round(cmd[0]), math.Round(cmd[1])

	// Accumulator.
	d["pack_voltage"] = packOCV - 0.06*packI + noise(0.2)
	d["pack_current"] = packI + noise(0.3)
	d["soc"] = math.Round(s.soc)
	cellTemp := 28 + 30*smooth(6, 56, ct) - 12*smooth(59, 74, ct)
	d["cell_temp_max"] = math.Round(cellTemp)
	cellMin := cellAvg - 0.02 - packI*0.0004
	if between(ct, 72, 73.5) {
		cellMin = 3.15
	}
	d["cell_v_min"], d["cell_v_max"] = cellMin, cellAvg+0.012
	d["bms_current_limit"] = bmsLimit
	if between(ct, 56, 58) {
		d["bms_fault"] = 0x0004 // over-temperature warning
	}

	var ts uint16
	set := func(bit int, on bool) {
		if on {
			ts |= 1 << bit
		}
	}
	set(0, ct >= 1.5 && !sdcOpen) // shutdown circuit closed
	set(1, airPlus)               // AIR+
	set(2, ct >= 2 && !sdcOpen)   // AIR−
	set(3, prechargeRelay)        // precharge relay
	set(4, airPlus)               // precharge done
	set(5, ct >= 1)               // IMD ok
	set(6, ct >= 0.8)             // AMS ok
	set(7, ct >= 1 && !sdcOpen)   // BSPD ok
	set(8, link > 60)             // tractive system active
	set(9, ct >= 1.4)             // TSMS on
	d["ts_state"] = float64(ts)
	if sdcOpen {
		d["sdc_open_segment"] = 3
	}

	// Currents: kept in the units the ECU sends (see currentScale).
	d["dc_current_1"], d["dc_current_2"] = dcI[0], dcI[1]
	d["ac_current_1"] = dcI[0]*1.6 + noise(1)
	d["ac_current_2"] = dcI[1]*1.6 + noise(1)

	// IMU, coolant, low voltage.
	d["accel_x"] = accel/9.81 + noise(0.005)
	d["accel_y"] = steer*s.speed*s.speed/(9.81*12) + noise(0.005)
	d["yaw_rate"] = steer*s.speed/12*180/math.Pi + noise(0.1)
	d["coolant_temp"] = math.Round(25 + 25*smooth(6, simCycle, ct))
	lv := 13.2 + noise(0.02)
	if link > 60 {
		lv += 0.5 // DC-DC running
	}
	if ct >= 73.5 {
		lv = 8.6
	}
	d["lv_voltage"] = lv

	// Temperatures: driver L's motor overheats towards its fault at 42 s.
	motorT := [2]float64{35 + 60*smooth(6, 42, ct), 35 + 25*smooth(6, simCycle, ct)}
	if ct >= 44.5 {
		motorT[0] = 95 - 25*smooth(44.5, 60, ct)
	}
	for i := range drv {
		p := fmt.Sprintf("drv%d_", i)
		s.tqAct[i] = 0.7*s.tqAct[i] + 0.3*target[i]
		d[p+"control_word"] = drv[i].control
		d[p+"status_word"] = drv[i].status
		d[p+"warnings"] = drv[i].warnings
		d[p+"error"] = drv[i].err
		d[p+"heartbeat_state"] = drv[i].nmt
		d[p+"target_torque"] = target[i]
		d[p+"torque_actual"] = math.Round(s.tqAct[i])
		d[p+"current_actual"] = math.Round(math.Abs(s.tqAct[i]) * 1.1)
		d[p+"motor_velocity"] = wheel*simGearRatio*slip[i] + noise(2)
		d[p+"motor_temp"] = motorT[i] + noise(0.2)
		d[p+"driver_temp"] = 30 + 15*smooth(6, simCycle, ct) + 2*float64(i) + noise(0.2)
		if i == 1 && between(ct, 69, 70) {
			d[p+"driver_temp"] += 20
		}
		d[p+"driver_dc_voltage"] = math.Max(0, link-0.06*packI+noise(0.2))
	}
	// Rear wheel speeds: the ECU derives them from motor velocity.
	d["wheel_speed_rl"] = math.Max(0, d["drv0_motor_velocity"]/simGearRatio)
	d["wheel_speed_rr"] = math.Max(0, d["drv1_motor_velocity"]/simGearRatio)

	// Fields whose CAN message has not been received yet are sent as 0.
	valid := simValidMask(ct)
	d["valid_mask"] = float64(valid)
	d["fresh_mask"] = float64(valid &^ simStaleMask(ct))
	for key, bit := range simFieldBits {
		if valid&(1<<bit) == 0 {
			d[key] = 0
		}
	}
	return d
}

// slowValues returns the slow frame channels at cycle time ct.
func (s *simScenario) slowValues(ct float64) map[string]float64 {
	d := map[string]float64{
		"fw_major": 0, "fw_minor": 3, "fw_patch": 0, "fw_dirty": 1,
		"git_hash":                 0x0885E466,
		"uptime_s":                 math.Floor(ct),
		"reset_count":              float64(s.cycle + 1),
		"last_reset_cause":         1, // power-on
		"cpu_load":                 math.Round(38 + 8*math.Sin(ct/5)),
		"loop_time_max":            math.Round(850 + 100*math.Sin(ct/3)),
		"drv0_motor_rated_current": 200000, "drv1_motor_rated_current": 200000,
		"drv0_motor_rated_torque": 9.8, "drv1_motor_rated_torque": 9.8,
		"cell_v_min_id": 37, "cell_v_max_id": 12, "cell_temp_max_id": 64,
	}
	if s.cycle > 0 {
		d["last_reset_cause"] = 7 // brown-out
	}
	switch {
	case ct >= 61:
		d["loop_overruns"] = 4
	case ct >= 40:
		d["loop_overruns"] = 1
	}
	if between(ct, 40, 40.5) || between(ct, 61, 61.5) {
		d["loop_time_max"], d["cpu_load"] = 2400, 92
	}
	var faults uint32
	if between(ct, 64, 65.5) {
		faults |= 1 << 2 // rear sensor timeout
	}
	if between(ct, 22, 26) {
		faults |= 1 << 3 // 0x401 sensor implausibility, latched
	}
	if between(ct, 62, 63.5) {
		faults |= 1 << 4 // EV.4.7
	}
	d["active_faults"] = float64(faults)
	switch {
	case between(ct, 36, 38):
		d["can_tx_err"] = 96 + 20*(ct-36) // warning, then error passive
	case between(ct, 38, 40):
		d["can_tx_err"] = 200
	default:
		d["can_tx_err"] = 0
	}
	d["can_rx_err"] = math.Floor(math.Mod(ct, 4))
	if ct >= 39 {
		d["can_busoff_count"] = 1
	}
	return d
}

// simInputs returns the pedal (%), brake (0–1) and steering (-1–1) at cycle
// time ct.
func simInputs(ct float64) (pedal, brake, steer float64) {
	if between(ct, 62, 62.8) {
		return 50, 0.5, 0 // brake and throttle at once
	}
	if ct < 6 {
		return 0, 0.1, 0 // parked, foot on the brake
	}
	lap := math.Mod(ct-6, 16)
	switch {
	case lap < 1.5: // launch
		pedal = 100 * lap / 1.5
	case lap < 6:
		pedal = 100
	case lap < 6.5:
		pedal = 100 * (6.5 - lap) / 0.5
	case lap < 8.5: // braking
		brake = math.Sin(math.Pi * (lap - 6.5) / 2)
	case lap < 12.5: // corner
		pedal = 35 + 10*math.Sin(2*math.Pi*(lap-8.5)/4)
		steer = math.Sin(math.Pi * (lap - 8.5) / 4)
	case lap < 14: // exit
		pedal = 80
		steer = 0.2 * (14 - lap) / 1.5
	default: // coast
		pedal = 10
	}
	return pedal, brake, steer
}

type simDriver struct {
	status, control, err, warnings, nmt float64
}

func (d simDriver) enabled() bool { return d.status == cia402Enabled }

// simEnable is the CiA 402 enable sequence, dt seconds after it starts:
// switch on disabled → ready → switched on → operation enabled, each with the
// control word that requests the next step.
func simEnable(dt float64) (status, control float64) {
	switch {
	case dt < 0.5:
		return cia402Disabled, 0x0000
	case dt < 1.0:
		return cia402Ready, 0x0006 // shutdown
	case dt < 1.5:
		return cia402SwitchedOn, 0x0007 // switch on
	default:
		return cia402Enabled, 0x000F // enable operation
	}
}

func simDriverState(n int, ct float64) simDriver {
	lag := 0.3 * float64(n)
	d := simDriver{nmt: 5}
	switch {
	case ct < 1.8+lag:
		d.nmt = 0 // boot-up
	case ct < 2.5+lag:
		d.nmt = 127 // pre-operational
	}
	d.status, d.control = simEnable(ct - 2.5 - lag)

	if n == 0 {
		if between(ct, 38, 44.5) {
			d.warnings = 0x0002
		}
		switch {
		case between(ct, 42, 44.5):
			d.status, d.err, d.control = cia402Fault, 0x4310, 0x000F
			if ct >= 44.3 {
				d.control = 0x0080 // fault reset
			}
		case between(ct, 44.5, 47):
			d.status, d.control = simEnable(ct - 44.5)
		}
	} else {
		if between(ct, 15, 19) {
			d.warnings = 0x0008
		}
		switch {
		case between(ct, 50, 51):
			d.status, d.control = cia402QuickStop, 0x000B
		case between(ct, 51, 53):
			d.status, d.control = simEnable(ct - 51)
		case between(ct, 55, 57):
			d.nmt, d.err = 127, 0x8130
			d.status, d.control = cia402Disabled, 0x0000
		case between(ct, 57, 59):
			d.status, d.control = simEnable(ct - 57)
		}
	}

	// Shutdown circuit open: no DC link, both drivers fall back and re-enable
	// once the precharge is done.
	if between(ct, 66, 71) {
		d.status, d.control = simEnable(ct - 69.2)
	}
	return d
}

func simCalibration(ct float64) float64 {
	switch {
	case ct < 1.2:
		return 0
	case ct < 58:
		return 1
	default:
		return 2
	}
}

// simValidMask: when each CAN message is first received after boot.
func simValidMask(ct float64) uint32 {
	at := map[int]float64{
		0: 0.3, 1: 0.5, 2: 0.7, 3: 0.9, 4: 1.2, // sensors
		15: 0.8, 16: 2.3, 17: 0.6, 18: 1.1, 19: 0.2, 20: 0.4, // BMS, rear wheels, IMU, coolant, LV, TS
	}
	for n := 0; n < 2; n++ {
		lag := 0.3 * float64(n)
		at[5+5*n] = 2.6 + lag // RXPDO1
		at[6+5*n] = 1.9 + lag // TXPDO1
		at[7+5*n] = 2.0 + lag // TXPDO2
		at[8+5*n] = 2.1 + lag // TXPDO3
		at[9+5*n] = 1.0 + lag // heartbeat
	}
	var mask uint32
	for bit, t := range at {
		if ct >= t {
			mask |= 1 << bit
		}
	}
	return mask
}

// simStaleMask: messages that stop arriving for a while (fresh_mask bit off).
func simStaleMask(ct float64) uint32 {
	var m uint32
	if between(ct, 55, 57) {
		m |= 0xF << 11 // driver R TXPDO1–3 and heartbeat
	}
	if between(ct, 64, 65) {
		m |= 1 << 3 // rear sensor
	}
	return m
}

// simFieldBits maps each fast frame field to the valid_mask bit of the CAN
// message it comes from (the driver PDO contents are an assumption).
var simFieldBits = func() map[string]int {
	m := map[string]int{
		"tps1": 0, "tps2": 0,
		"wheel_speed_fl": 1, "wheel_speed_fr": 1, "steering_angle": 1, "front_brake": 1,
		"ac_current_1": 2, "ac_current_2": 2, "dc_current_1": 2, "dc_current_2": 2,
		"rear_brake":       3,
		"calibration_type": 4,
		"pack_voltage":     15, "pack_current": 15, "soc": 15, "cell_temp_max": 15,
		"cell_v_min": 15, "cell_v_max": 15, "bms_current_limit": 15, "bms_fault": 15,
		"wheel_speed_rl": 16, "wheel_speed_rr": 16,
		"accel_x": 17, "accel_y": 17, "yaw_rate": 17,
		"coolant_temp": 18,
		"lv_voltage":   19,
		"ts_state":     20, "sdc_open_segment": 20,
	}
	for n := 0; n < 2; n++ {
		p := fmt.Sprintf("drv%d_", n)
		base := 5 + 5*n
		for _, f := range []string{"control_word", "target_torque"} {
			m[p+f] = base // RXPDO1
		}
		for _, f := range []string{"status_word", "warnings", "error"} {
			m[p+f] = base + 1 // TXPDO1
		}
		for _, f := range []string{"motor_velocity", "torque_actual", "current_actual"} {
			m[p+f] = base + 2 // TXPDO2
		}
		for _, f := range []string{"driver_temp", "driver_dc_voltage", "motor_temp"} {
			m[p+f] = base + 3 // TXPDO3
		}
		m[p+"heartbeat_state"] = base + 4
	}
	return m
}()

func between(t, a, b float64) bool { return t >= a && t < b }

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// smooth rises from 0 at t=a to 1 at t=b.
func smooth(a, b, t float64) float64 {
	x := clamp((t-a)/(b-a), 0, 1)
	return x * x * (3 - 2*x)
}
