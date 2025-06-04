# Implementation Plan

## Project Setup

- [x] Step 1: Bootstrap Go module and directory layout

  - **Task**: Initialise `go.mod`, create `cmd/ampd/main.go` with a “Hello World” HTTP server, and add standard `.gitignore`.
  - **Description**: Provides the foundational project scaffold so every later step can build & run.
  - **Files**:
    - `go.mod`: `module github.com/your-org/ampd`
    - `.gitignore`: Go, logs, editor files
    - `cmd/ampd/main.go`: minimal `http.ListenAndServe(":8080", nil)`
  - **Step Dependencies**: None
  - **User Instructions**: `go run ./cmd/ampd` should start a server on port 8080.

- [x] Step 2: Add basic router, health check, and test tooling
  - **Task**: Introduce `chi` router, `/healthz` endpoint, and install `github.com/stretchr/testify`.
  - **Description**: Gives us one real route and the test library we’ll use throughout.
  - **Files**:
    - `cmd/ampd/main.go`: replace `nil` handler with router
    - `internal/api/router.go`: `NewRouter()` returning \*chi.Mux
    - `internal/api/health.go`: handler returning 200/`"ok"`
    - `internal/api/health_test.go`: `httptest` verifying 200
    - `go.mod/go.sum`: add deps
  - **Step Dependencies**: Step 1
  - **User Instructions**: `go test ./...` passes; `curl :8080/healthz` returns `ok`.

## Worker Manager Extraction

- [x] Step 3: Refactor existing WorkerManager into `internal/worker`
  - **Task**: Move `worker.go` into dedicated package, split into `manager.go` & `types.go`, and add basic unit tests for `StartWorker` (mocked).
  - **Description**: Isolates core process logic for reuse by API layer.
  - **Files**:
    - `internal/worker/manager.go`: moved & package `worker`
    - `internal/worker/types.go`: defines `Worker` struct
    - `internal/worker/manager_test.go`: uses `exec.CommandContext` with dummy script to test status transitions
  - **Step Dependencies**: Step 1
  - **User Instructions**: `go test ./internal/worker` should pass.

## Configuration Layer

- [x] Step 4: Introduce config handling
  - **Task**: Add `pkg/config/config.go` that reads env vars (PORT, AMP_BINARY, LOG_DIR).
  - **Description**: Centralises configuration; makes tests deterministic by overriding env.
  - **Files**:
    - `pkg/config/config.go`: `Load()` returns `Config`
    - `pkg/config/config_test.go`: unit tests env parsing
    - `cmd/ampd/main.go`: use `config.Load()` for port
  - **Step Dependencies**: Steps 1, 2
  - **User Instructions**: can set `PORT=9090 go run ./cmd/ampd`.

## REST API – Tasks

- [x] Step 5: List tasks endpoint (`GET /api/tasks`)

  - **Task**: Add handler that loads workers from `WorkerManager` and returns JSON slice. Include test.
  - **Description**: First “real” business route used by dashboard.
  - **Files**:
    - `internal/api/tasks.go`: `ListTasks(wm *worker.Manager)`
    - `internal/api/tasks_test.go`
    - `internal/api/dto.go`: define `TaskDTO`
  - **Step Dependencies**: Steps 2, 3
  - **User Instructions**: `curl :8080/api/tasks` returns `[]`.

- [ ] Step 6: Start task endpoint (`POST /api/tasks`)

  - **Task**: Parse `{message}` JSON, call `wm.StartWorker`, return created `TaskDTO`; write test using fake binary.
  - **Description**: Enables front-end “New Task” button; lays pattern for mutating endpoints.
  - **Files**:
    - `internal/api/tasks.go`: `StartTask`
    - `internal/api/tasks_test.go`: happy-path + validation error
  - **Step Dependencies**: Step 5
  - **User Instructions**: `curl -XPOST -d'{"message":"hi"}' :8080/api/tasks` returns task JSON.

- [ ] Step 7: Stop and Continue endpoints
  - **Task**: Add `POST /api/tasks/{id}/stop` and `/continue` routes & tests; propagate 404 & 409 codes.
  - **Description**: Completes CRUD set needed by PromptBar actions.
  - **Files**:
    - `internal/api/tasks.go`: `StopTask`, `ContinueTask`
    - `internal/api/tasks_test.go`
  - **Step Dependencies**: Steps 5, 6
  - **User Instructions**: commands return 202 on success.

## WebSocket Hub

