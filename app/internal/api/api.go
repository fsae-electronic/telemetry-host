package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"telemetry-host/internal/hub"
	"telemetry-host/internal/serial"
	"telemetry-host/internal/store"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Server struct {
	serial *serial.Manager
	hub    *hub.Hub
	store  *store.Store

	mu          sync.Mutex
	activeRunID int64 // 0 = not recording
}

func New(sm *serial.Manager, h *hub.Hub, st *store.Store) *Server {
	return &Server{serial: sm, hub: h, store: st}
}

// Shutdown ends any active recording and closes the serial port, so a run is
// not left without an end time when the app quits.
func (s *Server) Shutdown() {
	s.mu.Lock()
	if s.activeRunID != 0 {
		s.store.StopRun(s.activeRunID, nowTs())
		s.activeRunID = 0
	}
	s.mu.Unlock()
	s.serial.Disconnect()
}

// Routes registers all API routes on mux.
func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/ws", s.handleWS)
	mux.HandleFunc("/api/ports", s.handlePorts)
	mux.HandleFunc("/api/connect", s.handleConnect)
	mux.HandleFunc("/api/disconnect", s.handleDisconnect)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/runs", s.handleRuns)
	mux.HandleFunc("/api/runs/", s.handleRunsID)
}

// ConsumePackets reads from serial.Sink, stores if recording, and broadcasts.
// Should be called in its own goroutine.
func (s *Server) ConsumePackets() {
	for p := range s.serial.Sink {
		s.mu.Lock()
		runID := s.activeRunID
		s.mu.Unlock()

		if runID != 0 {
			s.store.InsertSamples(runID, p.Ts, p.Data)
		}

		s.hub.Broadcast(p)
	}
}

// BroadcastLinkStats pushes the serial link counters to all clients once per
// second as {"type": "link"}. Should be called in its own goroutine.
func (s *Server) BroadcastLinkStats() {
	type linkPayload struct {
		Connected bool    `json:"connected"`
		RateHz    float64 `json:"rate_hz"` // frames decoded in the last interval
		serial.LinkStats
	}
	const interval = time.Second
	var prev uint64
	for range time.Tick(interval) {
		st := s.serial.Stats()
		if st.Frames < prev { // counters reset by a reconnect
			prev = 0
		}
		rate := float64(st.Frames-prev) / interval.Seconds()
		prev = st.Frames
		s.hub.BroadcastMsg("link", linkPayload{
			Connected: s.serial.Connected(),
			RateHz:    rate,
			LinkStats: st,
		})
	}
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s.hub.Register(conn)
	// Send current status immediately on connect
	s.broadcastStatus()
}

func (s *Server) handlePorts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ports, err := serial.ListPorts()
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ports == nil {
		ports = []string{}
	}
	jsonOK(w, ports)
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Port string `json:"port"`
		Baud int    `json:"baud"`
	}
	body.Baud = 115200
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.Port == "" {
		jsonError(w, "port required", http.StatusBadRequest)
		return
	}
	if err := s.serial.Connect(body.Port, body.Baud); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.broadcastStatus()
	jsonOK(w, map[string]any{"connected": true, "port": body.Port})
}

