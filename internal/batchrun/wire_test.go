package batchrun_test

import (
	"encoding/json"
	"testing"
	"time"

	"belochka/internal/batchrun"
	"belochka/internal/model"
)

// TestEventWireFormat pins the batch run WebSocket wire contract: field
// names, omitempty behavior, and the fixed-width timestamp format. The
// frontend compares finished_at lexicographically, so fractional seconds
// must never leak into the frame.
func TestEventWireFormat(t *testing.T) {
	code := 3
	finished := time.Date(2026, 8, 3, 12, 1, 5, 0, time.UTC)
	ev := batchrun.Event{
		Type:       "status",
		ServerID:   "s1",
		Status:     model.ResultFailed,
		ExitCode:   &code,
		Truncated:  true,
		Error:      "exit status 3",
		FinishedAt: (*batchrun.EventTime)(&finished),
	}
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal status event: %v", err)
	}
	expected := `{"type":"status","server_id":"s1","status":"failed","exit_code":3,"truncated":true,"error":"exit status 3","finished_at":"2026-08-03T12:01:05Z"}`
	if string(data) != expected {
		t.Fatalf("status frame mismatch:\n got %s\nwant %s", data, expected)
	}

	// Output frames carry only the stream fields — status, timestamps and
	// friends must be omitted when empty.
	out := batchrun.Event{Type: "output", ServerID: "s1", Data: "hello"}
	data, err = json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal output event: %v", err)
	}
	expectedOut := `{"type":"output","server_id":"s1","data":"hello"}`
	if string(data) != expectedOut {
		t.Fatalf("output frame mismatch:\n got %s\nwant %s", data, expectedOut)
	}

	// Timestamps must not gain fractional seconds from time.Time's default
	// RFC 3339Nano marshaling.
	nanos := finished.Add(123456789 * time.Nanosecond)
	ev.FinishedAt = (*batchrun.EventTime)(&nanos)
	data, err = json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal with nanos: %v", err)
	}
	expectedNanos := `{"type":"status","server_id":"s1","status":"failed","exit_code":3,"truncated":true,"error":"exit status 3","finished_at":"2026-08-03T12:01:05Z"}`
	if string(data) != expectedNanos {
		t.Fatalf("nanos frame mismatch:\n got %s\nwant %s", data, expectedNanos)
	}
}
