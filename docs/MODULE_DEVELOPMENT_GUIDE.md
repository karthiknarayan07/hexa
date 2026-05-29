# How to Write a New Module — A Step-by-Step Guide

This guide teaches you how to build a new bounded context (module) in this hexagonal architecture project using an inside-out, layer-by-layer approach. You'll start with a single operation (Create) and flow it end-to-end, then add capabilities incrementally.

## Core Philosophy

**Inside-Out Development:**

- Start at the **domain layer** (the core business rules)
- Build **inbound ports** (interfaces the outside world calls)
- Implement the **application service** (orchestrates domain + ports)
- Add **one adapter** at a time (persistence, HTTP, CLI, etc.)
- Wire everything together
- Verify one complete flow works end-to-end
- Then add the next operation (List, Get, Update, Delete)

**Why this order?** The domain has no dependencies, so you can test it in isolation. Then each layer builds on the previous one, reducing cascading errors.

---

## Example: Building the Task Module from Scratch

Let's say we're starting fresh with just the task module. Here's how to build it incrementally.

### Layer 1: Domain (The Core — Always Start Here)

**File:** `internal/task/domain/task.go`

Start with your aggregate. This is **pure business logic** with zero external dependencies.

```go
package domain

import "errors"

var (
    ErrTaskTitleRequired = errors.New("task title is required")
    ErrTaskInvalidStatus = errors.New("task has invalid status")
)

type Status string

const (
    StatusPlanned     Status = "PLANNED"
    StatusInProgress  Status = "IN_PROGRESS"
    StatusCompleted   Status = "COMPLETED"
)

// Task is the domain aggregate for task lifecycle.
type Task struct {
    id          string
    title       string
    description string
    status      Status
    createdAt   time.Time
}

// NewTask constructs a task in PLANNED state.
func NewTask(id, title string) (Task, error) {
    if title == "" {
        return Task{}, ErrTaskTitleRequired
    }
    return Task{
        id:        id,
        title:     title,
        status:    StatusPlanned,
        createdAt: time.Now(),
    }, nil
}

// Start transitions PLANNED → IN_PROGRESS.
func (t *Task) Start() error {
    if t.status != StatusPlanned {
        return ErrTaskInvalidStatus
    }
    t.status = StatusInProgress
    return nil
}

// Complete transitions IN_PROGRESS → COMPLETED.
func (t *Task) Complete() error {
    if t.status != StatusInProgress {
        return ErrTaskInvalidStatus
    }
    t.status = StatusCompleted
    return nil
}

// Snapshot returns an immutable view of the task.
func (t *Task) Snapshot() TaskSnapshot {
    return TaskSnapshot{
        ID:          t.id,
        Title:       t.title,
        Description: t.description,
        Status:      string(t.status),
        CreatedAt:   t.createdAt,
    }
}

type TaskSnapshot struct {
    ID          string
    Title       string
    Description string
    Status      string
    CreatedAt   time.Time
}
```

**Key principles:**

- All fields private (lowercase) — controlled via methods only
- No external dependencies (no database, no HTTP, no logging)
- Pure validation and state transitions
- Returns immutable snapshots via `Snapshot()`

**Verification:** Run `go test ./internal/task/domain` — domain should be isolated and fast to test.

---

### Layer 2: Inbound Ports (The Interface Contracts)

**File:** `internal/task/ports/inbound/task_usecases.go`

Define what operations the outside world can request. These are **behavior contracts**, not implementations.

Start with just **one operation**: Create.

```go
package inbound

import (
    "context"

    "hexa/internal/task/domain"
)

// CreateTaskCommand is the input DTO for creating a task.
type CreateTaskCommand struct {
    Title       string
    Description string
}

// TaskView is the output DTO returned after operations.
type TaskView struct {
    ID          string
    Title       string
    Description string
    Status      string
    CreatedAt   string // RFC3339 for JSON
}

// CreateTaskUseCase is an inbound port for task creation.
type CreateTaskUseCase interface {
    CreateTask(ctx context.Context, cmd CreateTaskCommand) (TaskView, error)
}
```

**Key principles:**

- Define **one interface per operation** (even if operation is "Create")
- Use **DTOs** (Data Transfer Objects) for input/output, not domain aggregates
- Keep contracts minimal — only what the operation needs
- No implementation details here

**Why separate interfaces?** Because you can independently implement, test, and replace each operation.

---

### Layer 3: Outbound Ports (The Dependencies)

**File:** `internal/task/ports/outbound/interfaces.go`

Define what the application service needs from infrastructure.

```go
package outbound

import (
    "context"

    "hexa/internal/task/domain"
)

// TaskRepository saves and loads tasks.
type TaskRepository interface {
    Save(ctx context.Context, task domain.Task) error
    FindByID(ctx context.Context, taskID string) (domain.Task, error)
}

// IDGenerator creates unique task identifiers.
type IDGenerator interface {
    Generate() string
}

// Clock provides the current time.
type Clock interface {
    Now() time.Time
}
```