func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	if s.activeRunID != 0 {
		ts := float64(time.Now().UnixMilli()) / 1000.0
		s.store.StopRun(s.activeRunID, ts)
		s.activeRunID = 0
	}
	s.mu.Unlock()

	s.serial.Disconnect()
	s.broadcastStatus()
	jsonOK(w, map[string]any{"connected": false})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jsonOK(w, s.currentStatus())
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		runs, err := s.store.ListRuns()
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if runs == nil {
			runs = []store.Run{}
		}
		jsonOK(w, runs)

	case http.MethodPost:
		// Start recording
		var body store.Run
		json.NewDecoder(r.Body).Decode(&body)
		if body.Name == "" {
			body.Name = fmt.Sprintf("Run %s", time.Now().Format("2006-01-02 15:04:05"))
		}
		if s.serial.PortName() == serial.SimulatorPort {
			body.Name = "[SIM] " + body.Name // keep simulated runs apart from real ones
		}
		body.StartedAt = nowTs()
		id, err := s.store.CreateRun(body)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.mu.Lock()
		s.activeRunID = id
		s.mu.Unlock()
		s.broadcastStatus()
		jsonOK(w, map[string]any{"id": id, "name": body.Name})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRunsID(w http.ResponseWriter, r *http.Request) {
	// Path: /api/runs/{id}, /api/runs/{id}/stop, /api/runs/{id}/samples,
	// /api/runs/{id}/markers or /api/runs/{id}/markers/{markerID}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/runs/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		jsonError(w, "invalid run id", http.StatusBadRequest)
		return
	}

	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "stop":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ts := float64(time.Now().UnixMilli()) / 1000.0
		if err := s.store.StopRun(id, ts); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.mu.Lock()
		if s.activeRunID == id {
			s.activeRunID = 0
		}
		s.mu.Unlock()
		s.broadcastStatus()
		jsonOK(w, map[string]any{"stopped": true})

	case "samples":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		samples, err := s.store.GetSamples(id)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if samples == nil {
			samples = []map[string]any{}
		}
		jsonOK(w, samples)

	case "markers":
		s.handleMarkers(w, r, id, parts[2:])

	case "":
		switch r.Method {
		case http.MethodPatch:
			var p store.RunPatch
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				jsonError(w, "invalid body", http.StatusBadRequest)
				return
			}
			if err := s.store.UpdateRun(id, p); err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			run, err := s.store.GetRun(id)
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if run == nil {
				http.NotFound(w, r)
				return
			}
			jsonOK(w, run)

		case http.MethodGet:
			run, err := s.store.GetRun(id)
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if run == nil {
				http.NotFound(w, r)
				return
			}
			jsonOK(w, run)

		case http.MethodDelete:
			if err := s.store.DeleteRun(id); err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			s.mu.Lock()
			if s.activeRunID == id {
				s.activeRunID = 0
			}
			s.mu.Unlock()
			jsonOK(w, map[string]any{"deleted": true})

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}

	default:
		http.NotFound(w, r)
	}
}

// handleMarkers serves /api/runs/{runID}/markers[/{markerID}].
// rest holds the path segments after "markers".
func (s *Server) handleMarkers(w http.ResponseWriter, r *http.Request, runID int64, rest []string) {
	if len(rest) == 0 || rest[0] == "" {
		switch r.Method {
		case http.MethodGet:
			markers, err := s.store.ListMarkers(runID)
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if markers == nil {
				markers = []store.Marker{}
			}
			jsonOK(w, markers)

		case http.MethodPost:
			// ts defaults to now, so a live marker lands on the current sample.
			var body struct {
				Ts    *float64 `json:"ts"`
				Label string   `json:"label"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				jsonError(w, "invalid body", http.StatusBadRequest)
				return
			}
			ts := nowTs()
			if body.Ts != nil {
				ts = *body.Ts
			}
			run, err := s.store.GetRun(runID)
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if run == nil {
				http.NotFound(w, r)
				return
			}
			m, err := s.store.AddMarker(runID, ts, body.Label)
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			jsonOK(w, m)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	markerID, err := strconv.ParseInt(rest[0], 10, 64)
	if err != nil {
		jsonError(w, "invalid marker id", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var body struct {
			Ts    *float64 `json:"ts"`
			Label *string  `json:"label"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "invalid body", http.StatusBadRequest)
			return
		}
		ok, err := s.store.UpdateMarker(runID, markerID, body.Ts, body.Label)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		jsonOK(w, map[string]any{"updated": true})

	case http.MethodDelete:
		if err := s.store.DeleteMarker(runID, markerID); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonOK(w, map[string]any{"deleted": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type statusPayload struct {
	Connected   bool   `json:"connected"`
	Port        string `json:"port"`
	Recording   bool   `json:"recording"`
	ActiveRunID int64  `json:"active_run_id"`
}

func (s *Server) currentStatus() statusPayload {
	s.mu.Lock()
	runID := s.activeRunID
	s.mu.Unlock()
	return statusPayload{
		Connected:   s.serial.Connected(),
		Port:        s.serial.PortName(),
		Recording:   runID != 0,
		ActiveRunID: runID,
	}
}

func (s *Server) broadcastStatus() {
	s.hub.BroadcastStatus(s.currentStatus())
}

func nowTs() float64 { return float64(time.Now().UnixMilli()) / 1000.0 }

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
