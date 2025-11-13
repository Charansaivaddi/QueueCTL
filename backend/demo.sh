#!/usr/bin/env bash
set -euo pipefail
BASE_DIR="$(cd "$(dirname "$0")" && pwd)"

printf "Enqueue failing job (will retry)\n"
cd "$BASE_DIR"
go run ./cmd/queuectl enqueue --command "bash -c 'exit 2'" --max-retries 2
printf "Enqueue succeeding job\n"
go run ./cmd/queuectl enqueue --command "echo ok" --max-retries 1

printf "Run workers for 6 seconds to process jobs\n"

go run ./cmd/queuectl worker --count 2 --duration 6

printf "List completed jobs:\n"
go run ./cmd/queuectl list --state completed

printf "List pending jobs:\n"
go run ./cmd/queuectl list --state pending

printf "List dead (DLQ) jobs:\n"
go run ./cmd/queuectl dlq list

printf "Demo finished\n"
#!/usr/bin/env bash
set -euo pipefail
BASE_DIR="$(cd "$(dirname "$0")" && pwd)"

printf "Enqueue failing job (will retry)\n"
cd "$BASE_DIR"
go run ./cmd/queuectl enqueue --command "bash -c 'exit 2'" --max-retries 2
printf "Enqueue succeeding job\n"
go run ./cmd/queuectl enqueue --command "echo ok" --max-retries 1

printf "Run workers for 6 seconds to process jobs\n"

go run ./cmd/queuectl worker --count 2 --duration 6

printf "List completed jobs:\n"
go run ./cmd/queuectl list --state completed

printf "List pending jobs:\n"
go run ./cmd/queuectl list --state pending

printf "List dead (DLQ) jobs:\n"
go run ./cmd/queuectl dlq list

printf "Demo finished\n"
