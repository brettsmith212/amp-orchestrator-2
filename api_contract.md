# API Contract - Amp Orchestrator

This document outlines the complete API contract for the Amp Orchestrator backend, including REST endpoints and WebSocket functionality.

## Base URL

- **Development**: `http://localhost:8080`
- **Production**: TBD

## Authentication

Currently no authentication is required. Future versions may implement JWT-based authentication.

---

## REST API Endpoints

### Health Check

#### `GET /healthz`

Health check endpoint to verify server status.

**Request:**
```http
GET /healthz
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: text/plain; charset=utf-8

ok
```

---

### Task Management

#### `GET /api/tasks`

Retrieve all tasks.

**Request:**
```http
GET /api/tasks
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

[
  {
    "id": "49bb7b72",
    "thread_id": "T-87247d00-6028-4815-a33b-62b3a155faa9",
    "status": "running",
    "started": "2025-06-04T16:14:07.975328556-07:00",
    "log_file": "logs/worker-49bb7b72.log"
  },
  {
    "id": "83d660b7",
    "thread_id": "T-6eb1a877-b8b0-4cbb-83ce-481c36ca2231", 
    "status": "stopped",
    "started": "2025-06-04T16:15:14.496200845-07:00",
    "log_file": "logs/worker-83d660b7.log"
  }
]
```

**Task Object Structure:**
- `id` (string): Unique task identifier (8-character hex)
- `thread_id` (string): Amp thread identifier (T-{uuid})
- `status` (string): Current task status (`running` | `stopped`)
- `started` (string): ISO 8601 timestamp when task was created
- `log_file` (string): Path to task's log file

#### `POST /api/tasks`

Create and start a new task.

**Request:**
```http
POST /api/tasks
Content-Type: application/json

{
  "message": "write a hello world program in Python"
}
```

**Response (Success):**
```http
HTTP/1.1 201 Created
Content-Type: application/json

{
  "id": "4811eece",
  "thread_id": "T-4a7e2c82-d080-4128-acea-e00a04e4f02e",
  "status": "running",
  "started": "2025-06-04T16:18:19.118703147-07:00",
  "log_file": "logs/worker-4811eece.log"
}
```

**Error Responses:**
```http
HTTP/1.1 400 Bad Request
Content-Type: text/plain

Invalid JSON request body
```

```http
HTTP/1.1 400 Bad Request
Content-Type: text/plain

Message is required
```

```http
HTTP/1.1 500 Internal Server Error
Content-Type: text/plain

Failed to start task
```

#### `POST /api/tasks/{id}/stop`

Stop a running task.

**Request:**
```http
POST /api/tasks/4811eece/stop
```

**Response (Success):**
```http
HTTP/1.1 202 Accepted
```

**Error Responses:**
```http
HTTP/1.1 400 Bad Request
Content-Type: text/plain

Task ID is required
```

```http
HTTP/1.1 404 Not Found
Content-Type: text/plain

Task not found
```

```http
HTTP/1.1 409 Conflict
Content-Type: text/plain

Task is not running
```

```http
HTTP/1.1 500 Internal Server Error
Content-Type: text/plain

Failed to stop task
```

#### `POST /api/tasks/{id}/continue`

Send additional message to a running task (continue conversation).

**Request:**
```http
POST /api/tasks/4811eece/continue
Content-Type: application/json

{
  "message": "also add error handling to the program"
}
```

**Response (Success):**
```http
HTTP/1.1 202 Accepted
```

**Error Responses:**
```http
HTTP/1.1 400 Bad Request
Content-Type: text/plain

Task ID is required
```

```http
HTTP/1.1 400 Bad Request
Content-Type: text/plain

Invalid JSON request body
```

```http
HTTP/1.1 400 Bad Request
Content-Type: text/plain

Message is required
```

```http
HTTP/1.1 404 Not Found
Content-Type: text/plain

Task not found
```

