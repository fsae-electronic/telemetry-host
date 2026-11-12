package store

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Run is a recording session. The free-text metadata fields (driver … notes)
// are filled in by the user and used to search and compare runs.
type Run struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	StartedAt float64  `json:"started_at"`
	EndedAt   *float64 `json:"ended_at"`
	Driver    string   `json:"driver"`
	Mode      string   `json:"mode"`
	Track     string   `json:"track"`
	Weather   string   `json:"weather"`
	Setup     string   `json:"setup"`
	Notes     string   `json:"notes"`
	Markers   int      `json:"markers"`
}

// RunPatch holds the metadata fields to change; nil fields are left as is.
type RunPatch struct {
	Name    *string `json:"name"`
	Driver  *string `json:"driver"`
	Mode    *string `json:"mode"`
	Track   *string `json:"track"`
	Weather *string `json:"weather"`
	Setup   *string `json:"setup"`
	Notes   *string `json:"notes"`
}

// Marker is a labelled point in time inside a run.
type Marker struct {
	ID    int64   `json:"id"`
	RunID int64   `json:"run_id"`
	Ts    float64 `json:"ts"`
	Label string  `json:"label"`
}

type Sample struct {
	Ts      float64 `json:"ts"`
	Channel string  `json:"channel"`
	Value   float64 `json:"value"`
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer

	s := &Store{db: db}
	return s, s.migrate()
}

// metaColumns are the runs columns added after the first release; migrate
// adds any that an older database is missing.
var metaColumns = []string{"driver", "mode", "track", "weather", "setup"}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS runs (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT    NOT NULL,
			started_at REAL    NOT NULL,
			ended_at   REAL,
			notes      TEXT    NOT NULL DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS samples (
			id      INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id  INTEGER NOT NULL REFERENCES runs(id),
			ts      REAL    NOT NULL,
			channel TEXT    NOT NULL,
			value   REAL    NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_samples_run ON samples(run_id, ts);
		CREATE TABLE IF NOT EXISTS markers (
			id     INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id INTEGER NOT NULL REFERENCES runs(id),
			ts     REAL    NOT NULL,
			label  TEXT    NOT NULL DEFAULT ''
		);
		CREATE INDEX IF NOT EXISTS idx_markers_run ON markers(run_id, ts);
	`)
	if err != nil {
		return err
	}

	have := map[string]bool{}
	rows, err := s.db.Query(`SELECT name FROM pragma_table_info('runs')`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		have[name] = true
	}
	rows.Close()
	for _, col := range metaColumns {
		if have[col] {
			continue
		}
		if _, err := s.db.Exec(`ALTER TABLE runs ADD COLUMN ` + col + ` TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add column %s: %w", col, err)
		}
	}

	// user_version 1: runs recorded with the v2 frame stored driver_dc_voltage
	// in 0.1 V units; the v3 decoder publishes volts, so rescale them once.
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version < 1 {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE samples SET value = value * 0.1
			WHERE channel IN ('drv0_driver_dc_voltage', 'drv1_driver_dc_voltage')`); err != nil {
			tx.Rollback()
			return fmt.Errorf("rescale dc voltage: %w", err)
		}
		if _, err := tx.Exec(`PRAGMA user_version = 1`); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// CreateRun inserts a run from r's name and metadata; ID, EndedAt and
// Markers are ignored.
func (s *Store) CreateRun(r Run) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO runs (name, started_at, driver, mode, track, weather, setup, notes)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Name, r.StartedAt, r.Driver, r.Mode, r.Track, r.Weather, r.Setup, r.Notes,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) StopRun(id int64, endedAt float64) error {
	_, err := s.db.Exec(`UPDATE runs SET ended_at = ? WHERE id = ?`, endedAt, id)
	return err
}

// UpdateRun applies the non-nil fields of p to run id.
func (s *Store) UpdateRun(id int64, p RunPatch) error {
	var sets []string
	var args []any
	add := func(col string, v *string) {
		if v != nil {
			sets = append(sets, col+" = ?")
			args = append(args, *v)
		}
	}
	add("name", p.Name)
	add("driver", p.Driver)
	add("mode", p.Mode)
	add("track", p.Track)
	add("weather", p.Weather)
	add("setup", p.Setup)
	add("notes", p.Notes)
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	_, err := s.db.Exec(`UPDATE runs SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	return err
}

