package worker

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/CharanSaiVaddi/queuectl-backend/internal/job"
	"github.com/CharanSaiVaddi/queuectl-backend/internal/storage"
)

// Worker runs jobs from storage.
type Worker struct {
	id     int
	store  storage.Storage
	cfg    *Config
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Config for worker behavior
type Config struct {
	BaseBackoff int
	MaxRetries  int
}

func NewWorker(id int, store storage.Storage, cfg *Config) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{id: id, store: store, cfg: cfg, ctx: ctx, cancel: cancel}
}

func (w *Worker) Start() {
	w.wg.Add(1)
	go w.loop()
}

func (w *Worker) loop() {
	defer w.wg.Done()
	for {
		select {
		case <-w.ctx.Done():
			fmt.Printf("worker %d: shutting down\n", w.id)
			return
		default:
			// Claim next job and run it
			j, err := w.store.ClaimNextPending(time.Now().UTC())
			if err != nil {
				fmt.Println("claim error:", err)
				time.Sleep(1 * time.Second)
				continue
			}
			if j == nil {
				// nothing ready
				time.Sleep(500 * time.Millisecond)
				continue
			}
			w.runJob(j)
		}
	}
}

func (w *Worker) runJob(j *job.Job) {
	// execute command
	cmd := exec.Command("/bin/sh", "-lc", j.Command)
	err := cmd.Run()
	if err != nil {
		j.Attempts++
		j.LastError = err.Error()
		maxRetries := j.MaxRetries
		if maxRetries == 0 {
			maxRetries = w.cfg.MaxRetries
		}
		if j.Attempts >= maxRetries {
			j.State = job.StateDead
			w.store.UpdateJob(j)
			fmt.Printf("job %s moved to DLQ\n", j.ID)
			return
		}
		j.State = job.StatePending
		base := w.cfg.BaseBackoff
		if base <= 1 {
			base = 2
		}
		pow := 1
		for i := 0; i < j.Attempts; i++ {
			pow *= base
		}
		wait := time.Duration(pow) * time.Second
		j.NextRunAt = time.Now().UTC().Add(wait)
		w.store.UpdateJob(j)
		fmt.Printf("job %s failed, retry in %s\n", j.ID, wait)
		return
	}
	j.State = job.StateCompleted
	w.store.UpdateJob(j)
	fmt.Printf("job %s completed\n", j.ID)
}

func (w *Worker) Stop() {
	w.cancel()
	w.wg.Wait()
}
