package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"belochka/internal/model"
)

func newBatchTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	return newTestStore(t)
}

func batchRun(id string) model.BatchRun {
	return model.BatchRun{
		ID:        id,
		Script:    "echo hi",
		Status:    model.BatchRunRunning,
		CreatedAt: time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC),
	}
}

func batchResults(runID string, serverIDs ...string) []model.RunResult {
	results := make([]model.RunResult, 0, len(serverIDs))
	for _, id := range serverIDs {
		results = append(results, model.RunResult{RunID: runID, ServerID: id, Status: model.ResultPending})
	}
	return results
}

func TestReplaceBatchRunAndGet(t *testing.T) {
	s := newBatchTestStore(t)
	ctx := context.Background()

	run := batchRun("run-1")
	if _, err := s.ReplaceBatchRun(ctx, run, batchResults("run-1", "s1", "s2")); err != nil {
		t.Fatalf("replace: %v", err)
	}

	got, err := s.GetBatchRun(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != "run-1" || got.Script != "echo hi" || got.Status != model.BatchRunRunning {
		t.Fatalf("unexpected run: %+v", got)
	}

	results, err := s.ListBatchRunResults(ctx, "run-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(results) != 2 || results[0].ServerID != "s1" || results[1].ServerID != "s2" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestReplaceBatchRunRejectsRunningRun(t *testing.T) {
	s := newBatchTestStore(t)
	ctx := context.Background()

	if _, err := s.ReplaceBatchRun(ctx, batchRun("run-1"), batchResults("run-1", "s1")); err != nil {
		t.Fatalf("replace: %v", err)
	}

	_, err := s.ReplaceBatchRun(ctx, batchRun("run-2"), batchResults("run-2", "s2"))
	if !errors.Is(err, model.ErrBatchRunInProgress) {
		t.Fatalf("expected ErrBatchRunInProgress, got %v", err)
	}

	// A done run is replaceable.
	if err := s.SetBatchRunDone(ctx, "run-1"); err != nil {
		t.Fatalf("mark done: %v", err)
	}
	if _, err := s.ReplaceBatchRun(ctx, batchRun("run-2"), batchResults("run-2", "s2")); err != nil {
		t.Fatalf("replace after done: %v", err)
	}
}

func TestReplaceBatchRunReplacesPreviousRecords(t *testing.T) {
	s := newBatchTestStore(t)
	ctx := context.Background()

	run1 := batchRun("run-1")
	if _, err := s.ReplaceBatchRun(ctx, run1, batchResults("run-1", "s1")); err != nil {
		t.Fatalf("replace 1: %v", err)
	}
	if err := s.SetBatchRunDone(ctx, "run-1"); err != nil {
		t.Fatalf("mark done: %v", err)
	}

	run2 := batchRun("run-2")
	if _, err := s.ReplaceBatchRun(ctx, run2, batchResults("run-2", "s2", "s3")); err != nil {
		t.Fatalf("replace 2: %v", err)
	}

	got, err := s.GetBatchRun(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != "run-2" {
		t.Fatalf("expected run-2 as current, got %s", got.ID)
	}
	if _, err := s.ListBatchRunResults(ctx, "run-1"); err != nil {
		t.Fatalf("expected run-1 results gone, got %v", err)
	}
	results, err := s.ListBatchRunResults(ctx, "run-2")
	if err != nil {
		t.Fatalf("list run-2: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestGetBatchRunNotFound(t *testing.T) {
	s := newBatchTestStore(t)
	_, err := s.GetBatchRun(context.Background())
	if !errors.Is(err, model.ErrBatchRunNotFound) {
		t.Fatalf("expected ErrBatchRunNotFound, got %v", err)
	}
}

func TestUpdateBatchRunResultUpsert(t *testing.T) {
	s := newBatchTestStore(t)
	ctx := context.Background()

	run := batchRun("run-1")
	if _, err := s.ReplaceBatchRun(ctx, run, batchResults("run-1", "s1")); err != nil {
		t.Fatalf("replace: %v", err)
	}

	// Status transition to running, then a final success with output.
	started := time.Date(2026, 8, 3, 12, 0, 1, 0, time.UTC)
	if err := s.UpdateBatchRunResult(ctx, model.RunResult{RunID: "run-1", ServerID: "s1", Status: model.ResultRunning, StartedAt: &started}); err != nil {
		t.Fatalf("update running: %v", err)
	}
	finished := started.Add(time.Second)
	code := 0
	if err := s.UpdateBatchRunResult(ctx, model.RunResult{
		RunID: "run-1", ServerID: "s1", Status: model.ResultSuccess,
		ExitCode: &code, Output: "done\n", StartedAt: &started, FinishedAt: &finished,
	}); err != nil {
		t.Fatalf("update success: %v", err)
	}

	results, err := s.ListBatchRunResults(ctx, "run-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	r := results[0]
	if r.Status != model.ResultSuccess || r.Output != "done\n" || r.Truncated || r.ExitCode == nil || *r.ExitCode != 0 {
		t.Fatalf("unexpected result: %+v", r)
	}
	if r.StartedAt == nil || r.FinishedAt == nil {
		t.Fatalf("expected timestamps, got %+v", r)
	}
}
