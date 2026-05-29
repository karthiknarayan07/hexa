package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	// task module — every import from this module is prefixed with
	// "task" in the alias so it is obvious which module each name belongs to.
	"go-example/internal/task/adapters/in/api"
	"go-example/internal/task/adapters/out/sqlite"
	"go-example/internal/task/adapters/out/system"
	taskApp "go-example/internal/task/application"
	taskOut "go-example/internal/task/ports/outbound"

	// notification module — similarly prefixed with "notification".
	"go-example/internal/notification/adapters/out/console"
	notificationApp "go-example/internal/notification/application"
	notificationIn "go-example/internal/notification/ports/inbound"

	_ "modernc.org/sqlite"
)

/*
main is the composition root of the entire application.

"Composition root" means: the single place that is allowed to know about
every module, every adapter, and every concrete type. Everything else in the
codebase depends only on interfaces.

Think of this file as the electrical wiring panel of the house. Every room
(module) has its own circuits (ports), and this panel connects them to the
power source (infrastructure). The rooms do not know how the panel works;
the panel does not care what the rooms do with the electricity.
*/
func main() {
	if err := run(); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

/*
run builds the complete object graph and starts the HTTP server.

The wiring order matters and teaches something important:

 1. Infrastructure first  — things that have no dependencies on modules
    (database connection, console sender)
 2. Leaf modules next     — modules that produce nothing for other modules
    (notification: only receives, never calls out to modules)
 3. Dependent modules     — modules that use other modules via the bridge
    (task: calls notification through the bridge)
 4. Transport adapters last — the outermost layer that faces the network
    (HTTP server)

This inside-out order ensures every dependency is ready before the thing that
needs it is created. It also reveals the dependency graph at a glance.
*/
func run() error {

	// =========================================================================
	// Step 1 — Infrastructure
	// =========================================================================

	databasePath := filepath.Join("data", "backlog.db")
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}

	databaseDSN := fmt.Sprintf("file:%s", filepath.ToSlash(databasePath))
	database, err := sql.Open("sqlite", databaseDSN)
	if err != nil {
		return fmt.Errorf("open sqlite database: %w", err)
	}
	defer database.Close()

	database.SetMaxOpenConns(1)

	// =========================================================================
	// Step 2 — Notification module
	//
	// We build the notification module before task because
	// task will hold a reference to the notification module through
	// the bridge adapter (see Step 3).
	//
	// Important: this section knows nothing about task.
	// The notification module is fully self-contained. It receives a sender,
	// an ID generator, and a clock — all from its own outbound ports.
	// =========================================================================

	/*
		consoleSender satisfies notification's outbound NotificationSender port.
		In a real system you would swap this for an EmailSender or SlackSender
		without touching a single line inside the notification package.
	*/
	consoleSender := console.NewConsoleNotificationSender()

	/*
		system.RandomIDGenerator{} satisfies BOTH modules' IDGenerator interfaces.
		No shared package is needed because Go uses structural typing:
		if a type has the right method signature, it satisfies the interface,
		regardless of which package the interface is declared in.
	*/
	notificationService := notificationApp.NewNotificationService(
		consoleSender,
		system.RandomIDGenerator{},
		system.SystemClock{},
	)

	// =========================================================================
	// Step 3 — Cross-module bridge adapter
	//
	// The bridge is the architectural centrepiece of this example.
	// Read the taskCompletionBridge comment below for the full explanation.
	// =========================================================================

	bridge := &taskCompletionBridge{notifier: notificationService}

	// =========================================================================
	// Step 4 — Task module
	//
	// Now we build task. It receives the bridge as its event publisher.
	// The task module does not know it is talking to notification;
	// it only knows it has something that satisfies TaskEventPublisher.
	// =========================================================================

	taskRepository, err := sqlite.NewSQLiteTaskRepository(database)
	if err != nil {
		return fmt.Errorf("build task repository: %w", err)
	}

	taskService := taskApp.NewTaskService(
		taskRepository,
		system.RandomIDGenerator{},
		system.SystemClock{},
		bridge, // <-- bridge satisfies task's TaskEventPublisher outbound port
	)

	// =========================================================================
	// Step 5 — HTTP inbound adapter
	//
	// The HTTP adapter is the outermost inbound layer. It is built last because
	// it depends on the application service being fully wired already.
	// =========================================================================

	taskAPI := api.NewTaskAPI(
		taskService,
		taskService,
		taskService,
		taskService,
	)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           taskAPI.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("starting service", "addr", server.Addr, "database", databasePath)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serve http api: %w", err)
	}

	return nil
}

