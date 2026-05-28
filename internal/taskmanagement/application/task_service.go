package application

import (
	"context"
	"fmt"

	"go-example/internal/taskmanagement/domain"
	"go-example/internal/taskmanagement/ports/inbound"
	"go-example/internal/taskmanagement/ports/outbound"
)

var (
	_ inbound.CreateTaskUseCase   = (*TaskService)(nil)
	_ inbound.StartTaskUseCase    = (*TaskService)(nil)
	_ inbound.CompleteTaskUseCase = (*TaskService)(nil)
	_ inbound.ListTasksUseCase    = (*TaskService)(nil)
)

/*
TaskService is the application-layer implementation of the use cases.

This service sits at the heart of the taskmanagement module. It:
  - receives commands through inbound ports (called by adapters like HTTP)
  - delegates business-rule enforcement to the domain aggregate
  - persists state through the repository outbound port
  - publishes domain events through the eventPublisher outbound port

Notice what this service does NOT know:
  - how HTTP requests are structured
  - what SQL table stores tasks
  - which module (or which concrete type) receives the published events

That deliberate ignorance is the point. Clean architecture keeps the
application core independent of everything that can change.
*/
type TaskService struct {
	repository  outbound.TaskRepository
	idGenerator outbound.IDGenerator
	clock       outbound.Clock
	/*
		eventPublisher is the outbound port for domain events.

		When the domain transitions a task to COMPLETED, the service notifies
		the outside world through this port. The service holds an interface,
		not a concrete type, so it cannot know or care whether the concrete
		adapter logs to a console, publishes to Kafka, or calls another module.

		In this example, the composition root (main.go) wires in a bridge adapter
		that translates the event and calls the notification module.
	*/
	eventPublisher outbound.TaskEventPublisher
}

/*
NewTaskService creates the application service with the dependencies it needs.

All parameters are interfaces. The service never receives concrete adapters
directly. This is the dependency inversion principle: high-level policy
(use cases) does not depend on low-level details (SQL, HTTP, notifications).
Both depend on abstractions (the port interfaces).

The composition root (main.go) is the only place that creates concrete
adapters and passes them here.
*/
func NewTaskService(
	repository outbound.TaskRepository,
	idGenerator outbound.IDGenerator,
	clock outbound.Clock,
	eventPublisher outbound.TaskEventPublisher,
) *TaskService {
	return &TaskService{
		repository:     repository,
		idGenerator:    idGenerator,
		clock:          clock,
		eventPublisher: eventPublisher,
	}
}

/*
CreateTask handles the use case of creating a new task.

Notice the flow:
the adapter calls this method,
the service creates a domain aggregate,
and the service persists it through an outbound port.
*/
func (service *TaskService) CreateTask(ctx context.Context, command inbound.CreateTaskCommand) (inbound.TaskView, error) {
	taskID, err := service.idGenerator.NewID()
	if err != nil {
		return inbound.TaskView{}, fmt.Errorf("generate task id: %w", err)
	}

	task, err := domain.NewTask(taskID, command.Title, command.Description, service.clock.Now())
	if err != nil {
		return inbound.TaskView{}, err
	}

	if err := service.repository.Save(ctx, task); err != nil {
		return inbound.TaskView{}, fmt.Errorf("save created task: %w", err)
	}

	return toTaskView(task), nil
}

/*
StartTask loads an aggregate, asks the domain to apply a lifecycle change,
and then persists the new state through the repository port.
*/
func (service *TaskService) StartTask(ctx context.Context, taskID string) (inbound.TaskView, error) {
	task, err := service.repository.FindByID(ctx, taskID)
	if err != nil {
		return inbound.TaskView{}, err
	}

	if err := task.Start(service.clock.Now()); err != nil {
		return inbound.TaskView{}, err
	}

	if err := service.repository.Save(ctx, task); err != nil {
		return inbound.TaskView{}, fmt.Errorf("save started task: %w", err)
	}

	return toTaskView(task), nil
}

/*
CompleteTask orchestrates the task-completion use case.

The flow inside this method demonstrates the recommended ordering for
application services that combine persistence with event publishing:

	Step 1 — load:    fetch the current aggregate from the repository
	Step 2 — mutate:  ask the domain to apply the lifecycle transition
	Step 3 — persist: save the new state durably
	Step 4 — publish: fire the domain event to notify other modules

Why persist BEFORE publishing?

	If we published first and the database write then failed, the notification
	module would have acted on an event for a task that was never saved.
	That creates an inconsistency that is hard to recover from.

	By persisting first, the task is durably completed even if event delivery
	fails. In a more advanced system you would use an outbox pattern to make
	event delivery also durable, but for this example simple ordering suffices.

Why publish AFTER the full save rather than inside the domain aggregate?

	The domain aggregate should not know about messaging or other modules.
	It only knows business rules. The application service is the right layer
	to trigger side effects after state changes are complete.
*/
func (service *TaskService) CompleteTask(ctx context.Context, taskID string) (inbound.TaskView, error) {
	task, err := service.repository.FindByID(ctx, taskID)
	if err != nil {
		return inbound.TaskView{}, err
	}

	if err := task.Complete(service.clock.Now()); err != nil {
		return inbound.TaskView{}, err
	}

	if err := service.repository.Save(ctx, task); err != nil {
		return inbound.TaskView{}, fmt.Errorf("save completed task: %w", err)
	}

	/*
		Build the domain event from the aggregate snapshot.

		The event carries only the fields that consumers are likely to need.
		It is a deliberate subset of the aggregate's full state — not a raw
		dump of the database row.
	*/
	snapshot := task.Snapshot()
	event := outbound.TaskCompletedEvent{
		TaskID:      snapshot.ID,
		Title:       snapshot.Title,
		CompletedAt: *snapshot.CompletedAt,
	}

	if err := service.eventPublisher.PublishTaskCompleted(ctx, event); err != nil {
		return inbound.TaskView{}, fmt.Errorf("publish task completed event: %w", err)
	}

	return toTaskView(task), nil
}

/*
ListTasks is a query use case.

Even though the logic is simple, the method still goes through the same
port boundary as the command use cases.
*/
func (service *TaskService) ListTasks(ctx context.Context) ([]inbound.TaskView, error) {
	tasks, err := service.repository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	views := make([]inbound.TaskView, 0, len(tasks))
	for _, task := range tasks {
		views = append(views, toTaskView(task))
	}

	return views, nil
}

/*
toTaskView maps a domain aggregate into a use-case response DTO.

This keeps adapters from needing to understand aggregate internals.
*/
func toTaskView(task domain.Task) inbound.TaskView {
	snapshot := task.Snapshot()

	return inbound.TaskView{
		ID:          snapshot.ID,
		Title:       snapshot.Title,
		Description: snapshot.Description,
		Status:      snapshot.Status,
		CreatedAt:   snapshot.CreatedAt,
		StartedAt:   snapshot.StartedAt,
		CompletedAt: snapshot.CompletedAt,
	}
}