```http
HTTP/1.1 409 Conflict
Content-Type: text/plain

Task is not running
```

```http
HTTP/1.1 500 Internal Server Error
Content-Type: text/plain

Failed to continue task
```

---

### Log Retrieval

#### `GET /api/tasks/{id}/logs`

Retrieve log output for a specific task.

**Request (Full Log):**
```http
GET /api/tasks/4811eece/logs
```

**Request (Last N Lines):**
```http
GET /api/tasks/4811eece/logs?tail=20
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: text/plain; charset=utf-8
Cache-Control: no-cache

> write a hello world program in Python
>

╭─────────────────────────╮
│ Create hello.py         │
├─────────────────────────┤
│ print("Hello, World!")  │
╰─────────────────────────╯

Created hello.py successfully.

Shutting down...
Thread ID: T-4a7e2c82-d080-4128-acea-e00a04e4f02e
```

**Query Parameters:**
- `tail` (optional integer): Number of lines to return from the end of the log file. If omitted, returns entire log.

**Error Responses:**
```http
HTTP/1.1 400 Bad Request
Content-Type: text/plain

Task ID is required
```

```http
HTTP/1.1 400 Bad Request
Content-Type: text/plain

Invalid tail parameter
```

```http
HTTP/1.1 404 Not Found
Content-Type: text/plain

Task not found
```

```http
HTTP/1.1 404 Not Found
Content-Type: text/plain

Log file not found
```

```http
HTTP/1.1 500 Internal Server Error
Content-Type: text/plain

Failed to read log file
```

---

## WebSocket API

### Connection

#### `GET /api/ws`

Upgrade HTTP connection to WebSocket for real-time events.

**Request:**
```http
GET /api/ws
Connection: Upgrade
Upgrade: websocket
Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==
Sec-WebSocket-Version: 13
```

**Response:**
```http
HTTP/1.1 101 Switching Protocols
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=
```

### Event Types

Once connected, the WebSocket will send JSON messages for various events:

#### Task Update Events

Sent when a task's status changes (created, stopped, etc.).

**Event Structure:**
```json
{
  "type": "task-update",
  "data": {
    "id": "4811eece",
    "thread_id": "T-4a7e2c82-d080-4128-acea-e00a04e4f02e",
    "status": "stopped",
    "started": "2025-06-04T16:18:19.118703147-07:00",
    "log_file": "logs/worker-4811eece.log"
  }
}
```

**When Triggered:**
- Task is created (`POST /api/tasks`)
- Task is stopped (`POST /api/tasks/{id}/stop`)
- Task completes naturally
- Task status changes

#### Log Events

Sent in real-time as new log lines are written to task log files.

**Event Structure:**
```json
{
  "type": "log",
  "data": {
    "worker_id": "4811eece",
    "timestamp": "2025-06-04T16:18:25.123456789-07:00",
    "content": "Created hello.py successfully."
  }
}
```

**When Triggered:**
- Any time a new line is written to a task's log file
- Real-time streaming of Amp output

---

## Error Handling

### HTTP Status Codes

- `200 OK`: Successful GET requests
- `201 Created`: Successful task creation
- `202 Accepted`: Successful task operations (stop/continue)
- `204 No Content`: Successful operations with no response body
- `400 Bad Request`: Invalid input (malformed JSON, missing required fields, invalid parameters)
- `404 Not Found`: Resource not found (task ID, log file)
- `409 Conflict`: Operation not allowed in current state (e.g., stopping a stopped task)
- `500 Internal Server Error`: Server-side errors

### Error Response Format

All error responses return consistent plain text messages with appropriate HTTP status codes:

```http
HTTP/1.1 400 Bad Request
Content-Type: text/plain; charset=utf-8

Message is required
```

**Error Response Consistency:**
- All errors use standardized response helpers for consistent formatting
- Error messages are descriptive and actionable
- Internal server errors are logged but return generic messages to clients
- Panic recovery ensures the server remains stable

