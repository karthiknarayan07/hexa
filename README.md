# Go Clean Architecture + Hexagonal Example

This repository is a small learning project that shows how to combine:

- clean architecture
- hexagonal architecture (ports and adapters)
- DDD-style domain modeling with two separate bounded contexts
- Go structs and interfaces
- a file-based SQLite database
- cross-module communication through a bridge adapter

## Modules

The project contains two independent bounded contexts (modules):

**taskmanagement** — manages the backlog of tasks and their lifecycle.  
Tasks move through: `PLANNED → IN_PROGRESS → COMPLETED`.

**notification** — sends notifications when significant events happen.  
In this example it logs to the console; in a real system it would send emails or push messages.

Neither module imports the other. They communicate only through port interfaces
and a bridge adapter in the composition root.

## Why this example exists

The code is intentionally small, but the boundaries are strict.
The main learning goal is to make the dependency rule obvious:

1. Outside layers call inward.
2. Inner layers never import outer layers.
3. Interfaces live at the boundary that needs them.
4. Adapters depend on ports, not the other way around.

## Project structure

```text
cmd/api/main.go                                    -> composition root + bridge adapter

internal/taskmanagement/domain                     -> task aggregate and lifecycle rules
internal/taskmanagement/ports/inbound              -> use case interfaces
internal/taskmanagement/ports/outbound             -> repository, clock, ID, event publisher interfaces
internal/taskmanagement/application                -> use case implementation
internal/taskmanagement/adapters/in/api            -> HTTP driver adapter
internal/taskmanagement/adapters/out/sqlite        -> SQLite driven adapter
internal/taskmanagement/adapters/out/system        -> clock and ID generator adapters

internal/notification/domain                       -> notification domain object
internal/notification/ports/inbound                -> notification use case interface
internal/notification/ports/outbound               -> sender, clock, ID interfaces
internal/notification/application                  -> notification service
internal/notification/adapters/out/console         -> console (slog) delivery adapter
```

## Data flow

```text
HTTP POST /tasks/{id}/complete
  -> taskmanagement HTTP adapter
  -> taskmanagement inbound port (CompleteTaskUseCase)
  -> taskmanagement application service
  -> domain aggregate (applies lifecycle rule)
  -> SQLite adapter (persist new state)
  -> TaskEventPublisher outbound port
  -> bridge adapter in main.go  <-- only place that knows both modules
  -> notification inbound port (SendTaskCompletionNotificationUseCase)
  -> notification application service
  -> ConsoleSender outbound port
  -> slog output
```

For a folder-by-folder walkthrough of the same path, see `docs/data-flow.md`.

Notice the direction of source-code dependencies:

- `domain` imports nothing from the outside
- `application` imports `domain` and ports
- adapters import ports and application-facing contracts
- `main` is the outermost composition root and wires everything together

## Run the example

```bash
go mod tidy
go run ./cmd/api
```

The server listens on `:8080` and uses the file database at `data/backlog.db`.

## Try the flow

Create a task:

```bash
curl -X POST http://localhost:8080/tasks \
  -H 'Content-Type: application/json' \
  -d '{"title":"Learn clean architecture","description":"Trace the request from adapter to domain"}'
```

List tasks:

```bash
curl http://localhost:8080/tasks
```

Start a task:

```bash
curl -X POST http://localhost:8080/tasks/<task-id>/start
```

Complete a task:

```bash
curl -X POST http://localhost:8080/tasks/<task-id>/complete
```

## What to read first

If you want to study from the inside out:

**taskmanagement module:**
1. `internal/taskmanagement/domain/task.go`
2. `internal/taskmanagement/ports/inbound/task_usecases.go`
3. `internal/taskmanagement/ports/outbound/task_repository.go`
4. `internal/taskmanagement/ports/outbound/task_event_publisher.go`
5. `internal/taskmanagement/application/task_service.go`

**notification module:**
6. `internal/notification/domain/notification.go`
7. `internal/notification/ports/inbound/notification_usecases.go`
8. `internal/notification/ports/outbound/interfaces.go`
9. `internal/notification/application/notification_service.go`
10. `internal/notification/adapters/out/console/notification_sender.go`

**Adapters and wiring:**
11. `internal/taskmanagement/adapters/out/sqlite/task_repository.go`
12. `internal/taskmanagement/adapters/in/api/handler.go`
13. `cmd/api/main.go`  ← the bridge adapter lives here; read this last

If you want to follow a live request from the edge inward, read that list in reverse.

