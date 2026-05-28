package inbound
package inbound

import "context"

/*
SendTaskCompletionNotificationCommand is the input DTO for the notification
use case triggered by a task completion.

This DTO lives in the INBOUND PORTS of the notification module.
It is the shape callers must use to request a notification — there is no
other way to drive the notification module from outside.

Compare this to CreateTaskCommand in taskmanagement/ports/inbound:
  - Both are plain structs with no methods.
  - Both belong to the boundary of their own module.
  - Neither leaks domain internals to callers.

The bridge adapter in cmd/api/main.go builds this struct when translating
the taskmanagement.TaskCompletedEvent into a notification command.
That adapter is the ONLY place that knows about both modules.
*/
type SendTaskCompletionNotificationCommand struct {
	// TaskID is passed so the notification can reference the source task.
	TaskID string

	// TaskTitle is the human-readable name of the completed task.
	TaskTitle string

	// Message is the body text of the notification to be delivered.
	Message string

	// Recipient is who or what should receive the notification.
	// In this example it is always "system-log"; in a real system it
	// might be a user email address or a Slack channel ID.
	Recipient string
}

/*
NotificationView is the response DTO returned after a notification is processed.

The view is what the caller (the bridge adapter) receives back.
In this example the caller ignores it, but it is good practice to return
a view from every use case so callers can observe what happened.
*/
type NotificationView struct {
	ID        string `json:"id"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Recipient string `json:"recipient"`
}

/*
SendTaskCompletionNotificationUseCase is the inbound port of the notification module.

Any code that wants to trigger a notification must depend on this interface,
not on the concrete NotificationService struct.

This mirrors the same pattern used in taskmanagement:
  - taskmanagement exposes CreateTaskUseCase, StartTaskUseCase, etc.
  - notification exposes SendTaskCompletionNotificationUseCase

Every module that exposes behaviour to the outside world does so through
inbound ports. The outside world never gets a direct reference to the
concrete service.

The bridge adapter in cmd/api/main.go holds a value of this interface type,
which means you can swap the entire notification implementation (switch from
console logging to email delivery) without touching the bridge adapter at all.
*/
type SendTaskCompletionNotificationUseCase interface {
	SendTaskCompletionNotification(ctx context.Context, cmd SendTaskCompletionNotificationCommand) (NotificationView, error)
}
