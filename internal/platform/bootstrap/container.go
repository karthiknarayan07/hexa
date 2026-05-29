package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	notificationSQLite "hexa/internal/notification/adapters/out/sqlite"
	taskAPI "hexa/internal/task/adapters/in/api"
	taskSQLite "hexa/internal/task/adapters/out/sqlite"
	systemAdapters "hexa/internal/task/adapters/out/system"
	taskApp "hexa/internal/task/application"
	taskOut "hexa/internal/task/ports/outbound"

	"hexa/internal/notification/adapters/out/console"
	notificationApp "hexa/internal/notification/application"
	notificationIn "hexa/internal/notification/ports/inbound"

	_ "modernc.org/sqlite"
)

// CLIContainer holds only what CLI commands need: application services and infra.
// No transport adapters (HTTP handler, gRPC server, etc.) are created.
type CLIContainer struct {
	TaskService         *taskApp.TaskService
	NotificationService *notificationApp.NotificationService

	database *sql.DB
}

// Close releases infrastructure resources.
func (c *CLIContainer) Close() error {
	if c.database == nil {
		return nil
	}
	return c.database.Close()
}

// BuildForCLI wires infra and application services only.
// Safe to call from interactive CLI commands — starts no network listeners.
func BuildForCLI(options BuildOptions) (*CLIContainer, error) {
	database, err := openDatabase(options)
	if err != nil {
		return nil, err
	}

	taskService, notificationService, err := buildCoreServices(database)
	if err != nil {
		database.Close()
		return nil, err
	}

	return &CLIContainer{
		TaskService:         taskService,
		NotificationService: notificationService,
		database:            database,
	}, nil
}

// BuildOptions controls composition-root construction.
type BuildOptions struct {
	DatabasePath string
}

// RuntimeAddresses holds transport addresses.
type RuntimeAddresses struct {
	HTTP string
	GRPC string
}

// Container is the server composition root object graph.
// Holds transport handlers in addition to application services.
type Container struct {
	TaskService         *taskApp.TaskService
	NotificationService *notificationApp.NotificationService
	HTTPHandler         http.Handler

	database *sql.DB
}

// Close releases infrastructure resources.
func (container *Container) Close() error {
	if container.database == nil {
		return nil
	}

	return container.database.Close()
}

// BuildForServer creates infrastructure, modules, bridges, and all transport
// adapters (HTTP handler, etc.). Use this for `app run`.
func BuildForServer(options BuildOptions) (*Container, error) {
	database, err := openDatabase(options)
	if err != nil {
		return nil, err
	}

	taskService, notificationService, err := buildCoreServices(database)
	if err != nil {
		database.Close()
		return nil, err
	}

	taskHTTPAdapter := taskAPI.NewTaskAPI(
		taskService,
		taskService,
		taskService,
		taskService,
		taskService,
		taskService,
		taskService,
	)

	return &Container{
		TaskService:         taskService,
		NotificationService: notificationService,
		HTTPHandler:         taskHTTPAdapter.Routes(),
		database:            database,
	}, nil
}

// openDatabase is a shared infra helper used by both build paths.
func openDatabase(options BuildOptions) (*sql.DB, error) {
	databasePath := options.DatabasePath
	if databasePath == "" {
		databasePath = filepath.Join("data", "backlog.db")
	}

	if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	databaseDSN := fmt.Sprintf("file:%s", filepath.ToSlash(databasePath))
	database, err := sql.Open("sqlite", databaseDSN)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	database.SetMaxOpenConns(1)
	return database, nil
}

