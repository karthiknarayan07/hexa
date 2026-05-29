package application

import (
	"context"
	"fmt"

	"go-example/internal/notification/domain"
	"go-example/internal/notification/ports/inbound"
	"go-example/internal/notification/ports/outbound"
)

/*
Compile-time interface satisfaction check.

If NotificationService ever drifts out of alignment with the inbound port
(for example, if the method signature changes in one place but not the other),
this line makes the build fail here with a clear error message instead of
failing at the call site in main.go — or worse, at runtime.

This is a Go idiom that every application service in a hexagonal codebase
should use. Compare with the same pattern in task/application/task_service.go.
*/
var _ inbound.SendTaskCompletionNotificationUseCase = (*NotificationService)(nil)

/*
NotificationService is the application service for the notification module.

Its role is identical to what TaskService plays in task:
  - Receives commands through inbound ports (called by adapters or bridges)
  - Builds domain values that enforce business rules
  - Delegates infrastructure concerns to outbound ports

The structural similarity between the two services is deliberate.
When you understand one, you immediately understand the other.
Every module in a hexagonal system follows this same shape.

Key difference: this service does NOT know it was triggered by a task
lifecycle event. It knows only that someone called its inbound port with
a SendTaskCompletionNotificationCommand. That deliberate ignorance is what
keeps modules decoupled. The notification module can be tested, extended,
or replaced without touching task at all.
*/
type NotificationService struct {
	sender      outbound.NotificationSender
	idGenerator outbound.IDGenerator
	clock       outbound.Clock
}

/*
NewNotificationService creates the notification application service.

The same dependency-injection pattern as task.NewTaskService:
all parameters are interfaces from the module's own outbound ports.
The composition root (main.go) satisfies them with concrete adapters.
*/
func NewNotificationService(
	sender outbound.NotificationSender,
	idGenerator outbound.IDGenerator,
	clock outbound.Clock,
) *NotificationService {
	return &NotificationService{
		sender:      sender,
		idGenerator: idGenerator,
		clock:       clock,
	}
}

/*
SendTaskCompletionNotification handles the notification use case.

Data flow through this method:
 1. Generate a unique ID for this notification via the IDGenerator port.
 2. Build a domain.Notification value — domain rules apply here.
 3. Deliver it via the NotificationSender port — infrastructure concern.
 4. Return a view DTO to the caller (the bridge adapter in main.go).

This method is reached indirectly from the task module:

	task.TaskService.CompleteTask
	  → taskOutbound.TaskEventPublisher.PublishTaskCompleted
	    → taskCompletionBridge.PublishTaskCompleted  (in cmd/api/main.go)
	      → notificationInbound.SendTaskCompletionNotificationUseCase
	        → here

Neither the bridge nor this service holds a reference to the other module.
All communication is through interfaces. The composition root is the
only code that connects them.
*/
func (service *NotificationService) SendTaskCompletionNotification(
	ctx context.Context,
	cmd inbound.SendTaskCompletionNotificationCommand,
) (inbound.NotificationView, error) {
	id, err := service.idGenerator.NewID()
	if err != nil {
		return inbound.NotificationView{}, fmt.Errorf("generate notification id: %w", err)
	}

	/*
		Build the Notification domain value.

		The subject line is composed here in the application service because
		formatting a human-readable subject from a command field is application
		logic, not domain logic. The domain only validates that the subject
		is non-empty once it arrives.
	*/
	notification, err := domain.NewNotification(
		id,
		fmt.Sprintf("Task Completed: %s", cmd.TaskTitle),
		cmd.Message,
		cmd.Recipient,
		service.clock.Now(),
	)
	if err != nil {
		return inbound.NotificationView{}, fmt.Errorf("build notification: %w", err)
	}

	if err := service.sender.Send(ctx, notification); err != nil {
		return inbound.NotificationView{}, fmt.Errorf("send notification: %w", err)
	}

	snapshot := notification.Snapshot()
	return inbound.NotificationView{
		ID:        snapshot.ID,
		Subject:   snapshot.Subject,
		Body:      snapshot.Body,
		Recipient: snapshot.Recipient,
	}, nil
}
