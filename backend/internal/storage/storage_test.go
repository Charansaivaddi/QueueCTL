package storage

import (
	"os"
	"testing"
	"time"

	"github.com/CharanSaiVaddi/queuectl-backend/internal/job"
)

func newTestStorage(t *testing.T) *SQLiteStorage {
	t.Helper()
	f, err := os.CreateTemp("", "queue_test_*.db")
	if err != nil {
		t.Fatalf("tmp file: %v", err)
	}
	path := f.Name()
	f.Close()
	t.Cleanup(func() { os.Remove(path) })

	s := NewSQLiteStorage()
	if err := s.Init(path); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSaveClaimUpdate(t *testing.T) {
	s := newTestStorage(t)
	j := &job.Job{Command: "echo hello", MaxRetries: 1}
	if err := s.SaveJob(j); err != nil {
		t.Fatalf("save job: %v", err)
	}

	// claim
	got, err := s.ClaimNextPending(time.Now().UTC())
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if got == nil {
		t.Fatal("expected job, got nil")
	}
	if got.State != job.StateProcessing {
		t.Fatalf("expected processing, got %s", got.State)
	}

	// mark completed
	got.State = job.StateCompleted
	if err := s.UpdateJob(got); err != nil {
		t.Fatalf("update job: %v", err)
	}

	rows, err := s.ListByState(job.StateCompleted)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 completed job, got %d", len(rows))
	}
}

func TestClaimNoDuplicate(t *testing.T) {
	s := newTestStorage(t)
	j := &job.Job{Command: "echo x", MaxRetries: 1}
	if err := s.SaveJob(j); err != nil {
		t.Fatalf("save job: %v", err)
	}

	got1, err := s.ClaimNextPending(time.Now().UTC())
	if err != nil {
		t.Fatalf("claim1: %v", err)
	}
	if got1 == nil {
		t.Fatal("expected job on first claim")
	}

	got2, err := s.ClaimNextPending(time.Now().UTC())
	if err != nil {
		t.Fatalf("claim2: %v", err)
	}
	if got2 != nil {
		t.Fatalf("expected nil on second claim but got job %s", got2.ID)
	}

	got1.State = job.StateCompleted
	if err := s.UpdateJob(got1); err != nil {
		t.Fatalf("update: %v", err)
	}

	got3, err := s.ClaimNextPending(time.Now().UTC())
	if err != nil {
		t.Fatalf("claim3: %v", err)
	}
	if got3 != nil {
		t.Fatalf("expected nil after completion, got %s", got3.ID)
	}
}
