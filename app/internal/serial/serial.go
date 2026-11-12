package serial

import (
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vmihailenco/msgpack/v5"
	goserial "go.bug.st/serial"
)

// Packet is a decoded telemetry frame with a server-side timestamp. Kind is
// KindFast (10 Hz) or KindSlow (1 Hz).
type Packet struct {
	Ts   float64            `json:"ts"`
	Kind string             `json:"kind"`
	Data map[string]float64 `json:"data"`
}

// Manager owns the serial port lifecycle and publishes decoded packets.
type Manager struct {
	mu       sync.Mutex
	port     io.ReadWriteCloser // a serial port, or the built-in simulator
	portName string

	// Sink receives every successfully decoded packet.
	Sink chan Packet

	stopCh chan struct{}

	// Link counters for the current connection, reset on Connect.
	frames     atomic.Uint64
	slowFrames atomic.Uint64
	discarded  atomic.Uint64
	rejected   atomic.Uint64
	crcErrors  atomic.Uint64
	lost       atomic.Uint64
	dropped    atomic.Uint64
}

// LinkStats is a snapshot of the link counters since the last Connect.
type LinkStats struct {
	Frames         uint64 `json:"frames"`          // fast frames decoded
	SlowFrames     uint64 `json:"slow_frames"`     // slow frames decoded
	DiscardedBytes uint64 `json:"discarded_bytes"` // bytes skipped while resyncing
	Rejected       uint64 `json:"rejected"`        // syncs that did not lead to a valid frame
	CrcErrors      uint64 `json:"crc_errors"`      // frames with a bad CRC
	Lost           uint64 `json:"lost"`            // frames missing according to their seq
	Dropped        uint64 `json:"dropped"`         // frames lost because the consumer was slow
}

func (m *Manager) Stats() LinkStats {
	return LinkStats{
		Frames:         m.frames.Load(),
		SlowFrames:     m.slowFrames.Load(),
		DiscardedBytes: m.discarded.Load(),
		Rejected:       m.rejected.Load(),
		CrcErrors:      m.crcErrors.Load(),
		Lost:           m.lost.Load(),
		Dropped:        m.dropped.Load(),
	}
}

func NewManager() *Manager {
	return &Manager{
		Sink: make(chan Packet, 256),
	}
}

func (m *Manager) Connected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.port != nil
}

func (m *Manager) PortName() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.portName
}

// Connect opens the serial port and starts the read loop.
func (m *Manager) Connect(portName string, baud int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.port != nil {
		m.port.Close()
		close(m.stopCh)
	}

	var p io.ReadWriteCloser
	if portName == SimulatorPort {
		p = newSimulator()
	} else {
		sp, err := goserial.Open(portName, &goserial.Mode{BaudRate: baud})
		if err != nil {
			return err
		}
		p = sp
	}

	m.port = p
	m.portName = portName
	m.stopCh = make(chan struct{})
	m.frames.Store(0)
	m.slowFrames.Store(0)
	m.discarded.Store(0)
	m.rejected.Store(0)
	m.crcErrors.Store(0)
	m.lost.Store(0)
	m.dropped.Store(0)

	go m.readLoop(p, m.stopCh)
	return nil
}

// Disconnect closes the active port.
func (m *Manager) Disconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.port == nil {
		return
	}
	m.port.Close()
	close(m.stopCh)
	m.port = nil
	m.portName = ""
}

func (m *Manager) SendCommand(cmd map[string]any) error {
	m.mu.Lock()
	p := m.port
	m.mu.Unlock()

	if p == nil {
		return io.ErrClosedPipe
	}

	payload, err := msgpack.Marshal(cmd)
	if err != nil {
		return err
	}

	encoded := cobsEncode(payload)
	frame := append(encoded, 0x00)

	_, err = p.Write(frame)
	return err
}

func (m *Manager) readLoop(p io.Reader, stop chan struct{}) {
	parser := &frameParser{}
	tmp := make([]byte, 256)

	for {
		select {
		case <-stop:
			return
		default:
		}

		n, err := p.Read(tmp)
		if err != nil {
			if err != io.EOF {
				log.Printf("serial read error: %v", err)
			}
			return
		}

		before := *parser
		frames := parser.feed(tmp[:n])
		m.discarded.Add(uint64(parser.discarded - before.discarded))
		m.rejected.Add(uint64(parser.rejected - before.rejected))
		m.crcErrors.Add(uint64(parser.crcErrors - before.crcErrors))
		m.lost.Add(uint64(parser.lost - before.lost))

		for _, fd := range frames {
			if fd.kind == KindSlow {
				m.slowFrames.Add(1)
			} else {
				m.frames.Add(1)
			}
			ts := float64(time.Now().UnixMilli()) / 1000.0
			select {
			case m.Sink <- Packet{Ts: ts, Kind: fd.kind, Data: fd.data}:
			default:
				// drop if consumer is slow
				m.dropped.Add(1)
			}
		}
	}
}

// ListPorts returns available serial port names, followed by the simulator.
func ListPorts() ([]string, error) {
	ports, err := goserial.GetPortsList()
	return append(ports, SimulatorPort), err
}