---

## Data Types

### Task Status Values

- `running`: Task is currently executing
- `stopped`: Task has been stopped (either manually or completed)

### Timestamp Format

All timestamps use ISO 8601 format with timezone:
```
2025-06-04T16:18:19.118703147-07:00
```

### Task ID Format

Task IDs are 8-character hexadecimal strings:
```
4811eece
```

### Thread ID Format

Amp thread IDs follow the pattern `T-{uuid}`:
```
T-4a7e2c82-d080-4128-acea-e00a04e4f02e
```

---

---

## Implementation Notes

### Response Standardization

The API uses standardized response helpers and error middleware to ensure consistent behavior:

**Success Responses:**
- JSON responses use `Content-Type: application/json`
- Consistent status codes across all endpoints
- Structured error handling with proper HTTP semantics

**Error Handling:**
- Centralized error middleware processes all errors
- API errors are logged server-side for debugging
- Panic recovery prevents server crashes
- Consistent error message formatting

**Reliability Features:**
- Graceful error recovery
- Structured logging for debugging
- Type-safe response generation
- Consistent HTTP status code usage

---

## Usage Examples

### React Integration Examples

#### Creating a Task
```javascript
const createTask = async (message) => {
  const response = await fetch('/api/tasks', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ message }),
  });
  
  if (!response.ok) {
    // API returns consistent error messages in plain text
    const errorMessage = await response.text();
    throw new Error(`Failed to create task (${response.status}): ${errorMessage}`);
  }
  
  return await response.json();
};
```

#### WebSocket Connection
```javascript
const connectWebSocket = () => {
  const ws = new WebSocket('ws://localhost:8080/api/ws');
  
  ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    
    switch (data.type) {
      case 'task-update':
        handleTaskUpdate(data.data);
        break;
      case 'log':
        handleLogEvent(data.data);
        break;
      default:
        console.log('Unknown event type:', data.type);
    }
  };
  
  return ws;
};
```

#### Fetching Logs
```javascript
const fetchLogs = async (taskId, tail = null) => {
  const url = tail 
    ? `/api/tasks/${taskId}/logs?tail=${tail}`
    : `/api/tasks/${taskId}/logs`;
    
  const response = await fetch(url);
  
  if (!response.ok) {
    // Handle different error types with consistent error messages
    const errorMessage = await response.text();
    if (response.status === 404) {
      throw new Error(`Task or log file not found: ${errorMessage}`);
    }
    throw new Error(`Failed to fetch logs (${response.status}): ${errorMessage}`);
  }
  
  return await response.text();
};
```

---

## Rate Limiting

Currently no rate limiting is implemented. Future versions may include:
- Request rate limits per IP
- WebSocket connection limits
- Task creation limits

---

## CORS

The server includes basic CORS middleware. For production deployment, configure appropriate CORS policies for your frontend domain.

---

## Notes

1. **WebSocket Reconnection**: Implement reconnection logic in your frontend as WebSocket connections can drop.

2. **Log Streaming**: For real-time log viewing, combine the initial log fetch (`GET /api/tasks/{id}/logs`) with WebSocket log events.

3. **Task Polling**: While WebSocket provides real-time updates, you may want to implement periodic polling of `GET /api/tasks` as a fallback.

4. **Error Handling**: Always handle both network errors and HTTP error status codes in your frontend. The API returns consistent error messages with appropriate status codes.

5. **Timestamps**: All timestamps are in the server's local timezone. Consider converting to user's local timezone in the frontend.

6. **Log Content**: Log content is plain text and may contain ANSI escape codes for formatting. Consider using a library like `ansi-to-html` for proper display.

7. **Response Consistency**: All API responses use standardized helpers ensuring consistent JSON formatting and error handling across all endpoints.

8. **Error Recovery**: The server includes panic recovery middleware, ensuring stability even during unexpected errors.
