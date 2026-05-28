package sqlite_test

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
	"time"

	"go-example/internal/taskmanagement/adapters/out/sqlite"
	"go-example/internal/taskmanagement/domain"

	_ "modernc.org/sqlite"
)

func TestTaskRepositoryRoundTrip(t *testing.T) {
	databasePath := t.TempDir() + "/backlog.db"
	database, err := sql.Open("sqlite", "file:"+databasePath)
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	defer database.Close()

	repository, err := sqlite.NewTaskRepository(database)
	if err != nil {
		t.Fatalf("build repository: %v", err)
	}

	createdAt := time.Date(2026, time.May, 28, 9, 0, 0, 0, time.UTC)
	task, err := domain.NewTask("task-1", "Persist aggregate", "Round-trip through SQLite", createdAt)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if err := task.Start(createdAt.Add(5 * time.Minute)); err != nil {
		t.Fatalf("start task: %v", err)
	}

	if err := repository.Save(context.Background(), task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	loadedTask, err := repository.FindByID(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	if !reflect.DeepEqual(task.Snapshot(), loadedTask.Snapshot()) {
		t.Fatalf("expected loaded task to match saved task")
	}
}
