package store

import (
	"context"
	"database/sql"
	"fmt"

	"belochka/internal/model"
)

const createBatchRunsTable = `
CREATE TABLE IF NOT EXISTS batch_runs (
	id         TEXT PRIMARY KEY,
	script     TEXT NOT NULL,
	status     TEXT NOT NULL,
	created_at DATETIME NOT NULL
);`

const createBatchRunResultsTable = `
CREATE TABLE IF NOT EXISTS batch_run_results (
	run_id      TEXT NOT NULL,
	server_id   TEXT NOT NULL,
	status      TEXT NOT NULL,
	exit_code   INTEGER,
	output      TEXT NOT NULL DEFAULT '',
	truncated   INTEGER NOT NULL DEFAULT 0,
	error       TEXT NOT NULL DEFAULT '',
	started_at  DATETIME,
	finished_at DATETIME,
	PRIMARY KEY (run_id, server_id)
);`

// ReplaceBatchRun atomically replaces any existing Batch Run with the given
// run and results. Returns model.ErrBatchRunInProgress when a run is still
// running — runs are serialized, so a new dispatch may only replace a done run.
func (s *SQLiteStore) ReplaceBatchRun(ctx context.Context, run model.BatchRun, results []model.RunResult) (model.BatchRun, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.BatchRun{}, fmt.Errorf("begin batch replace: %w", err)
	}
	defer tx.Rollback()

	// Reject replacement while the most recent run is still running.
	var status string
	err = tx.QueryRowContext(ctx,
		`SELECT status FROM batch_runs ORDER BY created_at DESC, rowid DESC LIMIT 1`).Scan(&status)
	if err == nil && status == string(model.BatchRunRunning) {
		return model.BatchRun{}, model.ErrBatchRunInProgress
	}
	if err != nil && err != sql.ErrNoRows {
		return model.BatchRun{}, fmt.Errorf("check batch run status: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM batch_run_results`); err != nil {
		return model.BatchRun{}, fmt.Errorf("clear batch results: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM batch_runs`); err != nil {
		return model.BatchRun{}, fmt.Errorf("clear batch runs: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO batch_runs (id, script, status, created_at) VALUES (?, ?, ?, ?)`,
		run.ID, run.Script, string(run.Status), run.CreatedAt); err != nil {
		return model.BatchRun{}, fmt.Errorf("insert batch run: %w", err)
	}

	for _, r := range results {
		if err := insertRunResult(ctx, tx, r); err != nil {
			return model.BatchRun{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return model.BatchRun{}, fmt.Errorf("commit batch replace: %w", err)
	}
	return run, nil
}

// GetBatchRun returns the most recent Batch Run, or model.ErrBatchRunNotFound
// when none has ever been dispatched.
func (s *SQLiteStore) GetBatchRun(ctx context.Context) (model.BatchRun, error) {
	var run model.BatchRun
	var status string

	err := s.db.QueryRowContext(ctx,
		`SELECT id, script, status, created_at FROM batch_runs ORDER BY created_at DESC, rowid DESC LIMIT 1`,
	).Scan(&run.ID, &run.Script, &status, &run.CreatedAt)
	if err == sql.ErrNoRows {
		return model.BatchRun{}, model.ErrBatchRunNotFound
	}
	if err != nil {
		return model.BatchRun{}, fmt.Errorf("query batch run: %w", err)
	}
	run.Status = model.BatchRunStatus(status)
	return run, nil
}

// ListBatchRunResults returns a run's per-Server results in dispatch order.
func (s *SQLiteStore) ListBatchRunResults(ctx context.Context, runID string) ([]model.RunResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT run_id, server_id, status, exit_code, output, truncated, error, started_at, finished_at
		 FROM batch_run_results WHERE run_id = ? ORDER BY rowid ASC`, runID)
	if err != nil {
		return nil, fmt.Errorf("query batch results: %w", err)
	}
	defer rows.Close()

	var results []model.RunResult
	for rows.Next() {
		var r model.RunResult
		var status string
		var truncated int
		if err := rows.Scan(&r.RunID, &r.ServerID, &status, &r.ExitCode, &r.Output, &truncated, &r.Error, &r.StartedAt, &r.FinishedAt); err != nil {
			return nil, fmt.Errorf("scan batch result: %w", err)
		}
		r.Status = model.RunResultStatus(status)
		r.Truncated = truncated != 0
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate batch results: %w", err)
	}
	return results, nil
}

// UpdateBatchRunResult upserts a single Run Result, carrying the run's latest
// in-memory state (status transitions and streamed output) to the database so
// a page refresh mid-run shows live progress.
func (s *SQLiteStore) UpdateBatchRunResult(ctx context.Context, r model.RunResult) error {
	if err := insertRunResult(ctx, s.db, r); err != nil {
		return err
	}
	return nil
}

func insertRunResult(ctx context.Context, exec interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}, r model.RunResult) error {
	_, err := exec.ExecContext(ctx,
		`INSERT INTO batch_run_results (run_id, server_id, status, exit_code, output, truncated, error, started_at, finished_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (run_id, server_id) DO UPDATE SET
		   status = excluded.status,
		   exit_code = excluded.exit_code,
		   output = excluded.output,
		   truncated = excluded.truncated,
		   error = excluded.error,
		   started_at = excluded.started_at,
		   finished_at = excluded.finished_at`,
		r.RunID, r.ServerID, string(r.Status), r.ExitCode, r.Output, boolToInt(r.Truncated), r.Error, r.StartedAt, r.FinishedAt)
	if err != nil {
		return fmt.Errorf("upsert batch result: %w", err)
	}
	return nil
}

// SetBatchRunDone marks a run as done. Called after all per-Server executions
// have finalized.
func (s *SQLiteStore) SetBatchRunDone(ctx context.Context, runID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE batch_runs SET status = ? WHERE id = ?`, string(model.BatchRunDone), runID)
	if err != nil {
		return fmt.Errorf("mark batch run done: %w", err)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
