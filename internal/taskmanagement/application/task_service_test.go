package application_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"go-example/internal/taskmanagement/application"
	"go-example/internal/taskmanagement/domain"
	"go-example/internal/taskmanagement/ports/inbound"
	"go-example/internal/taskmanagement/ports/outbound"
)

type fakeTaskRepository struct {
	tasks map[string]domain.Task
}

func newFakeTaskRepository() *fakeTaskRepository {
	return &fakeTaskRepository{tasks: make(map[string]domain.Task)}
}

func (repository *fakeTaskRepository) Save(_ context.Context, task domain.Task) error {
	repository.tasks[task.Snapshot().ID] = task
	return nil
}

func (repository *fakeTaskRepository) FindByID(_ context.Context, taskID string) (domain.Task, error) {
	task, found := repository.tasks[taskID]
	if !found {
		return domain.Task{}, outbound.ErrTaskNotFound
	}

	return task, nil
}

func (repository *fakeTaskRepository) List(_ context.Context) ([]domain.Task, error) {
	tasks := make([]domain.Task, 0, len(repository.tasks))
	for _, task := range repository.tasks {
		tasks = append(tasks, task)
	}

	sort.Slice(tasks, func(left int, right int) bool {
		return tasks[left].Snapshot().CreatedAt.Before(tasks[right].Snapshot().CreatedAt)
	})

	return tasks, nil
}

type fixedIDGenerator struct {
	id string
}

func (generator fixedIDGenerator) NewID() (string, error) {
	return generator.id, nil
}

type fixedClock struct {
	now time.Time
}

func (clock fixedClock) Now() time.Time {
	return clock.now
}

/*
fakeEventPublisher is an in-memory implementation of outbound.TaskEventPublisher.

This fake adapter is one of the clearest benefits of the hexagonal pattern:
because TaskService depends on an interface (not a concrete type), tests can
inject a lightweight stand-in that simply records what was published.

Compare to the real bridge adapter in main.go:
  - The real adapter translates the event and calls the notification module.
  - The fake adapter appends to a slice so tests can make assertions.

Both satisfy the same interface. The service cannot tell them apart.
This is exactly why ports (interfaces) are defined at the boundary:
they make the application core independently testable without any infrastructure.
*/
type fakeEventPublisher struct {
	published []outbound.TaskCompletedEvent
}

func (publisher *fakeEventPublisher) PublishTaskCompleted(_ context.Context, event outbound.TaskCompletedEvent) error {
	publisher.published = append(publisher.published, event)
	return nil
}

func TestTaskServiceCreateTask(t *testing.T) {
	now := time.Date(2026, time.May, 28, 9, 0, 0, 0, time.UTC)
	repository := newFakeTaskRepository()
	service := application.NewTaskService(repository, fixedIDGenerator{id: "task-123"}, fixedClock{now: now}, &fakeEventPublisher{})

	view, err := service.CreateTask(context.Background(), inbound.CreateTaskCommand{
		Title:       "Learn use-case ports",
		Description: "Trace the request through the service",
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if view.ID != "task-123" {
		t.Fatalf("expected generated id, got %s", view.ID)
	}

	if view.Status != domain.TaskStatusPlanned {
		t.Fatalf("expected planned status, got %s", view.Status)
	}

	storedTask, err := repository.FindByID(context.Background(), "task-123")
	if err != nil {
		t.Fatalf("find stored task: %v", err)
	}

	if storedTask.Snapshot().Title != "Learn use-case ports" {
		t.Fatalf("expected stored task title to be preserved")
	}
}

func TestTaskServiceLifecycle(t *testing.T) {
	createdAt := time.Date(2026, time.May, 28, 9, 0, 0, 0, time.UTC)
	repository := newFakeTaskRepository()
	service := application.NewTaskService(repository, fixedIDGenerator{id: "task-456"}, fixedClock{now: createdAt}, &fakeEventPublisher{})

	_, err := service.CreateTask(context.Background(), inbound.CreateTaskCommand{
		Title:       "Finish the example",
		Description: "Move through the whole lifecycle",
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	service = application.NewTaskService(repository, fixedIDGenerator{id: "unused"}, fixedClock{now: createdAt.Add(15 * time.Minute)}, &fakeEventPublisher{})
	startedTask, err := service.StartTask(context.Background(), "task-456")
	if err != nil {
		t.Fatalf("start task: %v", err)
	}

	if startedTask.Status != domain.TaskStatusInProgress {
		t.Fatalf("expected in-progress status, got %s", startedTask.Status)
	}

	/*
		Use a named publisher so we can assert what was published after the call.
		This is the standard way to verify cross-module communication in tests:
		inject a fake outbound adapter, call the use case, then inspect the fake.
	*/
	completionPublisher := &fakeEventPublisher{}
	service = application.NewTaskService(repository, fixedIDGenerator{id: "unused"}, fixedClock{now: createdAt.Add(30 * time.Minute)}, completionPublisher)
	completedTask, err := service.CompleteTask(context.Background(), "task-456")
	if err != nil {
		t.Fatalf("complete task: %v", err)
	}

	if completedTask.Status != domain.TaskStatusCompleted {
		t.Fatalf("expected completed status, got %s", completedTask.Status)
	}

	if completedTask.CompletedAt == nil {
		t.Fatalf("expected completion timestamp to be set")
	}

	/*
		Verify that the domain event was published with the correct payload.

		This assertion proves that the application service called its outbound
		port with the right data. We do not need a real notification service,
		a real console, or any infrastructure to verify this behaviour.
		The fake publisher captured the call, and we inspect it directly.
	*/
	if len(completionPublisher.published) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(completionPublisher.published))
	}

	publishedEvent := completionPublisher.published[0]
	if publishedEvent.TaskID != "task-456" {
		t.Fatalf("expected published event for task-456, got %s", publishedEvent.TaskID)
	}

	if publishedEvent.Title != "Finish the example" {
		t.Fatalf("expected published event title to match task title")
	}
}

func TestTaskServiceReturnsNotFound(t *testing.T) {
	repository := newFakeTaskRepository()
	service := application.NewTaskService(
		repository,
		fixedIDGenerator{id: "unused"},
		fixedClock{now: time.Date(2026, time.May, 28, 9, 0, 0, 0, time.UTC)},
		&fakeEventPublisher{},
	)

	_, err := service.StartTask(context.Background(), "missing-task")
	if !errors.Is(err, outbound.ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
}
