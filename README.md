# queuectl-backend (Go)

Lightweight background job queue backend and CLI written in Go. It demonstrates:

- a small CLI to enqueue jobs and manage workers
- SQLite-backed persistent queue
- worker pool with retry/exponential backoff and a dead-letter queue (DLQ)

This repository is a scaffold intended for experimentation and simple demos.

Repository structure
- `cmd/queuectl` — CLI entrypoint (enqueue, worker, list, dlq, config)
- `internal/job` — Job model and state constants
- `internal/storage` — Storage interface and SQLite implementation
- `internal/worker` — Worker logic (claim/run/retry)
- `internal/config` — Configuration loader/saver

Quick start (macOS / zsh)

1. Build the CLI

```bash
cd backend
go build ./cmd/queuectl
```

2. Start clean (optional)

```bash
# Remove existing DB to start fresh
rm -f queue.db
```

3. Enqueue a job and run a worker (example)

```bash
./queuectl enqueue --command "echo hello && exit 0"
./queuectl worker --count 1 --duration 15
```

Commands overview

- `enqueue`: add a job (supports `--command` or `--job` JSON)
- `worker`: start worker(s); use `--count` and optional `--duration` (seconds)
- `list --state <pending|processing|completed|dead>`: list jobs by state
- `dlq list|retry <id>`: inspect or retry DLQ entries
- `config set|get`: update runtime config (max retries, backoff base, DB path)

Mermaid High-Level Design (HLD)

Below is a simple mermaid diagram describing the main components and flows.

``` mermaid
flowchart LR
  CLI["CLI (cmd/queuectl)"] -->|enqueue| Storage[(SQLite Storage: jobs table)]
  WorkerPool["Worker Pool (workers)"] -->|claim| Storage
  WorkerPool -->|exec| Shell["/bin/sh -c <command>"]

  Storage -->|"store (next_run_at, attempts)"| DLQ((Dead Letter Queue: state=dead))

  Shell -->|exit 0| StorageCompleted["Storage: mark completed"]
  Shell -->|exit != 0| StorageRetry["Storage: update attempts, next_run_at"]

  StorageRetry -->|attempts >= max| DLQ

  style Storage fill:#f9f,stroke:#333,stroke-width:1px
  style WorkerPool fill:#bbf
  style DLQ fill:#fdd
```

Demo recording : 

1. Building the CLI
2. Enqueuing one successful job and one failing job
3. Running a worker for ~20 seconds and showing retry -> DLQ
4. Listing `completed` and `dead` jobs

Demo Link: https://drive.google.com/file/d/1VpHAGU57gR4XPvdmPdG2Ak-AKvMuCHNF/view?usp=sharing

![Preview](https://drive.google.com/uc?export=view&id=1VpHAGU57gR4XPvdmPdG2Ak-AKvMuCHNF)



Notes & next steps

- The project uses SQLite for persistence. `queue.db` is created in the `backend` folder by default.
- The worker implements exponential backoff using a configurable base and moves jobs to `dead` after max retries.

