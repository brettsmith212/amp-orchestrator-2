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

- [x] Step 6: Start task endpoint (`POST /api/tasks`)

  - **Task**: Parse `{message}` JSON, call `wm.StartWorker`, return created `TaskDTO`; write test using fake binary.
  - **Description**: Enables front-end “New Task” button; lays pattern for mutating endpoints.
  - **Files**:
    - `internal/api/tasks.go`: `StartTask`
    - `internal/api/tasks_test.go`: happy-path + validation error
  - **Step Dependencies**: Step 5
  - **User Instructions**: `curl -XPOST -d'{"message":"hi"}' :8080/api/tasks` returns task JSON.

- [x] Step 7: Stop and Continue endpoints
  - **Task**: Add `POST /api/tasks/{id}/stop` and `/continue` routes & tests; propagate 404 & 409 codes.
  - **Description**: Completes CRUD set needed by PromptBar actions.
  - **Files**:
    - `internal/api/tasks.go`: `StopTask`, `ContinueTask`
    - `internal/api/tasks_test.go`
  - **Step Dependencies**: Steps 5, 6
  - **User Instructions**: commands return 202 on success.

## WebSocket Hub

- [x] Step 8: Implement hub skeleton and `/api/ws` endpoint

  - **Task**: Create `internal/hub/hub.go` managing clients & broadcast; handler upgrades to WS.
  - **Description**: Foundation for realtime updates.
  - **Files**:
    - `internal/hub/hub.go`: `Run`, `Broadcast`, `Register`
    - `internal/hub/client.go`: read/write pumps
    - `internal/api/ws.go`: wires hub to router
    - `internal/hub/hub_test.go`: broadcast unit test using in-memory connections
  - **Step Dependencies**: Steps 2, 4
  - **User Instructions**: `wscat -c ws://localhost:8080/api/ws` connects with no error.

- [x] Step 9: Emit `task-update` events
  - **Task**: Modify task handlers to send `TaskDTO` over hub after state changes; include list watcher goroutine that pushes status when worker exits.
  - **Description**: Enables live cards & status pills to refresh.
  - **Files**:
    - `internal/api/tasks.go`: send broadcasts
    - `internal/worker/watcher.go`: goroutine monitoring exit, invoking callback
    - tests updated
  - **Step Dependencies**: Step 8
  - **User Instructions**: observe JSON event when starting/stopping task.

## Log Streaming

- [x] Step 10: Tail logs and broadcast `log` events

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

## Authentication

- [ ] Step 12: Implement in-memory user store and /auth/login & /auth/refresh
  - **Task**: bcrypt-hash demo users, JWT (HS256) issuance, refresh-token rotation.
  - **Description**: Gives UI a real login flow; later we can swap in DB.
  - **Files** (≤7): auth handlers, tests, middleware wiring, user fixture JSON.
  - **Step Dependencies**: Config, router.

## Standard response & error envelope

- [ ] Step 13: Add response helper and error middleware
  - **Task**: `respond.JSON(w, payload)` and `errors.Wrap(...)` mapping to contract schema.
  - **Description**: Ensures every handler conforms without copy-pasting.
  - **Files**: pkg/response, pkg/apierr, middleware/error.go, tests.

## Pagination & advanced task list

- [ ] Step 14: Implement cursor pagination, filtering, sorting on GET /api/tasks
  - **Task**: query-parser util, update handler, unit tests incl. edge cases.
  - **Description**: Matches UI contract exactly.

## Extended Task actions

- [ ] Step 15: Add interrupt / abort / retry endpoints and state machine guard
  - **Task**: define `AllowedTransitions`, extend WorkerManager to persist `status`.
  - **Description**: Covers PromptBar & retry workflows.

## PATCH, DELETE and Git operations

- [ ] Step 16: Add PATCH /api/tasks/:id, DELETE /api/tasks/:id, merge/delete-branch/create-pr stubs
  - **Task**: accept JSON patch, update metadata; Git stubs return 202 + TODO.
  - **Description**: UI buttons won’t break even before Git integration is ready.

## Thread & message endpoints

- [ ] Step 17: Store thread messages (JSONL per task) and expose /thread endpoint
  - **Task**: append from WorkerManager, paginate, deliver over WS (`thread_message`).

## WebSocket protocol enrichment

- [ ] Step 18: Implement event type switching, ping/pong, heartbeat timeouts
  - **Task**: hub routes by `msg.Type`, responds with `pong`, closes idle conns.

## Rate limiting & CORS hardening

- [ ] Step 19: Integrate `chi/httprate`, add rate-limit headers, tighten CORS
  - **Task**: env-driven limits; tests for header presence.

## Enhanced /health

- [ ] Step 20: Expand /health to report version, uptime, and sub-service checks
  - **Task**: ping Redis (optional), git binary, websocket hub stats.

## Docs & code-gen alignment

- [ ] Step 21: Update Swagger comments to reflect new routes & wrapper schema
  - **Task**: regenerate OpenAPI, commit docs.