// =============================================================================
// Cross-module bridge adapter
//
// This struct is the answer to the question:
// "How do two modules communicate if neither is allowed to import the other?"
//
// The answer: a bridge adapter in the composition root translates between
// the two modules' port contracts. The composition root IS allowed to know
// about both modules — it is the only place with that permission.
// =============================================================================

/*
taskCompletionBridge is a composition-root adapter that connects the
task module to the notification module.

The problem it solves:

	task wants to say: "a task was just completed, do something."
	notification wants to hear: "please send a task-completion notification."
	Neither module knows the other exists. How do we connect them?

The hexagonal answer:

	Each module defines its own port in its own vocabulary:
	  - task defines TaskEventPublisher (outbound) with TaskCompletedEvent
	  - notification defines SendTaskCompletionNotificationUseCase (inbound) with
	    SendTaskCompletionNotificationCommand

	A bridge adapter (this struct) lives in the composition root and:
	  1. Implements task's outbound port so task can call it.
	  2. Holds a reference to notification's inbound port to forward the call.
	  3. Translates the event DTO into the command DTO in between.

Dependency arrows (→ means "source-code depends on"):

	task  →  TaskEventPublisher  ←  taskCompletionBridge
	taskCompletionBridge  →  SendTaskCompletionNotificationUseCase  ←  notification

Both modules point inward to their own interfaces.
This bridge points to both interfaces but to neither module's internals.

If you want a second consumer of the task-completed event (e.g. an analytics
module), you add a second bridge and compose them — no changes inside either
existing module.
*/
type taskCompletionBridge struct {
	/*
		notifier is the notification module's inbound use-case port.

		The bridge holds an interface, not a concrete *NotificationService.
		This means the bridge itself is testable: inject a fake notifier and
		you can verify translation logic without involving any real module.
	*/
	notifier notificationIn.SendTaskCompletionNotificationUseCase
}

/*
PublishTaskCompleted satisfies the taskOut.TaskEventPublisher interface.

This method is called by task.TaskService.CompleteTask after a task
is successfully persisted. The bridge receives the domain event and translates
it into the notification module's command format before forwarding.

Adapter code should look exactly like this:
  - map field names between two different vocabularies
  - compose derived values (like the human-readable message string)
  - call the downstream port
  - no business logic, no domain rules
*/
func (bridge *taskCompletionBridge) PublishTaskCompleted(ctx context.Context, event taskOut.TaskCompletedEvent) error {
	/*
		Translate task vocabulary → notification vocabulary.

		task produced: TaskCompletedEvent{TaskID, Title, CompletedAt}
		notification expects:    SendTaskCompletionNotificationCommand{TaskID, TaskTitle, Message, Recipient}

		The bridge owns this mapping. Neither module needs to agree on field names.
		If task renames "Title" to "Name", only this file changes.
		If notification renames "TaskTitle" to "TaskName", only this file changes.
	*/
	_, err := bridge.notifier.SendTaskCompletionNotification(ctx,
		notificationIn.SendTaskCompletionNotificationCommand{
			TaskID:    event.TaskID,
			TaskTitle: event.Title,
			Message: fmt.Sprintf(
				"Task '%s' was completed at %s.",
				event.Title,
				event.CompletedAt.Format(time.RFC3339),
			),
			Recipient: "system-log",
		},
	)
	return err
}
