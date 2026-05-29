package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrTaskTitleRequired      = errors.New("task title is required")
	ErrTaskAlreadyStarted     = errors.New("task already started")
	ErrTaskAlreadyCompleted   = errors.New("task already completed")
	ErrTaskMustBeStartedFirst = errors.New("task must be started before it can be completed")
	ErrTaskCannotBeUpdated    = errors.New("completed task cannot be updated")
	ErrInvalidTaskState       = errors.New("task state is invalid")
)

/*
TaskStatus is a domain concept, not a transport concept.

The values represent the lifecycle language of the backlog bounded context.
Because the type lives in the domain, the business language stays stable
even if the HTTP API or database schema changes later.
*/
type TaskStatus string

const (
	TaskStatusPlanned    TaskStatus = "PLANNED"
	TaskStatusInProgress TaskStatus = "IN_PROGRESS"
	TaskStatusCompleted  TaskStatus = "COMPLETED"
)

/*
TaskSnapshot is a safe data shape used when information must cross
the domain boundary without exposing the aggregate's internal fields.

The aggregate keeps its fields private so that callers cannot mutate
business state without going through domain behavior.
*/
type TaskSnapshot struct {
	ID          string
	Title       string
	Description string
	Status      TaskStatus
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
}

/*
Task is the aggregate root of this example bounded context.

The aggregate root owns the invariants for task lifecycle transitions.
Adapters and application services can ask the aggregate to change,
but they are not allowed to change its state directly.
*/
type Task struct {
	id          string
	title       string
	description string
	status      TaskStatus
	createdAt   time.Time
	startedAt   *time.Time
	completedAt *time.Time
}

/*
NewTask creates a brand-new aggregate in a valid initial state.

The constructor ensures that the first state of every task is the same:
the title must exist, the timestamps are normalized to UTC,
and the lifecycle begins in the planned state.
*/
func NewTask(id string, title string, description string, createdAt time.Time) (Task, error) {
	return RehydrateTask(TaskSnapshot{
		ID:          strings.TrimSpace(id),
		Title:       strings.TrimSpace(title),
		Description: strings.TrimSpace(description),
		Status:      TaskStatusPlanned,
		CreatedAt:   createdAt.UTC(),
	})
}

/*
RehydrateTask reconstructs an aggregate from persistent state.

This function is used by outbound adapters like the SQLite repository.
The adapter is allowed to rebuild the aggregate, but it still must pass
through domain validation so invalid persistence data is rejected early.
*/
func RehydrateTask(snapshot TaskSnapshot) (Task, error) {
	snapshot.ID = strings.TrimSpace(snapshot.ID)
	snapshot.Title = strings.TrimSpace(snapshot.Title)
	snapshot.Description = strings.TrimSpace(snapshot.Description)
	snapshot.CreatedAt = snapshot.CreatedAt.UTC()
	snapshot.StartedAt = cloneTimePointer(snapshot.StartedAt)
	snapshot.CompletedAt = cloneTimePointer(snapshot.CompletedAt)

	if err := validateSnapshot(snapshot); err != nil {
		return Task{}, err
	}

	return Task{
		id:          snapshot.ID,
		title:       snapshot.Title,
		description: snapshot.Description,
		status:      snapshot.Status,
		createdAt:   snapshot.CreatedAt,
		startedAt:   snapshot.StartedAt,
		completedAt: snapshot.CompletedAt,
	}, nil
}

/*
Start moves a task from planned to in-progress.

The method lives on the aggregate because lifecycle rules belong to
business behavior, not to handlers, services, or repositories.
*/
func (task *Task) Start(startedAt time.Time) error {
	switch task.status {
	case TaskStatusInProgress:
		return ErrTaskAlreadyStarted
	case TaskStatusCompleted:
		return ErrTaskAlreadyCompleted
	}

	normalizedStartedAt := startedAt.UTC()
	if normalizedStartedAt.Before(task.createdAt) {
		return fmt.Errorf("%w: start time cannot be before creation time", ErrInvalidTaskState)
	}

	task.status = TaskStatusInProgress
	task.startedAt = &normalizedStartedAt
	task.completedAt = nil

	return nil
}

