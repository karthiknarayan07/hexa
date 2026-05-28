package domain_test

import (
	"errors"
	"testing"
	"time"

	"go-example/internal/taskmanagement/domain"
)

func TestTaskLifecycle(t *testing.T) {
	createdAt := time.Date(2026, time.May, 28, 9, 0, 0, 0, time.UTC)
	startedAt := createdAt.Add(10 * time.Minute)
	completedAt := startedAt.Add(20 * time.Minute)

	task, err := domain.NewTask("task-1", "Learn the dependency rule", "Walk from adapter to aggregate", createdAt)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if err := task.Start(startedAt); err != nil {
		t.Fatalf("start task: %v", err)
	}

	if err := task.Complete(completedAt); err != nil {
		t.Fatalf("complete task: %v", err)
	}

	snapshot := task.Snapshot()
	if snapshot.Status != domain.TaskStatusCompleted {
		t.Fatalf("expected completed status, got %s", snapshot.Status)
	}

	if snapshot.StartedAt == nil || !snapshot.StartedAt.Equal(startedAt) {
		t.Fatalf("expected started_at to be recorded")
	}

	if snapshot.CompletedAt == nil || !snapshot.CompletedAt.Equal(completedAt) {
		t.Fatalf("expected completed_at to be recorded")
	}
}

func TestTaskCannotCompleteBeforeStart(t *testing.T) {
	createdAt := time.Date(2026, time.May, 28, 9, 0, 0, 0, time.UTC)

	task, err := domain.NewTask("task-2", "Break the lifecycle", "This should fail", createdAt)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	err = task.Complete(createdAt.Add(5 * time.Minute))
	if !errors.Is(err, domain.ErrTaskMustBeStartedFirst) {
		t.Fatalf("expected ErrTaskMustBeStartedFirst, got %v", err)
	}
}
