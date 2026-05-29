package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"hexa/internal/notification/domain"
	"hexa/internal/notification/ports/outbound"
)

// SQLiteNotificationRepository persists notifications to SQLite.
type SQLiteNotificationRepository struct {
	database *sql.DB
}

type notificationRow struct {
	ID        string
	Subject   string
	Body      string
	Recipient string
	SentAt    string
}

// NewSQLiteNotificationRepository builds the adapter and ensures schema exists.
func NewSQLiteNotificationRepository(database *sql.DB) (*SQLiteNotificationRepository, error) {
	repository := &SQLiteNotificationRepository{database: database}
	if err := repository.ensureSchema(); err != nil {
		return nil, err
	}
	return repository, nil
}

// Save upserts the notification snapshot.
func (repository *SQLiteNotificationRepository) Save(ctx context.Context, notification domain.Notification) error {
	snapshot := notification.Snapshot()

	_, err := repository.database.ExecContext(
		ctx,
		`INSERT INTO notifications (id, subject, body, recipient, sent_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   subject = excluded.subject,
		   body = excluded.body,
		   recipient = excluded.recipient,
		   sent_at = excluded.sent_at`,
		snapshot.ID,
		snapshot.Subject,
		snapshot.Body,
		snapshot.Recipient,
		formatTime(snapshot.SentAt),
	)
	if err != nil {
		return fmt.Errorf("upsert notification: %w", err)
	}

	return nil
}

// FindByID loads one notification by id.
func (repository *SQLiteNotificationRepository) FindByID(ctx context.Context, notificationID string) (domain.Notification, error) {
	row := repository.database.QueryRowContext(
		ctx,
		`SELECT id, subject, body, recipient, sent_at
		 FROM notifications
		 WHERE id = ?`,
		notificationID,
	)

	var databaseRow notificationRow
	if err := row.Scan(
		&databaseRow.ID,
		&databaseRow.Subject,
		&databaseRow.Body,
		&databaseRow.Recipient,
		&databaseRow.SentAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Notification{}, outbound.ErrNotificationNotFound
		}
		return domain.Notification{}, fmt.Errorf("query notification by id: %w", err)
	}

	notification, err := databaseRow.toDomainNotification()
	if err != nil {
		return domain.Notification{}, fmt.Errorf("rebuild notification from row: %w", err)
	}

	return notification, nil
}

// Delete removes one notification by id.
func (repository *SQLiteNotificationRepository) Delete(ctx context.Context, notificationID string) error {
	result, err := repository.database.ExecContext(
		ctx,
		`DELETE FROM notifications WHERE id = ?`,
		notificationID,
	)
	if err != nil {
		return fmt.Errorf("delete notification: %w", err)
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted rows: %w", err)
	}

	if affectedRows == 0 {
		return outbound.ErrNotificationNotFound
	}

	return nil
}

// List returns all notifications ordered by sent time descending.
func (repository *SQLiteNotificationRepository) List(ctx context.Context) ([]domain.Notification, error) {
	rows, err := repository.database.QueryContext(
		ctx,
		`SELECT id, subject, body, recipient, sent_at
		 FROM notifications
		 ORDER BY sent_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query notification list: %w", err)
	}
	defer rows.Close()

	notifications := make([]domain.Notification, 0)
	for rows.Next() {
		var databaseRow notificationRow
		if err := rows.Scan(
			&databaseRow.ID,
			&databaseRow.Subject,
			&databaseRow.Body,
			&databaseRow.Recipient,
			&databaseRow.SentAt,
		); err != nil {
			return nil, fmt.Errorf("scan notification row: %w", err)
		}

		notification, err := databaseRow.toDomainNotification()
		if err != nil {
			return nil, fmt.Errorf("rebuild notification from row: %w", err)
		}

		notifications = append(notifications, notification)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notification rows: %w", err)
	}

	return notifications, nil
}

func (repository *SQLiteNotificationRepository) ensureSchema() error {
	_, err := repository.database.Exec(`CREATE TABLE IF NOT EXISTS notifications (
		id TEXT PRIMARY KEY,
		subject TEXT NOT NULL,
		body TEXT NOT NULL,
		recipient TEXT NOT NULL,
		sent_at TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("ensure notifications schema: %w", err)
	}

	return nil
}

func (row notificationRow) toDomainNotification() (domain.Notification, error) {
	sentAt, err := time.Parse(time.RFC3339Nano, row.SentAt)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("parse sent_at: %w", err)
	}

	return domain.NewNotification(
		row.ID,
		row.Subject,
		row.Body,
		row.Recipient,
		sentAt,
	)
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
