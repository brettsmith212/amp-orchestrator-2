# amp-orchestrator-2

A tool to manage and orchestrate multiple amp CLI worker instances.

## Installation

### From Source

1. Clone the repository:
```bash
git clone https://github.com/brettsmith212/amp-orchestrator-2.git
cd amp-orchestrator-2
```

2. Build the binary:
```bash
go build -o ampd .
```

3. (Optional) Install globally:
```bash
go install .
```

### Prerequisites

- Go 1.21 or higher

## Usage

### Start a new worker
```bash
./ampd start -m "Your initial message" [-l ./logs]
```

### List active workers
```bash
./ampd list
```

### Send message to existing worker
```bash
./ampd continue -w WORKER_ID -m "Your message"
```

### Stop a worker
```bash
./ampd stop -w WORKER_ID
```

## Commands

- `start` - Start a new amp worker instance
- `stop` - Stop an amp worker instance  
- `continue` - Send a message to an existing amp worker
- `list` - List all active amp workers

## Logs

Worker logs are stored in the `./logs` directory by default. You can specify a different directory using the `-l` flag with the `start` command.
