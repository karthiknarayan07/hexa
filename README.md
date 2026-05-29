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

**task** — manages the backlog of tasks and their lifecycle.  
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
cmd/app/main.go                                    -> thin executable entrypoint

internal/platform/cli                              -> root cobra command tree (run + task)
internal/platform/bootstrap                        -> composition root, bridge, runtime starters

internal/task/domain                     -> task aggregate and lifecycle rules
internal/task/ports/inbound              -> use case interfaces
internal/task/ports/outbound             -> repository, clock, ID, event publisher interfaces
internal/task/application                -> use case implementation
internal/task/adapters/in/api            -> HTTP driver adapter
internal/task/adapters/in/cli            -> CLI driver adapter (cobra commands)
internal/task/adapters/out/sqlite        -> SQLite driven adapter
internal/task/adapters/out/system        -> clock and ID generator adapters

internal/notification/domain                       -> notification domain object
internal/notification/ports/inbound                -> notification use case interface
internal/notification/ports/outbound               -> sender, clock, ID interfaces
internal/notification/application                  -> notification service
internal/notification/adapters/out/console         -> console (slog) delivery adapter
```

## Data flow

```text
HTTP POST /tasks/{id}/complete
  -> task HTTP adapter
  -> task inbound port (CompleteTaskUseCase)
  -> task application service
  -> domain aggregate (applies lifecycle rule)
  -> SQLite adapter (persist new state)
  -> TaskEventPublisher outbound port
  -> bridge adapter in internal/platform/bootstrap
  -> notification inbound port (SendTaskCompletionNotificationUseCase)
  -> notification application service
  -> ConsoleNotificationSender outbound port
  -> slog output
```

For a folder-by-folder walkthrough of the same path, see `docs/data-flow.md`.

Notice the direction of source-code dependencies:

- `domain` imports nothing from the outside
- `application` imports `domain` and ports
- adapters import ports and application-facing contracts
- `internal/platform/bootstrap` is the composition root and wires everything together

## Run the example

```bash
go mod tidy
go run ./cmd/app run
```

By default `run` starts all runtime modes (`http`, `grpc`, `worker`, `cron`).
You can narrow to selected modes:

```bash
go run ./cmd/app run --run http
go run ./cmd/app run --run http --run grpc
```

Task management is also available via CLI:

```bash
go run ./cmd/app task create --title "Learn clean architecture" --description "trace the flow"
go run ./cmd/app task list
go run ./cmd/app task get --id <task-id>
go run ./cmd/app task update --id <task-id> --title "Updated title" --description "Updated description"
go run ./cmd/app task delete --id <task-id>
```

SQLite database path remains `data/backlog.db`.

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

**task module:**
1. `internal/task/domain/task.go`
2. `internal/task/ports/inbound/task_usecases.go`
3. `internal/task/ports/outbound/task_repository.go`
4. `internal/task/ports/outbound/task_event_publisher.go`
5. `internal/task/application/task_service.go`

**notification module:**
6. `internal/notification/domain/notification.go`
7. `internal/notification/ports/inbound/notification_usecases.go`
8. `internal/notification/ports/outbound/interfaces.go`
9. `internal/notification/application/notification_service.go`
10. `internal/notification/adapters/out/console/notification_sender.go`

**Adapters and wiring:**
11. `internal/task/adapters/out/sqlite/task_repository.go`
12. `internal/task/adapters/in/api/handler.go`
13. `internal/platform/bootstrap/container.go`  ← bridge + runtime composition root
14. `internal/platform/cli/root_command.go`  ← single-entrypoint command orchestration
15. `cmd/app/main.go`  ← tiny executable entrypoint

If you want to follow a live request from the edge inward, read that list in reverse.

