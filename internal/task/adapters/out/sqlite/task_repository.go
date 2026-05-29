package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go-example/internal/task/domain"
	"go-example/internal/task/ports/outbound"
)

/*
SQLiteTaskRepository is a SQLite implementation of the outbound repository port.

This adapter lives outside the core. It knows SQL, tables, and storage
concerns, but it does not decide business rules.
*/
type SQLiteTaskRepository struct {
	database *sql.DB
}

type taskRow struct {
	ID          string
	Title       string
	Description string
	Status      string
	CreatedAt   string
	StartedAt   sql.NullString
	CompletedAt sql.NullString
}

/*
NewSQLiteTaskRepository builds the adapter and ensures the table exists.

Schema management stays in the infrastructure side because it is a storage
concern, not an application or domain concern.
*/
func NewSQLiteTaskRepository(database *sql.DB) (*SQLiteTaskRepository, error) {
	repository := &SQLiteTaskRepository{database: database}

	if err := repository.ensureSchema(); err != nil {
		return nil, err
	}

	return repository, nil
}

/*
Save upserts the task snapshot into SQLite.

The adapter converts the aggregate snapshot into table columns,
which is the classic job of a driven adapter.
*/
func (repository *SQLiteTaskRepository) Save(ctx context.Context, task domain.Task) error {
	snapshot := task.Snapshot()

	_, err := repository.database.ExecContext(
		ctx,
		`INSERT INTO tasks (id, title, description, status, created_at, started_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   title = excluded.title,
		   description = excluded.description,
		   status = excluded.status,
		   created_at = excluded.created_at,
		   started_at = excluded.started_at,
		   completed_at = excluded.completed_at`,
		snapshot.ID,
		snapshot.Title,
		snapshot.Description,
		string(snapshot.Status),
		formatTime(snapshot.CreatedAt),
		formatOptionalTime(snapshot.StartedAt),
		formatOptionalTime(snapshot.CompletedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert task: %w", err)
	}

	return nil
}

/*
FindByID loads one task and reconstructs the aggregate through the domain.

Reconstruction still goes through domain validation so the adapter cannot
silently create an invalid aggregate.
*/
func (repository *SQLiteTaskRepository) FindByID(ctx context.Context, taskID string) (domain.Task, error) {
	row := repository.database.QueryRowContext(
		ctx,
		`SELECT id, title, description, status, created_at, started_at, completed_at
		 FROM tasks
		 WHERE id = ?`,
		taskID,
	)

	var databaseRow taskRow
	if err := row.Scan(
		&databaseRow.ID,
		&databaseRow.Title,
		&databaseRow.Description,
		&databaseRow.Status,
		&databaseRow.CreatedAt,
		&databaseRow.StartedAt,
		&databaseRow.CompletedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Task{}, outbound.ErrTaskNotFound
		}

		return domain.Task{}, fmt.Errorf("query task by id: %w", err)
	}

	task, err := databaseRow.toDomainTask()
	if err != nil {
		return domain.Task{}, fmt.Errorf("rebuild task from row: %w", err)
	}

	return task, nil
}

/*
List returns every task ordered by creation time.

The repository returns domain aggregates because the application layer still
works in domain concepts, not SQL row concepts.
*/
func (repository *SQLiteTaskRepository) List(ctx context.Context) ([]domain.Task, error) {
	rows, err := repository.database.QueryContext(
		ctx,
		`SELECT id, title, description, status, created_at, started_at, completed_at
		 FROM tasks
		 ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query task list: %w", err)
	}
	defer rows.Close()

	tasks := make([]domain.Task, 0)
	for rows.Next() {
		var databaseRow taskRow
		if err := rows.Scan(
			&databaseRow.ID,
			&databaseRow.Title,
			&databaseRow.Description,
			&databaseRow.Status,
			&databaseRow.CreatedAt,
			&databaseRow.StartedAt,
			&databaseRow.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan task row: %w", err)
		}

		task, err := databaseRow.toDomainTask()
		if err != nil {
			return nil, fmt.Errorf("rebuild task from row: %w", err)
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task rows: %w", err)
	}

	return tasks, nil
}

/*
ensureSchema creates the backing table when the application starts.

Keeping the schema in the adapter lets the example stay self-contained.
*/
func (repository *SQLiteTaskRepository) ensureSchema() error {
	_, err := repository.database.Exec(`CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		description TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at TEXT NOT NULL,
		started_at TEXT,
		completed_at TEXT
	)`)
	if err != nil {
		return fmt.Errorf("ensure tasks schema: %w", err)
	}

	return nil
}

/*
toDomainTask converts the storage representation back into a domain aggregate.

The adapter performs the technical parsing work, then hands the clean result
to the domain for validation and reconstruction.
*/
func (row taskRow) toDomainTask() (domain.Task, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return domain.Task{}, fmt.Errorf("parse created_at: %w", err)
	}

	startedAt, err := parseOptionalTime(row.StartedAt)
	if err != nil {
		return domain.Task{}, fmt.Errorf("parse started_at: %w", err)
	}

	completedAt, err := parseOptionalTime(row.CompletedAt)
	if err != nil {
		return domain.Task{}, fmt.Errorf("parse completed_at: %w", err)
	}

	return domain.RehydrateTask(domain.TaskSnapshot{
		ID:          row.ID,
		Title:       row.Title,
		Description: row.Description,
		Status:      domain.TaskStatus(row.Status),
		CreatedAt:   createdAt,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
	})
}

/*
formatTime keeps timestamp serialization explicit in the adapter.

The domain deals with `time.Time`, while SQLite persistence uses text.
*/
func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

/*
formatOptionalTime converts an optional timestamp into a nullable SQL value.
*/
func formatOptionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}

	return formatTime(*value)
}

/*
parseOptionalTime performs the reverse conversion for nullable timestamps.
*/
func parseOptionalTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}

	parsedValue, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil, err
	}

	return &parsedValue, nil
}
