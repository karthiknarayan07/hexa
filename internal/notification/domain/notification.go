package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrNotificationIDRequired        = errors.New("notification id is required")
	ErrNotificationSubjectRequired   = errors.New("notification subject is required")
	ErrNotificationBodyRequired      = errors.New("notification body is required")
	ErrNotificationRecipientRequired = errors.New("notification recipient is required")
)

/*
Notification is the aggregate root of the notification module.

It represents the record of a message that was built and delivered to a recipient.

Compare this domain object to the Task aggregate in task:
  - Task has a rich lifecycle: PLANNED → IN_PROGRESS → COMPLETED.
    It must guard against invalid transitions and carries time ranges.
  - Notification is simpler: it is created once, delivered once, and done.
    Its invariants are just "all required fields must be present".

Having a domain layer here, even for a simple concept, keeps the notification
module from devolving into a "bag of functions that call slog".
The domain layer makes the concept explicit, names it clearly, and gives
it a home that is independent of how it gets delivered.
*/
type Notification struct {
	id        string
	subject   string
	body      string
	recipient string
	sentAt    time.Time
}

/*
NotificationSnapshot is the read-only view of a Notification value.

The same encapsulation pattern is used in every module in this codebase:
private fields, public snapshot. Outer layers read data through the snapshot;
they cannot mutate it by poking fields directly.

This keeps each aggregate in full control of its own invariants.
*/
type NotificationSnapshot struct {
	ID        string
	Subject   string
	Body      string
	Recipient string
	SentAt    time.Time
}

/*
NewNotification creates a valid Notification value ready to be sent.

The factory function centralises validation so that it is impossible to
create an invalid Notification anywhere in the codebase: you can only get
one through this constructor.

Notice it returns (Notification, error) — the same Go pattern used in
domain.NewTask. Every constructor in the domain layer should follow this
pattern so callers are forced to handle the case where creation fails.
*/
func NewNotification(id, subject, body, recipient string, sentAt time.Time) (Notification, error) {
	id = strings.TrimSpace(id)
	subject = strings.TrimSpace(subject)
	body = strings.TrimSpace(body)
	recipient = strings.TrimSpace(recipient)

	if id == "" {
		return Notification{}, ErrNotificationIDRequired
	}
	if subject == "" {
		return Notification{}, ErrNotificationSubjectRequired
	}
	if body == "" {
		return Notification{}, ErrNotificationBodyRequired
	}
	if recipient == "" {
		return Notification{}, ErrNotificationRecipientRequired
	}

	return Notification{
		id:        id,
		subject:   subject,
		body:      body,
		recipient: recipient,
		sentAt:    sentAt.UTC(),
	}, nil
}

/*
Snapshot returns an immutable copy of the notification's state.

This is the only way code outside this package can read the fields.
*/
func (n Notification) Snapshot() NotificationSnapshot {
	return NotificationSnapshot{
		ID:        n.id,
		Subject:   n.subject,
		Body:      n.body,
		Recipient: n.recipient,
		SentAt:    n.sentAt,
	}
}
