package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func strp(s string) *string { return &s }

// A database created before run metadata and markers existed must be upgraded
// in place without losing its runs.
func TestMigrateOldSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL,
			started_at REAL NOT NULL, ended_at REAL, notes TEXT NOT NULL DEFAULT '');
		CREATE TABLE samples (
			id INTEGER PRIMARY KEY AUTOINCREMENT, run_id INTEGER NOT NULL,
			ts REAL NOT NULL, channel TEXT NOT NULL, value REAL NOT NULL);
		INSERT INTO runs (name, started_at) VALUES ('old run', 100);
		INSERT INTO samples (run_id, ts, channel, value) VALUES
			(1, 100, 'drv0_driver_dc_voltage', 936), (1, 100, 'tps1', 40);`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.GetRun(1)
	if err != nil || r == nil {
		t.Fatalf("GetRun: %v %v", r, err)
	}
	if r.Name != "old run" || r.Driver != "" || r.Markers != 0 {
		t.Errorf("got %+v", r)
	}

	// v2 DC voltage (0.1 V units) is rescaled to volts exactly once.
	for i := 0; i < 2; i++ {
		samples, err := s.GetSamples(1)
		if err != nil || len(samples) != 1 {
			t.Fatalf("GetSamples: %v %v", samples, err)
		}
		if v := samples[0]["drv0_driver_dc_voltage"].(float64); v < 93.59 || v > 93.61 {
			t.Errorf("open %d: dc voltage = %v, want 93.6", i+1, v)
		}
		if samples[0]["tps1"] != 40.0 {
			t.Errorf("tps1 changed: %v", samples[0]["tps1"])
		}
		if s, err = Open(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunMetadataAndMarkers(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.CreateRun(Run{Name: "r", StartedAt: 10, Driver: "Ana", Mode: "race"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateRun(id, RunPatch{Track: strp("Autódromo"), Notes: strp("lluvia")}); err != nil {
		t.Fatal(err)
	}

	m1, _ := s.AddMarker(id, 12.5, "salida")
	if _, err := s.AddMarker(id, 11, "boxes"); err != nil {
		t.Fatal(err)
	}
	ok, err := s.UpdateMarker(id, m1.ID, nil, strp("largada"))
	if err != nil || !ok {
		t.Fatalf("UpdateMarker: %v %v", ok, err)
	}
	if ok, _ := s.UpdateMarker(id+1, m1.ID, nil, strp("x")); ok {
		t.Error("updated a marker through the wrong run")
	}

	r, _ := s.GetRun(id)
	if r.Driver != "Ana" || r.Mode != "race" || r.Track != "Autódromo" || r.Notes != "lluvia" || r.Markers != 2 {
		t.Errorf("run = %+v", r)
	}

	ms, _ := s.ListMarkers(id)
	if len(ms) != 2 || ms[0].Label != "boxes" || ms[1].Label != "largada" || ms[1].Ts != 12.5 {
		t.Errorf("markers = %+v", ms)
	}

	if err := s.DeleteRun(id); err != nil {
		t.Fatal(err)
	}
	if ms, _ := s.ListMarkers(id); len(ms) != 0 {
		t.Errorf("markers left after DeleteRun: %+v", ms)
	}
}
