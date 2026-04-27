package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CalendarRepository struct {
	pool *pgxpool.Pool
}

func NewCalendarRepository(pool *pgxpool.Pool) *CalendarRepository {
	return &CalendarRepository{pool: pool}
}

type CalendarEventRecord struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Title       string
	Description string
	Location    *string
	StartAt     time.Time
	EndAt       time.Time
	Reminders   []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CalendarEventInput struct {
	UserID      uuid.UUID
	Title       string
	Description string
	Location    *string
	StartAt     time.Time
	EndAt       time.Time
	Reminders   []string
}

type CalendarNotificationRecord struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Type      string
	Title     string
	Message   string
	Payload   map[string]any
	IsRead    bool
	ReadAt    *time.Time
	CreatedAt time.Time
}

func (r *CalendarRepository) ListEventsByUser(ctx context.Context, userID uuid.UUID, from *time.Time, to *time.Time) ([]CalendarEventRecord, error) {
	query := `
SELECT
    e.id,
    e.user_id,
    e.title,
    e.description,
    e.location,
    e.start_at,
    e.end_at,
    COALESCE(array_remove(array_agg(rem.reminder_type ORDER BY rem.remind_at), NULL), ARRAY[]::text[]) AS reminders,
    e.created_at,
    e.updated_at
FROM calendar_events e
LEFT JOIN calendar_event_reminders rem ON rem.event_id = e.id
WHERE e.user_id = $1
  AND e.deleted_at IS NULL
`
	args := []any{userID}
	argPos := 2
	if from != nil {
		query += fmt.Sprintf(" AND e.end_at >= $%d\n", argPos)
		args = append(args, *from)
		argPos += 1
	}
	if to != nil {
		query += fmt.Sprintf(" AND e.start_at <= $%d\n", argPos)
		args = append(args, *to)
		argPos += 1
	}
	query += `
GROUP BY e.id
ORDER BY e.start_at ASC, e.created_at ASC
`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list calendar events: %w", err)
	}
	defer rows.Close()

	out := make([]CalendarEventRecord, 0)
	for rows.Next() {
		event, err := scanCalendarEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan calendar event: %w", err)
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate calendar events: %w", err)
	}
	return out, nil
}

func (r *CalendarRepository) GetEventByIDForUser(ctx context.Context, userID, eventID uuid.UUID) (CalendarEventRecord, error) {
	const query = `
SELECT
    e.id,
    e.user_id,
    e.title,
    e.description,
    e.location,
    e.start_at,
    e.end_at,
    COALESCE(array_remove(array_agg(rem.reminder_type ORDER BY rem.remind_at), NULL), ARRAY[]::text[]) AS reminders,
    e.created_at,
    e.updated_at
FROM calendar_events e
LEFT JOIN calendar_event_reminders rem ON rem.event_id = e.id
WHERE e.user_id = $1
  AND e.id = $2
  AND e.deleted_at IS NULL
GROUP BY e.id
LIMIT 1
`
	row := r.pool.QueryRow(ctx, query, userID, eventID)
	event, err := scanCalendarEvent(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CalendarEventRecord{}, pgx.ErrNoRows
		}
		return CalendarEventRecord{}, fmt.Errorf("get calendar event by id: %w", err)
	}
	return event, nil
}

