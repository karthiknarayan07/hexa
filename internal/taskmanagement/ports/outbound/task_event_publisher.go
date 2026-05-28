package outbound

import (
	"context"
	"time"
)

/*
TaskCompletedEvent is a domain event DTO.

A domain event records that something significant happened inside the domain.
This one captures the exact moment a task transitioned to the COMPLETED state.

Key design decision: the DTO lives in the OUTBOUND PORTS of the taskmanagement
module — not in the notification module, not in a shared package.
The module that produces the event owns its definition.

Consumers of this event (like the bridge adapter in main.go) translate it
into whatever shape the receiving module expects. That translation is the
adapter's job, not the domain's job.
*/
type TaskCompletedEvent struct {
	/*
		TaskID is the unique identifier of the completed task.
		Consumers can use this to look up additional details if needed.
	*/
	TaskID string

	/*
		Title is included in the event so consumers can act without an extra
		round-trip to the database. Denormalizing a few fields into events is
		a common and practical DDD pattern.
	*/
	Title string

	/*
		CompletedAt records the exact moment the DOMAIN registered completion,
		not the time the event was delivered. Using the domain clock keeps
		timestamps meaningful even if delivery is delayed.
	*/
	CompletedAt time.Time
}

/*
TaskEventPublisher is an outbound port for domain event delivery.

The application service calls this port after a significant state change.
The concrete adapter that satisfies this port decides what happens next:

  - In this example: a bridge adapter translates the event and calls
    the notification module's inbound port.
  - In production: you might publish to a Kafka topic, write to an event
    store, update a read model, or trigger a saga.

The taskmanagement module is completely oblivious to which adapter is used.
Swapping from the bridge adapter to a Kafka publisher requires zero changes
anywhere inside the taskmanagement package tree.

This is the "protect the core from infrastructure" promise of hexagonal architecture.
*/
type TaskEventPublisher interface {
	PublishTaskCompleted(ctx context.Context, event TaskCompletedEvent) error
}