**Key principles:**

- Small, focused interfaces — one responsibility per port
- Use domain types as method parameters/returns (not DTOs)
- Repository pattern for persistence
- Abstract time and ID generation (for testability)

---

### Layer 4: Application Service (Orchestration)

**File:** `internal/task/application/task_service.go`

This is where your use cases live. It orchestrates domain logic + ports.

Start with **just CreateTask**:

```go
package application

import (
    "context"
    "fmt"

    "hexa/internal/task/domain"
    "hexa/internal/task/ports/inbound"
    "hexa/internal/task/ports/outbound"
)

// TaskService implements inbound use-case ports.
type TaskService struct {
    repository outbound.TaskRepository
    idGen      outbound.IDGenerator
    clock      outbound.Clock
}

// NewTaskService constructs the service with its dependencies.
func NewTaskService(
    repository outbound.TaskRepository,
    idGen outbound.IDGenerator,
    clock outbound.Clock,
) *TaskService {
    return &TaskService{
        repository: repository,
        idGen:      idGen,
        clock:      clock,
    }
}

// Verify the service implements the use-case port.
var _ inbound.CreateTaskUseCase = (*TaskService)(nil)

// CreateTask implements the CreateTaskUseCase port.
func (svc *TaskService) CreateTask(ctx context.Context, cmd inbound.CreateTaskCommand) (inbound.TaskView, error) {
    // 1. Generate a unique ID
    taskID := svc.idGen.Generate()

    // 2. Create domain aggregate
    task, err := domain.NewTask(taskID, cmd.Title)
    if err != nil {
        return inbound.TaskView{}, err
    }

    // 3. Persist to repository
    if err := svc.repository.Save(ctx, task); err != nil {
        return inbound.TaskView{}, fmt.Errorf("save task: %w", err)
    }

    // 4. Return DTO to caller
    return toTaskView(task), nil
}

// Helper to convert domain aggregate to DTO.
func toTaskView(task domain.Task) inbound.TaskView {
    snapshot := task.Snapshot()
    return inbound.TaskView{
        ID:          snapshot.ID,
        Title:       snapshot.Title,
        Description: snapshot.Description,
        Status:      snapshot.Status,
        CreatedAt:   snapshot.CreatedAt.Format(time.RFC3339),
    }
}
```

**Key principles:**

- Constructor injection of all dependencies
- One method per inbound use-case interface
- Orchestrates: domain logic → persistence → response
- Returns DTOs, not domain aggregates
- Wraps errors with context (e.g., `fmt.Errorf("save task: %w", err)`)

**Testing hint:** At this point you can write unit tests with a fake repository and ID generator.

---

### Layer 5: Outbound Adapter (Persistence)

**File:** `internal/task/adapters/out/sqlite/task_repository.go`

Implement the TaskRepository port using SQLite.

```go
package sqlite

import (
    "context"
    "database/sql"
    "fmt"

    "hexa/internal/task/domain"
    "hexa/internal/task/ports/outbound"
)

type SQLiteTaskRepository struct {
    database *sql.DB
}

func NewSQLiteTaskRepository(database *sql.DB) *SQLiteTaskRepository {
    return &SQLiteTaskRepository{database: database}
}

// Verify the repository implements the port.
var _ outbound.TaskRepository = (*SQLiteTaskRepository)(nil)

// Save upserts a task to the database.
func (repo *SQLiteTaskRepository) Save(ctx context.Context, task domain.Task) error {
    snapshot := task.Snapshot()
    _, err := repo.database.ExecContext(
        ctx,
        `INSERT OR REPLACE INTO tasks (id, title, description, status, created_at)
         VALUES (?, ?, ?, ?, ?)`,
        snapshot.ID,
        snapshot.Title,
        snapshot.Description,
        snapshot.Status,
        snapshot.CreatedAt.Format(time.RFC3339),
    )
    if err != nil {
        return fmt.Errorf("insert task: %w", err)
    }
    return nil
}

// FindByID loads a task by id, or returns error if not found.
func (repo *SQLiteTaskRepository) FindByID(ctx context.Context, taskID string) (domain.Task, error) {
    var title, description, status string
    var createdAt string

    err := repo.database.QueryRowContext(
        ctx,
        `SELECT title, description, status, created_at FROM tasks WHERE id = ?`,
        taskID,
    ).Scan(&title, &description, &status, &createdAt)

    if err == sql.ErrNoRows {
        return domain.Task{}, fmt.Errorf("task %s: not found", taskID)
    }
    if err != nil {
        return domain.Task{}, fmt.Errorf("query task: %w", err)
    }

    // Reconstruct the domain aggregate from persisted data
    // (In a real system, you might use a factory or fromSnapshot method)
    return domain.NewTask(taskID, title)
}

// ensureSchema creates the tasks table on init.
func (repo *SQLiteTaskRepository) ensureSchema() error {
    _, err := repo.database.Exec(`CREATE TABLE IF NOT EXISTS tasks (
        id TEXT PRIMARY KEY,
        title TEXT NOT NULL,
        description TEXT,
        status TEXT NOT NULL,
        created_at TEXT NOT NULL
    )`)
    return err
}
```