- [ ] Step 8: Implement hub skeleton and `/api/ws` endpoint

  - **Task**: Create `internal/hub/hub.go` managing clients & broadcast; handler upgrades to WS.
  - **Description**: Foundation for realtime updates.
  - **Files**:
    - `internal/hub/hub.go`: `Run`, `Broadcast`, `Register`
    - `internal/hub/client.go`: read/write pumps
    - `internal/api/ws.go`: wires hub to router
    - `internal/hub/hub_test.go`: broadcast unit test using in-memory connections
  - **Step Dependencies**: Steps 2, 4
  - **User Instructions**: `wscat -c ws://localhost:8080/api/ws` connects with no error.

- [ ] Step 9: Emit `task-update` events
  - **Task**: Modify task handlers to send `TaskDTO` over hub after state changes; include list watcher goroutine that pushes status when worker exits.
  - **Description**: Enables live cards & status pills to refresh.
  - **Files**:
    - `internal/api/tasks.go`: send broadcasts
    - `internal/worker/watcher.go`: goroutine monitoring exit, invoking callback
    - tests updated
  - **Step Dependencies**: Step 8
  - **User Instructions**: observe JSON event when starting/stopping task.

## Log Streaming

- [ ] Step 10: Tail logs and broadcast `log` events

  - **Task**: Add `internal/worker/tailer.go` to follow log file line-by-line; integrate with hub.
  - **Description**: Powers ThreadView and LogsView real-time output.
  - **Files**:
    - `internal/worker/tailer.go`
    - `internal/worker/tailer_test.go`: uses temp file with writes
  - **Step Dependencies**: Step 9
  - **User Instructions**: receive `"type":"log"` messages over WS.

- [ ] Step 11: Historical log endpoint (`GET /api/tasks/{id}/logs`)
  - **Task**: Serve entire log file or tail `n` lines; stream as `text/plain`.
  - **Description**: Allows front-end to load existing backlog before websocket stream.
  - **Files**:
    - `internal/api/logs.go`
    - `internal/api/logs_test.go`
  - **Step Dependencies**: Steps 3, 5
  - **User Instructions**: `curl :8080/api/tasks/xyz/logs?tail=20`.

## Authentication & Security

- [ ] Step 12: Add JWT middleware

  - **Task**: `internal/api/middleware/auth.go` verifies Bearer token (HS256 secret env); unit test.
  - **Description**: Protects all endpoints except `/healthz`.
  - **Files**:
    - `internal/api/middleware/auth.go`
    - `internal/api/middleware/auth_test.go`
  - **Step Dependencies**: Steps 2, 4
  - **User Instructions**: Requests without token get 401.

- [ ] Step 13: Enable CORS and security headers
  - **Task**: Use `github.com/go-chi/cors` and add `X-Content-Type-Options` header middleware.
  - **Description**: Allows front-end dev server; hardens responses.
  - **Files**:
    - `internal/api/middleware/cors.go`
    - update router wiring
  - **Step Dependencies**: Step 2
  - **User Instructions**: Front-end running on :5173 can call API.

## Documentation & Tooling

- [ ] Step 14: Generate OpenAPI spec
  - **Task**: Integrate `swaggo/swag` comments on handlers and `make docs` target.
  - **Description**: Provides contract for front-end & future SDKs.
  - **Files**:
    - `docs/swagger/*`: generated
    - `Makefile`: `swagger` target
  - **Step Dependencies**: Steps 5–11
  - **User Instructions**: `make swagger` produces `docs/swagger/index.html`.

## CI & Containerisation

- [ ] Step 15: Add GitHub Actions workflow for lint & tests

  - **Task**: `.github/workflows/ci.yml` running `go vet`, `go test ./...`.
  - **Description**: Maintains code quality automatically.
  - **Files**:
    - `.github/workflows/ci.yml`
  - **Step Dependencies**: Steps 1–13
  - **User Instructions**: Push triggers green build.

- [ ] Step 16: Create Dockerfile and local compose
  - **Task**: Multi-stage Dockerfile for `ampd`; minimal `docker-compose.yml` with port mapping.
  - **Description**: Ready-to-deploy container image for staging/prod.
  - **Files**:
    - `Dockerfile`
    - `docker-compose.yml`
    - `Makefile`: `docker-build` target
  - **Step Dependencies**: Steps 1–13
  - **User Instructions**: `docker compose up` starts service on 8080.

## Final Polish

- [ ] Step 17: Add graceful shutdown & logging
  - **Task**: Use `context` + `http.Server` shutdown; add `zap` logger.
  - **Description**: Production-grade observability and SIGTERM handling.
  - **Files**:
    - `cmd/ampd/main.go`: wrap server with shutdown
    - `pkg/log/logger.go`: `zap` setup
  - **Step Dependencies**: Steps 1–13
  - **User Instructions**: `CTRL+C` waits for active requests, logs nicely.
