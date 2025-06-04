# Amp Orchestrator

A Go application to orchestrate and manage multiple amp CLI worker instances.

## Features

- **Start Workers**: Create new amp threads and start worker processes
- **Stop Workers**: Gracefully terminate worker processes
- **Continue Workers**: Send messages to existing workers
- **List Workers**: View all active and stopped workers
- **Logging**: Each worker maintains its own log file

## Installation

```bash
go mod tidy
go build -o amp-orchestrator
```

## Usage

### Start a new worker
```bash
./amp-orchestrator start --message "Hello, start processing this task"
# Output: Started worker a1b2c3d4 with thread T-bf92dd48-a4ef-48e9-bec2-c23988381844 (PID: 12345)
```

### Send message to existing worker
```bash
./amp-orchestrator continue --worker a1b2c3d4 --message "Continue with this new task"
```

### Stop a worker
```bash
./amp-orchestrator stop --worker a1b2c3d4
```

### List all workers
```bash
./amp-orchestrator list
```

## Architecture

The orchestrator manages:
- **Worker State**: Stored in `logs/workers.json`
- **Log Files**: Each worker gets `logs/worker-{id}.log`
- **Process Management**: Tracks PIDs for clean shutdown
- **Thread Management**: Maps workers to amp thread IDs

## Configuration

- `--log-dir`: Directory for log files (default: `./logs`)
- Assumes `amp` binary is in PATH

## File Structure

```
├── main.go          # CLI interface
├── worker.go        # Worker management logic
├── go.mod          # Go module dependencies
└── logs/           # Generated log directory
    ├── workers.json # Worker state storage
    └── worker-*.log # Individual worker logs
```
