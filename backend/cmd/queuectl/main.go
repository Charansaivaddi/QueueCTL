package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/CharanSaiVaddi/queuectl-backend/internal/config"
	"github.com/CharanSaiVaddi/queuectl-backend/internal/job"
	"github.com/CharanSaiVaddi/queuectl-backend/internal/storage"
	"github.com/CharanSaiVaddi/queuectl-backend/internal/worker"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cfg := config.Default()
	if c, err := config.Load("queue_config.json"); err == nil {
		cfg = c
	}

	store := storage.NewSQLiteStorage()
	if err := store.Init(cfg.DBPath); err != nil {
		fmt.Fprintln(os.Stderr, "failed to init storage:", err)
		os.Exit(2)
	}
	defer store.Close()

	cmd := os.Args[1]
	switch cmd {
	case "enqueue":
		enqueueCmd(store, cfg)
	case "worker":
		workerCmd(store, cfg)
	case "list":
		listCmd(store)
	case "dlq":
		dlqCmd(store)
	case "config":
		configCmd(cfg)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("queuectl - backend CLI")
	fmt.Println("usage: queuectl <command> [options]")
	fmt.Println("commands: enqueue, worker, list, dlq, config")
}

func enqueueCmd(store *storage.SQLiteStorage, cfg *config.Config) {
	fs := flag.NewFlagSet("enqueue", flag.ExitOnError)
	jobJSON := fs.String("job", "", "Full job JSON payload")
	command := fs.String("command", "", "Command to run")
	maxRetries := fs.Int("max-retries", cfg.MaxRetries, "Max retries")
	fs.Parse(os.Args[2:])

	var j job.Job
	if *jobJSON != "" {
		if err := json.Unmarshal([]byte(*jobJSON), &j); err != nil {
			fmt.Fprintln(os.Stderr, "invalid job json:", err)
			os.Exit(1)
		}
	} else if *command != "" {
		j = job.Job{Command: *command, MaxRetries: *maxRetries}
	} else {
		fmt.Fprintln(os.Stderr, "provide --job JSON or --command")
		os.Exit(1)
	}
	if j.MaxRetries == 0 {
		j.MaxRetries = cfg.MaxRetries
	}
	if err := store.SaveJob(&j); err != nil {
		fmt.Fprintln(os.Stderr, "failed to save job:", err)
		os.Exit(1)
	}
	fmt.Println("enqueued job", j.ID)
}

func workerCmd(store *storage.SQLiteStorage, cfg *config.Config) {
	fs := flag.NewFlagSet("worker", flag.ExitOnError)
	count := fs.Int("count", 1, "Number of workers to start")
	duration := fs.Int("duration", 0, "Run workers for N seconds then stop (0 = run until SIGINT)")
	fs.Parse(os.Args[2:])

	workers := make([]*worker.Worker, 0, *count)
	wcfg := &worker.Config{BaseBackoff: cfg.BackoffBase, MaxRetries: cfg.MaxRetries}
	for i := 0; i < *count; i++ {
		w := worker.NewWorker(i+1, store, wcfg)
		w.Start()
		workers = append(workers, w)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	if *duration > 0 {
		fmt.Printf("running %d worker(s) for %d seconds...\n", *count, *duration)
		select {
		case <-time.After(time.Duration(*duration) * time.Second):
			fmt.Println("duration elapsed; shutting down workers...")
		case <-sigs:
			fmt.Println("signal received; shutting down workers...")
		}
	} else {
		fmt.Printf("running %d worker(s) until interrupted (Ctrl+C)\n", *count)
		<-sigs
		fmt.Println("signal received; shutting down workers...")
	}

	for _, w := range workers {
		w.Stop()
	}
}

func listCmd(store *storage.SQLiteStorage) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	state := fs.String("state", "pending", "Job state to list")
	fs.Parse(os.Args[2:])
	rows, err := store.ListByState(*state)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list error:", err)
		os.Exit(1)
	}
	for _, j := range rows {
		fmt.Printf("%s \t %s \t attempts=%d state=%s\n", j.ID, j.Command, j.Attempts, j.State)
	}
}

func dlqCmd(store *storage.SQLiteStorage) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "dlq <list|retry> [jobid]")
		os.Exit(1)
	}
	switch os.Args[2] {
	case "list":
		rows, err := store.ListByState(job.StateDead)
		if err != nil {
			fmt.Fprintln(os.Stderr, "dlq list error:", err)
			os.Exit(1)
		}
		for _, j := range rows {
			fmt.Printf("%s \t %s \t attempts=%d last_error=%s\n", j.ID, j.Command, j.Attempts, j.LastError)
		}
	case "retry":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "provide job id to retry")
			os.Exit(1)
		}
		id := os.Args[3]
		if err := store.RetryJob(id); err != nil {
			fmt.Fprintln(os.Stderr, "retry error:", err)
			os.Exit(1)
		}
		fmt.Println("job retried:", id)
	default:
		fmt.Fprintln(os.Stderr, "unknown dlq command", os.Args[2])
		os.Exit(1)
	}
}

func configCmd(cfg *config.Config) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "config set key value | config get")
		os.Exit(1)
	}
	switch os.Args[2] {
	case "set":
		if len(os.Args) < 5 {
			fmt.Fprintln(os.Stderr, "usage: config set <key> <value>")
			os.Exit(1)
		}
		key := os.Args[3]
		val := os.Args[4]
		switch key {
		case "max-retries":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.MaxRetries = v
			}
		case "backoff-base":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.BackoffBase = v
			}
		case "db-path":
			cfg.DBPath = val
		default:
			fmt.Fprintln(os.Stderr, "unknown config key", key)
			os.Exit(1)
		}
		if err := cfg.Save("queue_config.json"); err != nil {
			fmt.Fprintln(os.Stderr, "failed to save config:", err)
			os.Exit(1)
		}
		fmt.Println("config saved")
	case "get":
		b, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Println(string(b))
	default:
		fmt.Fprintln(os.Stderr, "unknown config command", os.Args[2])
		os.Exit(1)
	}
}
