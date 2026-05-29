package outbound

import (
	"context"
	"errors"
	"time"

	"go-example/internal/task/domain"
)

var ErrTaskNotFound = errors.New("task not found")

/*
TaskRepository is an outbound port.

The application service owns the need for persistence,
so it defines the abstraction it wants instead of depending directly on SQL.
Concrete adapters must satisfy this contract from the outside.
*/
type TaskRepository interface {
	Save(ctx context.Context, task domain.Task) error
	FindByID(ctx context.Context, taskID string) (domain.Task, error)
	List(ctx context.Context) ([]domain.Task, error)
}

/*
IDGenerator is an outbound port for identity creation.

This keeps the application core free from concrete randomness libraries.
Even infrastructure details as small as ID generation are kept outside.
*/
type IDGenerator interface {
	NewID() (string, error)
}

/*
Clock is an outbound port for time.

The abstraction makes use-case tests deterministic because tests can inject
their own clock instead of relying on wall-clock time.
*/
type Clock interface {
	Now() time.Time
}