func (r *CalendarRepository) CreateEvent(ctx context.Context, input CalendarEventInput) (CalendarEventRecord, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CalendarEventRecord{}, fmt.Errorf("begin create calendar event tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const createQuery = `
INSERT INTO calendar_events (user_id, title, description, location, start_at, end_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now(), now())
RETURNING id, user_id, title, description, location, start_at, end_at, created_at, updated_at
`
	var (
		event       CalendarEventRecord
		locationRaw sql.NullString
	)
	if err := tx.QueryRow(ctx, createQuery,
		input.UserID,
		strings.TrimSpace(input.Title),
		strings.TrimSpace(input.Description),
		normalizeOptionalString(input.Location),
		input.StartAt,
		input.EndAt,
	).Scan(&event.ID, &event.UserID, &event.Title, &event.Description, &locationRaw, &event.StartAt, &event.EndAt, &event.CreatedAt, &event.UpdatedAt); err != nil {
		return CalendarEventRecord{}, fmt.Errorf("insert calendar event: %w", err)
	}
	if locationRaw.Valid {
		value := strings.TrimSpace(locationRaw.String)
		event.Location = &value
	}

	if err := upsertEventReminders(ctx, tx, event.ID, input.UserID, event.StartAt, input.Reminders); err != nil {
		return CalendarEventRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CalendarEventRecord{}, fmt.Errorf("commit create calendar event tx: %w", err)
	}

	return r.GetEventByIDForUser(ctx, input.UserID, event.ID)
}

func (r *CalendarRepository) UpdateEvent(ctx context.Context, userID, eventID uuid.UUID, input CalendarEventInput) (CalendarEventRecord, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CalendarEventRecord{}, fmt.Errorf("begin update calendar event tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const updateQuery = `
UPDATE calendar_events
SET title = $3,
    description = $4,
    location = $5,
    start_at = $6,
    end_at = $7,
    updated_at = now()
WHERE user_id = $1
  AND id = $2
  AND deleted_at IS NULL
RETURNING id, user_id, title, description, location, start_at, end_at, created_at, updated_at
`
	var (
		event       CalendarEventRecord
		locationRaw sql.NullString
	)
	if err := tx.QueryRow(ctx, updateQuery,
		userID,
		eventID,
		strings.TrimSpace(input.Title),
		strings.TrimSpace(input.Description),
		normalizeOptionalString(input.Location),
		input.StartAt,
		input.EndAt,
	).Scan(&event.ID, &event.UserID, &event.Title, &event.Description, &locationRaw, &event.StartAt, &event.EndAt, &event.CreatedAt, &event.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CalendarEventRecord{}, pgx.ErrNoRows
		}
		return CalendarEventRecord{}, fmt.Errorf("update calendar event: %w", err)
	}
	if locationRaw.Valid {
		value := strings.TrimSpace(locationRaw.String)
		event.Location = &value
	}

	if err := upsertEventReminders(ctx, tx, event.ID, userID, event.StartAt, input.Reminders); err != nil {
		return CalendarEventRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CalendarEventRecord{}, fmt.Errorf("commit update calendar event tx: %w", err)
	}

	return r.GetEventByIDForUser(ctx, userID, event.ID)
}

func (r *CalendarRepository) DeleteEvent(ctx context.Context, userID, eventID uuid.UUID) (bool, error) {
	const query = `
UPDATE calendar_events
SET deleted_at = now(),
    updated_at = now()
WHERE user_id = $1
  AND id = $2
  AND deleted_at IS NULL
`
	tag, err := r.pool.Exec(ctx, query, userID, eventID)
	if err != nil {
		return false, fmt.Errorf("soft delete calendar event: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *CalendarRepository) ListNotificationsByUser(ctx context.Context, userID uuid.UUID, limit int) ([]CalendarNotificationRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	const query = `
SELECT id, user_id, type, title, message, payload, is_read, read_at, created_at
FROM notifications
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2
`
	rows, err := r.pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query notifications: %w", err)
	}
	defer rows.Close()

	out := make([]CalendarNotificationRecord, 0, limit)
	for rows.Next() {
		item, err := scanNotification(rows)
		if err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notifications: %w", err)
	}
	return out, nil
}

func (r *CalendarRepository) MarkNotificationRead(ctx context.Context, userID, notificationID uuid.UUID) (CalendarNotificationRecord, error) {
	const query = `
UPDATE notifications
SET is_read = TRUE,
    read_at = COALESCE(read_at, now())
WHERE user_id = $1
  AND id = $2
RETURNING id, user_id, type, title, message, payload, is_read, read_at, created_at
`
	row := r.pool.QueryRow(ctx, query, userID, notificationID)
	item, err := scanNotification(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CalendarNotificationRecord{}, pgx.ErrNoRows
		}
		return CalendarNotificationRecord{}, fmt.Errorf("mark notification read: %w", err)
	}
	return item, nil
}

func (r *CalendarRepository) ProcessDueReminders(ctx context.Context, now time.Time, batchSize int) (int, error) {
	if batchSize <= 0 {
		batchSize = 100
	}

	const query = `
WITH due AS (
    SELECT rem.id, rem.user_id, rem.event_id, rem.reminder_type, rem.remind_at,
           ev.title,
           ev.start_at
    FROM calendar_event_reminders rem
    JOIN calendar_events ev ON ev.id = rem.event_id
    WHERE rem.delivered_at IS NULL
      AND rem.remind_at <= $1
      AND ev.deleted_at IS NULL
    ORDER BY rem.remind_at ASC
    LIMIT $2
    FOR UPDATE SKIP LOCKED
), inserted AS (
    INSERT INTO notifications (user_id, type, title, message, payload, source_reminder_id, created_at)
    SELECT
        due.user_id,
        'calendar.reminder',
        due.title,
        CASE due.reminder_type
            WHEN 'at_time' THEN 'Event time reached.'
            WHEN '10m' THEN '10 minutes left for the event.'
            WHEN '1h' THEN '1 hour left for the event.'
            WHEN '1d' THEN '1 day left for the event.'
            WHEN '3d' THEN '3 days left for the event.'
            ELSE 'Event reminder.'
        END,
        jsonb_build_object(
            'event_id', due.event_id,
            'reminder_type', due.reminder_type,
            'remind_at', due.remind_at,
            'start_at', due.start_at
        ),
        due.id,
        now()
    FROM due
    ON CONFLICT (source_reminder_id) DO NOTHING
    RETURNING source_reminder_id
), finalized AS (
    UPDATE calendar_event_reminders rem
    SET delivered_at = $1
    WHERE rem.id IN (SELECT id FROM due)
      AND (
            rem.id IN (SELECT source_reminder_id FROM inserted)
            OR EXISTS (SELECT 1 FROM notifications n WHERE n.source_reminder_id = rem.id)
      )
    RETURNING rem.id
)
SELECT COUNT(*)::int FROM finalized
`

	var processed int
	if err := r.pool.QueryRow(ctx, query, now, batchSize).Scan(&processed); err != nil {
		return 0, fmt.Errorf("process due reminders: %w", err)
	}
	return processed, nil
}

func upsertEventReminders(ctx context.Context, tx pgx.Tx, eventID, userID uuid.UUID, startAt time.Time, reminders []string) error {
	normalized := normalizeReminderTypes(reminders)

	const deleteQuery = `DELETE FROM calendar_event_reminders WHERE event_id = $1`
	if _, err := tx.Exec(ctx, deleteQuery, eventID); err != nil {
		return fmt.Errorf("delete calendar reminders: %w", err)
	}

	const insertQuery = `
INSERT INTO calendar_event_reminders (event_id, user_id, reminder_type, remind_at, created_at)
VALUES ($1, $2, $3, $4, now())
`
	for _, reminderType := range normalized {
		remindAt := startAt.Add(-reminderDuration(reminderType))
		if _, err := tx.Exec(ctx, insertQuery, eventID, userID, reminderType, remindAt); err != nil {
			return fmt.Errorf("insert calendar reminder %s: %w", reminderType, err)
		}
	}
	return nil
}

func normalizeReminderTypes(input []string) []string {
	allowed := map[string]struct{}{
		"at_time": {},
		"10m":     {},
		"1h":      {},
		"1d":      {},
		"3d":      {},
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(input))
	for _, raw := range input {
		key := strings.TrimSpace(strings.ToLower(raw))
		if key == "" {
			continue
		}
		if _, ok := allowed[key]; !ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func reminderDuration(reminderType string) time.Duration {
	switch reminderType {
	case "10m":
		return 10 * time.Minute
	case "1h":
		return time.Hour
	case "1d":
		return 24 * time.Hour
	case "3d":
		return 72 * time.Hour
	default:
		return 0
	}
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

type calendarEventScanner interface {
	Scan(dest ...any) error
}

func scanCalendarEvent(row calendarEventScanner) (CalendarEventRecord, error) {
	var (
		event       CalendarEventRecord
		locationRaw sql.NullString
		reminders   []string
	)
	if err := row.Scan(
		&event.ID,
		&event.UserID,
		&event.Title,
		&event.Description,
		&locationRaw,
		&event.StartAt,
		&event.EndAt,
		&reminders,
		&event.CreatedAt,
		&event.UpdatedAt,
	); err != nil {
		return CalendarEventRecord{}, err
	}
	if locationRaw.Valid {
		value := strings.TrimSpace(locationRaw.String)
		event.Location = &value
	}
	event.Reminders = reminders
	return event, nil
}

type notificationScanner interface {
	Scan(dest ...any) error
}

func scanNotification(row notificationScanner) (CalendarNotificationRecord, error) {
	var (
		item       CalendarNotificationRecord
		payloadRaw []byte
		readAtRaw  sql.NullTime
	)
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.Type,
		&item.Title,
		&item.Message,
		&payloadRaw,
		&item.IsRead,
		&readAtRaw,
		&item.CreatedAt,
	); err != nil {
		return CalendarNotificationRecord{}, err
	}
	if len(payloadRaw) > 0 {
		if err := json.Unmarshal(payloadRaw, &item.Payload); err != nil {
			return CalendarNotificationRecord{}, fmt.Errorf("unmarshal notification payload: %w", err)
		}
	}
	if item.Payload == nil {
		item.Payload = map[string]any{}
	}
	if readAtRaw.Valid {
		value := readAtRaw.Time
		item.ReadAt = &value
	}
	return item, nil
}
