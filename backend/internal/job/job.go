package job

import "time"

const (
	StatePending    = "pending"
	StateProcessing = "processing"
	StateCompleted  = "completed"
	StateFailed     = "failed"
	StateDead       = "dead"
)

type Job struct {
	ID         string    `json:"id" db:"id"`
	Command    string    `json:"command" db:"command"`
	State      string    `json:"state" db:"state"`
	Attempts   int       `json:"attempts" db:"attempts"`
	MaxRetries int       `json:"max_retries" db:"max_retries"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
	NextRunAt  time.Time `json:"next_run_at" db:"next_run_at"`
	LastError  string    `json:"last_error" db:"last_error"`
}
