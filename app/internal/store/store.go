package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Run struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	StartedAt float64  `json:"started_at"`
	EndedAt   *float64 `json:"ended_at"`
	Notes     string   `json:"notes"`
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
	`)
	return err
}

func (s *Store) CreateRun(name string, startedAt float64) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO runs (name, started_at) VALUES (?, ?)`,
		name, startedAt,
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

func (s *Store) ListRuns() ([]Run, error) {
	rows, err := s.db.Query(`SELECT id, name, started_at, ended_at, notes FROM runs ORDER BY started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []Run
	for rows.Next() {
		var r Run
		if err := rows.Scan(&r.ID, &r.Name, &r.StartedAt, &r.EndedAt, &r.Notes); err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

func (s *Store) GetRun(id int64) (*Run, error) {
	var r Run
	err := s.db.QueryRow(
		`SELECT id, name, started_at, ended_at, notes FROM runs WHERE id = ?`, id,
	).Scan(&r.ID, &r.Name, &r.StartedAt, &r.EndedAt, &r.Notes)
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
	_, err = s.db.Exec(`DELETE FROM runs WHERE id = ?`, id)
	return err
}
