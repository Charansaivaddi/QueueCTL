package worker

import (
	"os"
	"testing"
	"time"

	"github.com/CharanSaiVaddi/queuectl-backend/internal/job"
	"github.com/CharanSaiVaddi/queuectl-backend/internal/storage"
)

func newTestStorage(t *testing.T) *storage.SQLiteStorage {
	t.Helper()
	f, err := os.CreateTemp("", "queue_test_*.db")
	if err != nil {
		t.Fatalf("tmp file: %v", err)
	}
	path := f.Name()
	f.Close()
	t.Cleanup(func() { os.Remove(path) })

	s := storage.NewSQLiteStorage()
	if err := s.Init(path); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestWorkerHappyPath(t *testing.T) {
	s := newTestStorage(t)
	j := &job.Job{Command: "echo hi", MaxRetries: 1}
	if err := s.SaveJob(j); err != nil {
		t.Fatalf("save job: %v", err)
	}

	got, err := s.ClaimNextPending(time.Now().UTC())
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if got == nil {
		t.Fatal("expected job")
	}

	w := NewWorker(1, s, &Config{BaseBackoff: 2, MaxRetries: 3})
	w.runJob(got)

	g2, err := s.GetJobByID(got.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if g2.State != job.StateCompleted {
		t.Fatalf("expected completed, got %s", g2.State)
	}
}

func TestRetryToDLQ(t *testing.T) {
	s := newTestStorage(t)
	j := &job.Job{Command: "bash -c 'exit 2'", MaxRetries: 2}
	if err := s.SaveJob(j); err != nil {
		t.Fatalf("save job: %v", err)
	}

	w := NewWorker(1, s, &Config{BaseBackoff: 1, MaxRetries: 2})

	for {
		got, err := s.ClaimNextPending(time.Now().UTC())
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if got == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		w.runJob(got)

		cur, err := s.GetJobByID(got.ID)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		if cur.State == job.StateDead {
			break
		}
		cur.NextRunAt = time.Now().UTC()
		cur.State = job.StatePending
		if err := s.UpdateJob(cur); err != nil {
			t.Fatalf("update: %v", err)
		}
	}

	final, err := s.GetJobByID(j.ID)
	if err != nil {
		t.Fatalf("final get: %v", err)
	}
	if final.State != job.StateDead {
		t.Fatalf("expected dead, got %s", final.State)
	}
}
