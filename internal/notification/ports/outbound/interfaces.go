package outbound

import (
	"context"
	"time"

	"go-example/internal/notification/domain"
)

/*
NotificationSender is the outbound port for physically delivering notifications.

The notification application service knows how to BUILD a Notification value
using domain rules. But it delegates actual delivery to an adapter via this port.

Possible concrete adapters that could satisfy this interface:
  - console/log adapter (what this example uses): writes to stdout via slog
  - email adapter: sends via SMTP using net/smtp or a third-party library
  - push notification adapter: calls FCM or APNs APIs
  - Slack adapter: posts to a channel via a webhook HTTP call
  - message queue adapter: publishes to RabbitMQ or Kafka for async delivery

The notification module's core NEVER changes when you swap adapters.
That is the "plug and play infrastructure" promise of hexagonal architecture.
*/
type NotificationSender interface {
	Send(ctx context.Context, notification domain.Notification) error
}

/*
IDGenerator is the outbound port for creating notification identifiers.

You will notice this interface has the exact same shape as the IDGenerator
interface in task/ports/outbound. That is intentional.

Each module defines its own copy of the interfaces it needs.
There is no shared "common" package. This keeps modules truly independent:
  - You can change the task ID generation strategy without
    affecting the notification module at all.
  - You can test each module's service with its own fake ID generator.

Go's structural typing (duck typing) means the same concrete type
(system.RandomIDGenerator) satisfies the IDGenerator interface in BOTH modules
without any shared package. The two interfaces are not linked at the type level;
they just happen to have the same method signature.
*/
type IDGenerator interface {
	NewID() (string, error)
}

/*
Clock is the outbound port for time.

Having Clock here (instead of calling time.Now() directly) makes the
notification service deterministic in tests: inject a fixedClock and the
sentAt timestamps become predictable and assertable.

This is the same reasoning used in task/ports/outbound.
Reproducible tests are worth the small extra indirection.
*/
type Clock interface {
	Now() time.Time
}
