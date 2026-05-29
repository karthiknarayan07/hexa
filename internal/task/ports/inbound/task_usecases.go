package inbound

import (
	"context"
	"time"

	"go-example/internal/task/domain"
)

/*
CreateTaskCommand is an input DTO for the create-task use case.

The HTTP adapter can build this struct from JSON, but the struct itself
belongs to the use-case boundary rather than to the transport layer.
*/
type CreateTaskCommand struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

/*
TaskView is an output DTO exposed by inbound ports.

The application layer returns this shape to outer adapters so they do not
need direct access to domain internals.
*/
type TaskView struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Status      domain.TaskStatus `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
}

/*
CreateTaskUseCase is an inbound port.

Inbound adapters depend on this interface so they can ask the application
core to create a task without knowing which concrete service implements it.
*/
type CreateTaskUseCase interface {
	CreateTask(ctx context.Context, command CreateTaskCommand) (TaskView, error)
}

/*
StartTaskUseCase is an inbound port for the transition into work-in-progress.

The port is intentionally narrow so an adapter only depends on the exact
behavior it needs.
*/
type StartTaskUseCase interface {
	StartTask(ctx context.Context, taskID string) (TaskView, error)
}

/*
CompleteTaskUseCase is an inbound port for finishing work.

Keeping it separate from the start use case helps show that ports can be
small and explicit instead of one large interface.
*/
type CompleteTaskUseCase interface {
	CompleteTask(ctx context.Context, taskID string) (TaskView, error)
}

/*
ListTasksUseCase is an inbound query port.

Commands and queries are still small use cases with clear boundaries,
even in a tiny teaching example like this one.
*/
type ListTasksUseCase interface {
	ListTasks(ctx context.Context) ([]TaskView, error)
}