**Key principles:**

- Implements the port interface exactly
- Uses database transactions/contexts for cancellation
- Returns domain aggregates (not rows/DTOs)
- Wraps errors with operation context
- All persistence details hidden behind the port boundary

---

### Layer 6: Inbound Adapter (HTTP) — One Endpoint

**File:** `internal/task/adapters/in/api/handler.go`

Expose the CreateTask operation via HTTP.

```go
package api

import (
    "encoding/json"
    "errors"
    "net/http"

    "hexa/internal/task/ports/inbound"
)

type TaskHandler struct {
    createUseCase inbound.CreateTaskUseCase
}

func NewTaskHandler(createUseCase inbound.CreateTaskUseCase) *TaskHandler {
    return &TaskHandler{createUseCase: createUseCase}
}

// HandleCreateTask is the HTTP handler for POST /tasks
func (h *TaskHandler) HandleCreateTask(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Title       string `json:"title"`
        Description string `json:"description"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid JSON", http.StatusBadRequest)
        return
    }

    cmd := inbound.CreateTaskCommand{
        Title:       req.Title,
        Description: req.Description,
    }

    taskView, err := h.createUseCase.CreateTask(r.Context(), cmd)
    if err != nil {
        if errors.Is(err, inbound.ErrTaskTitleRequired) {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(taskView)
}
```

**Key principles:**

- Depends only on inbound port (use-case interface), not the concrete service
- Translates HTTP request → command DTO
- Maps domain errors → HTTP status codes
- Returns responses as JSON

---

### Layer 7: Composition Root (Wiring)

**File:** `internal/platform/bootstrap/container.go`

Wire all the layers together.

```go
package bootstrap

import (
    "database/sql"

    "hexa/internal/platform/system"
    "hexa/internal/task/adapters/in/api"
    "hexa/internal/task/adapters/out/sqlite"
    "hexa/internal/task/application"
    "hexa/internal/task/ports/inbound"
)

type Container struct {
    // Inbound ports (what the outside world calls)
    TaskService inbound.CreateTaskUseCase

    // HTTP handler
    TaskHandler *api.TaskHandler
}

func BuildForServer() (*Container, error) {
    // Open database
    db, err := openDatabase()
    if err != nil {
        return nil, err
    }

    // Create SQLite adapter
    taskRepository := sqlite.NewSQLiteTaskRepository(db)

    // Create system adapters (ID gen, clock)
    idGen := system.NewRandomIDGenerator()
    clock := system.NewSystemClock()

    // Create application service
    taskService := application.NewTaskService(taskRepository, idGen, clock)

    // Create HTTP handler
    taskHandler := api.NewTaskHandler(taskService)

    return &Container{
        TaskService: taskService,
        TaskHandler: taskHandler,
    }, nil
}

func openDatabase() (*sql.DB, error) {
    db, err := sql.Open("sqlite3", "data/backlog.db")
    if err != nil {
        return nil, err
    }
    return db, nil
}
```

**Key principles:**

- Central place where all layers are assembled
- Each component receives its dependencies via constructor
- The outside world only knows about the container, not the internals

---

### Layer 8: Route Registration (HTTP Server)

**File:** `cmd/api/main.go` (or wherever your HTTP server starts)

Register the endpoint.

```go
func main() {
    container, err := bootstrap.BuildForServer()
    if err != nil {
        log.Fatal(err)
    }

    mux := http.NewServeMux()

    // Register task routes
    mux.HandleFunc("POST /tasks", container.TaskHandler.HandleCreateTask)

    http.ListenAndServe(":8080", mux)
}
```

---

## Verification: Test the Create Flow End-to-End

Now test the complete flow manually:

```bash
# 1. Run the server
go run ./cmd/api

# 2. In another terminal, create a task
curl -X POST http://localhost:8080/tasks \
  -H "Content-Type: application/json" \
  -d '{"title":"My First Task","description":"Learn hexagonal architecture"}'

# 3. Response (201 Created):
# {
#   "id":"abc123...",
#   "title":"My First Task",
#   "description":"Learn hexagonal architecture",
#   "status":"PLANNED",
#   "createdAt":"2026-05-29T10:00:00Z"
# }
```

**Celebrate!** ✅ You've completed one entire flow: HTTP → Handler → Service → Domain → Repository → SQLite.

---

## Step 9: Add the Next Operation (Incrementally)

Now that Create works end-to-end, add **List** or **Get** using the same process:

1. **Add new inbound port** in `ports/inbound/task_usecases.go`:

   ```go
   type ListTasksUseCase interface {
       ListTasks(ctx context.Context) ([]TaskView, error)
   }
   ```

2. **Add outbound method** if needed (e.g., Repository.List):

   ```go
   type TaskRepository interface {
       Save(ctx context.Context, task domain.Task) error
       FindByID(ctx context.Context, taskID string) (domain.Task, error)
       List(ctx context.Context) ([]domain.Task, error)  // NEW
   }
   ```

3. **Implement in service**:

   ```go
   func (svc *TaskService) ListTasks(ctx context.Context) ([]inbound.TaskView, error) {
       tasks, err := svc.repository.List(ctx)
       if err != nil {
           return nil, err
       }
       views := make([]inbound.TaskView, len(tasks))
       for i, t := range tasks {
           views[i] = toTaskView(t)
       }
       return views, nil
   }
   ```

4. **Implement in SQLite adapter**:

   ```go
   func (repo *SQLiteTaskRepository) List(ctx context.Context) ([]domain.Task, error) {
       rows, err := repo.database.QueryContext(ctx, `SELECT id, title, ... FROM tasks`)
       // ... scan rows and reconstruct tasks
       return tasks, nil
   }
   ```

5. **Add HTTP handler**:

   ```go
   func (h *TaskHandler) HandleListTasks(w http.ResponseWriter, r *http.Request) {
       tasks, err := h.listUseCase.ListTasks(r.Context())
       // ... JSON response
   }
   ```

6. **Register route**:

   ```go
   mux.HandleFunc("GET /tasks", container.TaskHandler.HandleListTasks)
   ```

7. **Test**:

   ```bash
   curl http://localhost:8080/tasks
   ```

Repeat this pattern for Get, Update, Delete, etc.

---

## Adding CLI Commands (Optional but Recommended)

If you also want CLI support (like we did with the notification module):

1. **Create CLI adapter**: `internal/task/adapters/in/cli/command.go`

   ```go
   type Services struct {
       Create inbound.CreateTaskUseCase
       List   inbound.ListTasksUseCase
       // ...
   }

   func NewTaskCommand(provider func() (Services, func() error, error)) *cobra.Command {
       // Build Cobra command tree using provider pattern
   }
   ```

2. **Wire in bootstrap** with a `BuildForCLI()` function
3. **Register in root command** so CLI calls use-cases directly (no HTTP)

---

## Key Takeaways: The Development Process

| Step | What | Where | Dependencies |
|------|------|-------|--------------|
| 1 | Domain aggregate | `domain/` | None (pure logic) |
| 2 | Inbound port (one use case) | `ports/inbound/` | domain types |
| 3 | Outbound ports | `ports/outbound/` | domain types |
| 4 | Application service | `application/` | domain + ports |
| 5 | Persistence adapter | `adapters/out/` | domain + outbound ports |
| 6 | HTTP adapter | `adapters/in/api/` | inbound ports |
| 7 | Wiring (composition root) | `bootstrap/` | everything above |
| 8 | Route registration | `cmd/api/` | container |
| 9 | Test end-to-end | (manual or test file) | running server |
| 10+ | Add next operation | Repeat steps 2–9 | previous work |

**Flow direction:** Always build from the inside out (domain first), then outward to adapters. This ensures your core logic is never dependent on infrastructure.

---

## Design Questions to Answer for Each Module

Before you start coding a new module, ask yourself:

1. **What is the domain concept?** (Task, Notification, Payment, etc.)
2. **What are the state transitions?** (PLANNED → IN_PROGRESS → COMPLETED)
3. **What operations does the outside world request?** (Create, List, Get, Update, Delete, etc.)
4. **What external systems do we need?** (Database? Email service? ID generator? Clock?)
5. **What events might this module publish?** (TaskCompleted → triggers notifications)
6. **Does this module need to listen to events from other modules?** (If yes, use a bridge adapter in the composition root)

Answer these upfront, and the layers will flow naturally.

---

## Pro Tips

- **Test each layer in isolation:** Domain → no dependencies. Service → fake repository. Adapter → test query format.
- **Use `var _ Interface = (*Concrete)(nil)`** to verify implementations at compile time.
- **One operation per iteration:** Don't try to implement Create + List + Delete at once.
- **Error handling is important:** Distinguish between domain errors (validation) and infrastructure errors (database).
- **DTOs are not aggregates:** Input/output use DTOs; internal domain uses aggregates.
- **The composition root is your blueprint:** If it gets messy, your layering might be wrong.
