package serial

import (
	"bytes"
	"io"
	"log"
	"sync"
	"time"

	"github.com/vmihailenco/msgpack/v5"
	goserial "go.bug.st/serial"
)

// Packet is a decoded telemetry frame with a server-side timestamp.
type Packet struct {
	Ts   float64            `json:"ts"`
	Data map[string]float64 `json:"data"`
}

// Manager owns the serial port lifecycle and publishes decoded packets.
type Manager struct {
	mu      sync.Mutex
	port    goserial.Port
	portName string

	// Sink receives every successfully decoded packet.
	Sink chan Packet

	stopCh chan struct{}
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

	mode := &goserial.Mode{BaudRate: baud}
	p, err := goserial.Open(portName, mode)
	if err != nil {
		return err
	}

	m.port = p
	m.portName = portName
	m.stopCh = make(chan struct{})

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

func (m *Manager) readLoop(p goserial.Port, stop chan struct{}) {
	buf := &bytes.Buffer{}
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

		buf.Write(tmp[:n])

		for {
			frame, rest, found := bytes.Cut(buf.Bytes(), []byte{0x00})
			if !found {
				break
			}
			buf.Reset()
			buf.Write(rest)

			if len(frame) == 0 {
				continue
			}

			decoded, err := cobsDecode(frame)
			if err != nil {
				// Suppress at info level — partial frames are normal at connect time
				continue
			}

			var raw map[string]any
			if err := msgpack.Unmarshal(decoded, &raw); err != nil {
				continue
			}

			data := make(map[string]float64, len(raw))
			for k, v := range raw {
				key := k
				switch fv := v.(type) {
				case float64:
					data[key] = fv
				case float32:
					data[key] = float64(fv)
				case int8:
					data[key] = float64(fv)
				case int16:
					data[key] = float64(fv)
				case int32:
					data[key] = float64(fv)
				case int64:
					data[key] = float64(fv)
				case uint8:
					data[key] = float64(fv)
				case uint16:
					data[key] = float64(fv)
				case uint32:
					data[key] = float64(fv)
				case uint64:
					data[key] = float64(fv)
				}
			}

			ts := float64(time.Now().UnixMilli()) / 1000.0
			select {
			case m.Sink <- Packet{Ts: ts, Data: data}:
			default:
				// drop if consumer is slow
			}
		}
	}
}

// ListPorts returns available serial port names.
func ListPorts() ([]string, error) {
	return goserial.GetPortsList()
}