/*
Complete moves a task from in-progress to completed.

The method refuses invalid transitions so that the application layer
can orchestrate use cases without owning lifecycle rules itself.
*/
func (task *Task) Complete(completedAt time.Time) error {
	if task.status == TaskStatusCompleted {
		return ErrTaskAlreadyCompleted
	}

	if task.status != TaskStatusInProgress || task.startedAt == nil {
		return ErrTaskMustBeStartedFirst
	}

	normalizedCompletedAt := completedAt.UTC()
	if normalizedCompletedAt.Before(*task.startedAt) {
		return fmt.Errorf("%w: completion time cannot be before start time", ErrInvalidTaskState)
	}

	task.status = TaskStatusCompleted
	task.completedAt = &normalizedCompletedAt

	return nil
}

/*
UpdateDetails changes mutable task fields while preserving lifecycle rules.

Completed tasks are treated as immutable snapshots.
*/
func (task *Task) UpdateDetails(title string, description string) error {
	if task.status == TaskStatusCompleted {
		return ErrTaskCannotBeUpdated
	}

	normalizedTitle := strings.TrimSpace(title)
	if normalizedTitle == "" {
		return ErrTaskTitleRequired
	}

	task.title = normalizedTitle
	task.description = strings.TrimSpace(description)

	return nil
}

/*
Snapshot returns a copy of the aggregate's state.

The copy keeps the aggregate encapsulated while still giving outer layers
the data they need for persistence or response mapping.
*/
func (task Task) Snapshot() TaskSnapshot {
	return TaskSnapshot{
		ID:          task.id,
		Title:       task.title,
		Description: task.description,
		Status:      task.status,
		CreatedAt:   task.createdAt,
		StartedAt:   cloneTimePointer(task.startedAt),
		CompletedAt: cloneTimePointer(task.completedAt),
	}
}

/*
validateSnapshot centralizes aggregate consistency checks.

Having one validation path keeps new object creation and persistence
rehydration aligned around the same business rules.
*/
func validateSnapshot(snapshot TaskSnapshot) error {
	if snapshot.ID == "" {
		return fmt.Errorf("%w: task id is required", ErrInvalidTaskState)
	}

	if snapshot.Title == "" {
		return ErrTaskTitleRequired
	}

	if snapshot.CreatedAt.IsZero() {
		return fmt.Errorf("%w: creation time is required", ErrInvalidTaskState)
	}

	if !snapshot.Status.isValid() {
		return fmt.Errorf("%w: unsupported status %q", ErrInvalidTaskState, snapshot.Status)
	}

	if snapshot.StartedAt != nil && snapshot.StartedAt.Before(snapshot.CreatedAt) {
		return fmt.Errorf("%w: start time cannot be before creation time", ErrInvalidTaskState)
	}

	if snapshot.CompletedAt != nil {
		if snapshot.StartedAt == nil {
			return fmt.Errorf("%w: completed task must also have a start time", ErrInvalidTaskState)
		}

		if snapshot.CompletedAt.Before(*snapshot.StartedAt) {
			return fmt.Errorf("%w: completion time cannot be before start time", ErrInvalidTaskState)
		}
	}

	switch snapshot.Status {
	case TaskStatusPlanned:
		if snapshot.StartedAt != nil || snapshot.CompletedAt != nil {
			return fmt.Errorf("%w: planned task cannot have progress timestamps", ErrInvalidTaskState)
		}
	case TaskStatusInProgress:
		if snapshot.StartedAt == nil || snapshot.CompletedAt != nil {
			return fmt.Errorf("%w: in-progress task must have only a start time", ErrInvalidTaskState)
		}
	case TaskStatusCompleted:
		if snapshot.StartedAt == nil || snapshot.CompletedAt == nil {
			return fmt.Errorf("%w: completed task must have start and completion times", ErrInvalidTaskState)
		}
	}

	return nil
}

/*
cloneTimePointer returns a detached copy of a time pointer.

This avoids accidental aliasing when state crosses package boundaries.
*/
func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	clone := value.UTC()
	return &clone
}

/*
isValid keeps the allowed lifecycle vocabulary explicit.

The domain layer is the source of truth for what a valid status means.
*/
func (status TaskStatus) isValid() bool {
	switch status {
	case TaskStatusPlanned, TaskStatusInProgress, TaskStatusCompleted:
		return true
	default:
		return false
	}
}
