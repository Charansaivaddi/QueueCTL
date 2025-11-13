package storage

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

	"github.com/CharanSaiVaddi/queuectl-backend/internal/job"
)

// Storage provides persistence for jobs.
type Storage interface {
	Init(path string) error
	Close() error
	SaveJob(j *job.Job) error
	ClaimNextPending(now time.Time) (*job.Job, error)
	UpdateJob(j *job.Job) error
	ListByState(state string) ([]*job.Job, error)
	MoveToDead(j *job.Job) error
}

type SQLiteStorage struct {
	db *sql.DB
}

func NewSQLiteStorage() *SQLiteStorage { return &SQLiteStorage{} }

func (s *SQLiteStorage) Init(path string) error {
	if path == "" {
		path = "queue.db"
	}
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000")
	if err != nil {
		return err
	}
	s.db = db
	return s.migrate()
}

func (s *SQLiteStorage) migrate() error {
	q := `
	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		command TEXT,
		state TEXT,
		attempts INTEGER,
		max_retries INTEGER,
		created_at DATETIME,
		updated_at DATETIME,
		next_run_at DATETIME,
		last_error TEXT
	);
	`
	_, err := s.db.Exec(q)
	return err
}

func (s *SQLiteStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *SQLiteStorage) SaveJob(j *job.Job) error {
	if j.ID == "" {
		j.ID = uuid.New().String()
	}
	if j.State == "" {
		j.State = job.StatePending
	}
	now := time.Now().UTC()
	// preserve CreatedAt if set
	if j.CreatedAt.IsZero() {
		j.CreatedAt = now
	}
	j.UpdatedAt = now
	_, err := s.db.Exec(`INSERT INTO jobs(id,command,state,attempts,max_retries,created_at,updated_at,next_run_at,last_error) VALUES(?,?,?,?,?,?,?,?,?)`,
		j.ID, j.Command, j.State, j.Attempts, j.MaxRetries, j.CreatedAt, j.UpdatedAt, j.NextRunAt, j.LastError)
	if err != nil {
		return s.UpdateJob(j)
	}
	return nil
}

func (s *SQLiteStorage) ClaimNextPending(now time.Time) (*job.Job, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRow(`SELECT id,command,state,attempts,max_retries,created_at,updated_at,next_run_at,last_error FROM jobs WHERE state = ? AND (next_run_at IS NULL OR next_run_at <= ?) ORDER BY created_at LIMIT 1`, job.StatePending, now)
	j := &job.Job{}
	var createdAt, updatedAt, nextRun sql.NullTime
	if err := row.Scan(&j.ID, &j.Command, &j.State, &j.Attempts, &j.MaxRetries, &createdAt, &updatedAt, &nextRun, &j.LastError); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if createdAt.Valid {
		j.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		j.UpdatedAt = updatedAt.Time
	}
	if nextRun.Valid {
		j.NextRunAt = nextRun.Time
	}

	res, err := tx.Exec(`UPDATE jobs SET state = ?, updated_at = ? WHERE id = ? AND state = ?`, job.StateProcessing, time.Now().UTC(), j.ID, job.StatePending)
	if err != nil {
		return nil, err
	}
	aff, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if aff == 0 {
		return nil, nil
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	j.State = job.StateProcessing
	j.UpdatedAt = time.Now().UTC()
	return j, nil
}

func (s *SQLiteStorage) UpdateJob(j *job.Job) error {
	j.UpdatedAt = time.Now().UTC()
	_, err := s.db.Exec(`UPDATE jobs SET command=?, state=?, attempts=?, max_retries=?, updated_at=?, next_run_at=?, last_error=? WHERE id=?`,
		j.Command, j.State, j.Attempts, j.MaxRetries, j.UpdatedAt, j.NextRunAt, j.LastError, j.ID)
	return err
}

func (s *SQLiteStorage) ListByState(state string) ([]*job.Job, error) {
	rows, err := s.db.Query(`SELECT id,command,state,attempts,max_retries,created_at,updated_at,next_run_at,last_error FROM jobs WHERE state = ?`, state)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*job.Job
	for rows.Next() {
		j := &job.Job{}
		var createdAt, updatedAt, nextRun sql.NullTime
		if err := rows.Scan(&j.ID, &j.Command, &j.State, &j.Attempts, &j.MaxRetries, &createdAt, &updatedAt, &nextRun, &j.LastError); err != nil {
			return nil, err
		}
		if createdAt.Valid {
			j.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			j.UpdatedAt = updatedAt.Time
		}
		if nextRun.Valid {
			j.NextRunAt = nextRun.Time
		}
		out = append(out, j)
	}
	return out, nil
}

func (s *SQLiteStorage) MoveToDead(j *job.Job) error {
	j.State = job.StateDead
	return s.UpdateJob(j)
}

func (s *SQLiteStorage) GetJobByID(id string) (*job.Job, error) {
	row := s.db.QueryRow(`SELECT id,command,state,attempts,max_retries,created_at,updated_at,next_run_at,last_error FROM jobs WHERE id = ?`, id)
	j := &job.Job{}
	var createdAt, updatedAt, nextRun sql.NullTime
	if err := row.Scan(&j.ID, &j.Command, &j.State, &j.Attempts, &j.MaxRetries, &createdAt, &updatedAt, &nextRun, &j.LastError); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("job not found")
		}
		return nil, err
	}
	if createdAt.Valid {
		j.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		j.UpdatedAt = updatedAt.Time
	}
	if nextRun.Valid {
		j.NextRunAt = nextRun.Time
	}
	return j, nil
}

func (s *SQLiteStorage) RetryJob(id string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`UPDATE jobs SET state=?, attempts=?, updated_at=?, next_run_at=? WHERE id=?`, job.StatePending, 0, now, now, id)
	return err
}
