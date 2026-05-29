# Data Flow Guide

This document explains the runtime flow and the dependency flow using the folder layout of this repository.

## Modules in this project

This project has two independent modules (bounded contexts):

- **task** — owns the task lifecycle (PLANNED → IN_PROGRESS → COMPLETED)
- **notification** — delivers notifications when something significant happens

Neither module imports the other. They communicate through port interfaces and
a bridge adapter that lives in the composition root (`cmd/api/main.go`).

## Folder layers

```text
cmd/
  api/                                      -> composition root + cross-module bridge adapter

internal/
  task/                           -> bounded context: task lifecycle
    domain/                                 -> Task aggregate, lifecycle rules
    ports/
      inbound/                              -> use cases the outside world can call
      outbound/                             -> TaskRepository, IDGenerator, Clock, TaskEventPublisher
    application/                            -> use-case orchestration
    adapters/
      in/
        api/                                -> HTTP transport adapter
      out/
        sqlite/                             -> persistence adapter
        system/                             -> clock and ID generator adapters

  notification/                             -> bounded context: message delivery
    domain/                                 -> Notification value object
    ports/
      inbound/                              -> SendTaskCompletionNotificationUseCase
      outbound/                             -> NotificationSender, IDGenerator, Clock
    application/                            -> NotificationService
    adapters/
      out/
        console/                            -> slog console delivery adapter

data/                                       -> SQLite file created at runtime
docs/                                       -> learning notes and design docs
```

## Runtime flow: task completion end-to-end

When a client calls `POST /tasks/{id}/complete`:

1. `cmd/api` (composition root)
   The server is running with all adapters wired in.

2. `internal/task/adapters/in/api`
   The HTTP adapter extracts the task ID from the route and calls the
   `CompleteTaskUseCase` inbound port.

3. `internal/task/ports/inbound`
   The adapter uses only the interface at this boundary, not the concrete service.

4. `internal/task/application`
   The TaskService loads the task, asks the domain aggregate to complete it,
   persists the new state, then calls the `TaskEventPublisher` outbound port.

5. `internal/task/domain`
   The Task aggregate validates the transition: only IN_PROGRESS tasks can complete.

6. `internal/task/ports/outbound`
   Two outbound ports are called:
   - `TaskRepository.Save` — persists the completed task to SQLite
   - `TaskEventPublisher.PublishTaskCompleted` — fires the cross-module event

7. `cmd/api` (bridge adapter — `taskCompletionBridge`)
   This struct satisfies the `TaskEventPublisher` port.
   It translates `TaskCompletedEvent` into `SendTaskCompletionNotificationCommand`
   and calls the notification module's inbound port.
   This is the ONLY code in the project that knows about both modules.

8. `internal/notification/ports/inbound`
   The bridge calls `SendTaskCompletionNotificationUseCase`, crossing into
   the notification module through its own boundary.

9. `internal/notification/application`
   The NotificationService generates an ID, builds a Notification domain value,
   and calls the `NotificationSender` outbound port.

10. `internal/notification/domain`
    The Notification value object validates that all required fields are present.

11. `internal/notification/adapters/out/console`
   The ConsoleNotificationSender writes a structured slog entry to stdout.

12. Control unwinds back through all layers returning the HTTP 200 response.

## Dependency rule: source-code arrows always point inward

The runtime call path flows inward and then back outward.
But source-code dependencies MUST only point inward.

```text
cmd/api              →  all modules (composition root is the exception)
task       →  its own domain and ports only
notification         →  its own domain and ports only
adapters             →  the ports they satisfy
neither module       →  the other module  (this is the key rule)
```

## The bridge adapter pattern

The bridge adapter in `cmd/api/main.go` is the answer to:
"How do two modules communicate without importing each other?"

```text
  task.TaskService
        |
        |  calls outbound port
        v
  taskOut.TaskEventPublisher   <-- interface owned by task
        ^
        |  satisfied by
        |
  taskCompletionBridge   <-- in cmd/api/main.go only
        |
        |  calls inbound port
        v
  notificationIn.SendTaskCompletionNotificationUseCase  <-- interface owned by notification
        ^
        |  satisfied by
        |
  notification.NotificationService
```

Neither module is in the middle of this diagram.
The bridge sits at the outer edge and connects them.

## Quick "where does this code live" guide

| What it is | Where it lives |
|---|---|
| Business rule for a task | `task/domain` |
| Business rule for a notification | `notification/domain` |
| Task use-case the HTTP layer calls | `task/ports/inbound` |
| Notification use-case the bridge calls | `notification/ports/inbound` |
| Interface for things task needs | `task/ports/outbound` |
| Interface for things notification needs | `notification/ports/outbound` |
| Task use-case orchestration | `task/application` |
| Notification use-case orchestration | `notification/application` |
| HTTP, JSON, route parsing | `task/adapters/in/api` |
| SQLite row mapping | `task/adapters/out/sqlite` |
| Console log delivery | `notification/adapters/out/console` |
| Clock, ID generator (concrete) | `task/adapters/out/system` |
| All object wiring + bridge | `cmd/api/main.go` |
