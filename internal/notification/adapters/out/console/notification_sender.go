package console

import (
	"context"
	"log/slog"

	"go-example/internal/notification/domain"
)

/*
ConsoleNotificationSender is the outbound adapter for the notification module's NotificationSender port.

It satisfies the port by writing structured log entries to stdout using slog.

This is the simplest possible driven adapter: no network calls, no database
writes, no external dependencies beyond the standard library.

Why does this file exist in adapters/out/console and not directly in application/?
Because the delivery mechanism is an infrastructure concern, not a business concern.
The application service builds the Notification value and decides what to send;
this adapter decides HOW to physically deliver it.

Swap examples — none of these require changes inside the notification module:
  - Replace ConsoleNotificationSender with EmailSender to deliver via SMTP.
  - Replace ConsoleNotificationSender with SlackSender to post to a channel.
  - Replace ConsoleNotificationSender with QueueSender to publish to RabbitMQ.

All you change is which adapter you wire in main.go.
The NotificationService, NotificationDomain, and ports stay exactly the same.
*/
type ConsoleNotificationSender struct{}

/*
NewConsoleNotificationSender creates the console delivery adapter.
*/
func NewConsoleNotificationSender() *ConsoleNotificationSender {
	return &ConsoleNotificationSender{}
}

/*
Send writes the notification as a structured log entry.

The method receives the full domain.Notification value from the application
service. It reads the data through Snapshot() — the only public view of
the domain object's private fields — and formats a structured slog message.

Notice: the adapter does no business-rule checking. It trusts that the
application service already validated the notification through the domain
constructor. The adapter's only job is delivery.
*/
func (sender *ConsoleNotificationSender) Send(_ context.Context, notification domain.Notification) error {
	snapshot := notification.Snapshot()

	slog.Info("notification delivered",
		"notification_id", snapshot.ID,
		"subject", snapshot.Subject,
		"body", snapshot.Body,
		"recipient", snapshot.Recipient,
		"sent_at", snapshot.SentAt,
	)

	return nil
}