const runColumns = `id, name, started_at, ended_at, driver, mode, track, weather, setup, notes,
	(SELECT COUNT(*) FROM markers m WHERE m.run_id = runs.id)`

func scanRun(sc interface{ Scan(...any) error }) (Run, error) {
	var r Run
	err := sc.Scan(&r.ID, &r.Name, &r.StartedAt, &r.EndedAt,
		&r.Driver, &r.Mode, &r.Track, &r.Weather, &r.Setup, &r.Notes, &r.Markers)
	return r, err
}

func (s *Store) ListRuns() ([]Run, error) {
	rows, err := s.db.Query(`SELECT ` + runColumns + ` FROM runs ORDER BY started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

func (s *Store) GetRun(id int64) (*Run, error) {
	r, err := scanRun(s.db.QueryRow(`SELECT `+runColumns+` FROM runs WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &r, err
}

// InsertSamples bulk-inserts a batch of samples for a run.
func (s *Store) InsertSamples(runID int64, ts float64, data map[string]float64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO samples (run_id, ts, channel, value) VALUES (?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for ch, val := range data {
		if _, err := stmt.Exec(runID, ts, ch, val); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// GetSamples returns all samples for a run, pivoted per timestamp.
// Returns a slice of {ts, channel→value} maps for easy JSON serialization.
func (s *Store) GetSamples(runID int64) ([]map[string]any, error) {
	rows, err := s.db.Query(
		`SELECT ts, channel, value FROM samples WHERE run_id = ? ORDER BY ts ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Group by timestamp
	type kv struct {
		ts  float64
		ch  string
		val float64
	}
	var flat []kv
	for rows.Next() {
		var r kv
		if err := rows.Scan(&r.ts, &r.ch, &r.val); err != nil {
			return nil, err
		}
		flat = append(flat, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Pivot: group rows with the same ts into one map
	var result []map[string]any
	var cur map[string]any
	var curTs float64

	for _, r := range flat {
		if cur == nil || r.ts != curTs {
			cur = map[string]any{"ts": r.ts}
			result = append(result, cur)
			curTs = r.ts
		}
		cur[r.ch] = r.val
	}
	return result, nil
}

func (s *Store) DeleteRun(id int64) error {
	_, err := s.db.Exec(`DELETE FROM samples WHERE run_id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete samples: %w", err)
	}
	_, err = s.db.Exec(`DELETE FROM markers WHERE run_id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete markers: %w", err)
	}
	_, err = s.db.Exec(`DELETE FROM runs WHERE id = ?`, id)
	return err
}

func (s *Store) ListMarkers(runID int64) ([]Marker, error) {
	rows, err := s.db.Query(
		`SELECT id, run_id, ts, label FROM markers WHERE run_id = ? ORDER BY ts ASC`, runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Marker
	for rows.Next() {
		var m Marker
		if err := rows.Scan(&m.ID, &m.RunID, &m.Ts, &m.Label); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) AddMarker(runID int64, ts float64, label string) (Marker, error) {
	res, err := s.db.Exec(
		`INSERT INTO markers (run_id, ts, label) VALUES (?, ?, ?)`, runID, ts, label,
	)
	if err != nil {
		return Marker{}, err
	}
	id, err := res.LastInsertId()
	return Marker{ID: id, RunID: runID, Ts: ts, Label: label}, err
}

// UpdateMarker changes a marker's label and/or time; nil fields are kept.
// It reports false if the marker does not belong to runID.
func (s *Store) UpdateMarker(runID, id int64, ts *float64, label *string) (bool, error) {
	res, err := s.db.Exec(
		`UPDATE markers SET ts = COALESCE(?, ts), label = COALESCE(?, label)
		 WHERE id = ? AND run_id = ?`,
		ts, label, id, runID,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (s *Store) DeleteMarker(runID, id int64) error {
	_, err := s.db.Exec(`DELETE FROM markers WHERE id = ? AND run_id = ?`, id, runID)
	return err
}