// buildCoreServices wires application modules and the cross-module bridge.
// Used by both server and CLI paths — no transport code here.
func buildCoreServices(database *sql.DB) (*taskApp.TaskService, *notificationApp.NotificationService, error) {
	notificationRepository, err := notificationSQLite.NewSQLiteNotificationRepository(database)
	if err != nil {
		return nil, nil, fmt.Errorf("build notification repository: %w", err)
	}

	consoleSender := console.NewConsoleNotificationSender()
	notificationService := notificationApp.NewNotificationService(
		consoleSender,
		notificationRepository,
		systemAdapters.RandomIDGenerator{},
		systemAdapters.SystemClock{},
	)

	bridge := &taskCompletionBridge{notifier: notificationService}

	taskRepository, err := taskSQLite.NewSQLiteTaskRepository(database)
	if err != nil {
		return nil, nil, fmt.Errorf("build task repository: %w", err)
	}

	taskService := taskApp.NewTaskService(
		taskRepository,
		systemAdapters.RandomIDGenerator{},
		systemAdapters.SystemClock{},
		bridge,
	)

	return taskService, notificationService, nil
}

// Mode identifies one runtime component to start.
type Mode string

const (
	ModeHTTP   Mode = "http"
	ModeGRPC   Mode = "grpc"
	ModeWorker Mode = "worker"
	ModeCron   Mode = "cron"
)

var allModes = []Mode{ModeHTTP, ModeGRPC, ModeWorker, ModeCron}

// ParseModes validates and normalizes selected runtime modes.
func ParseModes(values []string) ([]Mode, error) {
	if len(values) == 0 {
		return append([]Mode(nil), allModes...), nil
	}

	modeSet := make(map[Mode]struct{})
	modes := make([]Mode, 0, len(values))

	for _, value := range values {
		candidate := Mode(value)
		switch candidate {
		case ModeHTTP, ModeGRPC, ModeWorker, ModeCron:
			if _, seen := modeSet[candidate]; seen {
				continue
			}
			modeSet[candidate] = struct{}{}
			modes = append(modes, candidate)
		default:
			return nil, fmt.Errorf("unsupported mode %q", value)
		}
	}

	return modes, nil
}

// RunSelected starts all selected runtime components and blocks until exit.
func RunSelected(ctx context.Context, container *Container, modes []Mode, addresses RuntimeAddresses) error {
	if len(modes) == 0 {
		return fmt.Errorf("no runtime modes selected")
	}

	if addresses.HTTP == "" {
		addresses.HTTP = ":8080"
	}
	if addresses.GRPC == "" {
		addresses.GRPC = ":9090"
	}

	childContext, cancel := context.WithCancel(ctx)
	defer cancel()

	errorChannel := make(chan error, len(modes))

	for _, mode := range modes {
		mode := mode
		go func() {
			var err error
			switch mode {
			case ModeHTTP:
				err = runHTTP(childContext, container.HTTPHandler, addresses.HTTP)
			case ModeGRPC:
				err = runPlaceholder(childContext, "grpc", addresses.GRPC)
			case ModeWorker:
				err = runPlaceholder(childContext, "worker", "n/a")
			case ModeCron:
				err = runPlaceholder(childContext, "cron", "n/a")
			}

			errorChannel <- err
		}()
	}

	var firstError error
	for range modes {
		err := <-errorChannel
		if err != nil && firstError == nil {
			firstError = err
			cancel()
		}
	}

	return firstError
}

func runHTTP(ctx context.Context, handler http.Handler, address string) error {
	server := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errorChannel := make(chan error, 1)
	go func() {
		errorChannel <- server.ListenAndServe()
	}()

	slog.Info("runtime started", "mode", "http", "addr", address)

	select {
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}

		err := <-errorChannel
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("serve http api: %w", err)
		}

		return nil
	case err := <-errorChannel:
		if err == nil || err == http.ErrServerClosed {
			return nil
		}
		return fmt.Errorf("serve http api: %w", err)
	}
}

func runPlaceholder(ctx context.Context, mode string, address string) error {
	slog.Info("runtime started", "mode", mode, "addr", address)
	<-ctx.Done()
	return nil
}

type taskCompletionBridge struct {
	notifier notificationIn.SendTaskCompletionNotificationUseCase
}

func (bridge *taskCompletionBridge) PublishTaskCompleted(ctx context.Context, event taskOut.TaskCompletedEvent) error {
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
