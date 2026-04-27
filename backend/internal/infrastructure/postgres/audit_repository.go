package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud/backend/internal/application/auth"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

type AuditListParams struct {
	Limit      int
	Page       int
	EventTypes []string
	Search     string
}

type AuditEventRecord struct {
	Event     auth.SecurityEvent
	UserEmail *string
}

type AuditListResult struct {
	Events []AuditEventRecord
	Total  int
	Page   int
	Limit  int
}

func (r *AuditRepository) RecordSecurityEvent(ctx context.Context, event auth.SecurityEvent) error {
	const query = `
INSERT INTO audit_logs (event_type, severity, user_id, session_id, ip, user_agent, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`

	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataRaw, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}

	if _, err := r.pool.Exec(ctx, query,
		event.EventType,
		event.Severity,
		event.UserID,
		event.SessionID,
		event.IP,
		event.UserAgent,
		metadataRaw,
	); err != nil {
		return fmt.Errorf("insert security audit event: %w", err)
	}
	return nil
}

func (r *AuditRepository) ListSecurityEvents(ctx context.Context, params AuditListParams) (AuditListResult, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	page := params.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit
	search := params.Search

	const countQuery = `
SELECT COUNT(*)
FROM audit_logs al
LEFT JOIN users u ON u.id = al.user_id
WHERE ($1::text[] IS NULL OR cardinality($1) = 0 OR al.event_type = ANY($1))
  AND (
    $2 = ''
    OR al.user_id::text ILIKE ('%' || $2 || '%')
    OR COALESCE(u.email, '') ILIKE ('%' || $2 || '%')
  )
`
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, params.EventTypes, search).Scan(&total); err != nil {
		return AuditListResult{}, fmt.Errorf("count audit logs: %w", err)
	}

	const query = `
SELECT al.id, al.event_type, al.severity, al.user_id, al.session_id, al.ip, al.user_agent, al.metadata, al.created_at, u.email
FROM audit_logs al
LEFT JOIN users u ON u.id = al.user_id
WHERE ($1::text[] IS NULL OR cardinality($1) = 0 OR al.event_type = ANY($1))
  AND (
    $2 = ''
    OR al.user_id::text ILIKE ('%' || $2 || '%')
    OR COALESCE(u.email, '') ILIKE ('%' || $2 || '%')
  )
ORDER BY al.created_at DESC, al.id DESC
LIMIT $3 OFFSET $4
`

	rows, err := r.pool.Query(ctx, query, params.EventTypes, search, limit, offset)
	if err != nil {
		return AuditListResult{}, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	entries := make([]AuditEventRecord, 0, limit)
	for rows.Next() {
		var (
			discardID    int64
			entry        auth.SecurityEvent
			metadataRaw  []byte
			userEmailRaw *string
		)
		if err := rows.Scan(
			&discardID,
			&entry.EventType,
			&entry.Severity,
			&entry.UserID,
			&entry.SessionID,
			&entry.IP,
			&entry.UserAgent,
			&metadataRaw,
			&entry.CreatedAt,
			&userEmailRaw,
		); err != nil {
			return AuditListResult{}, fmt.Errorf("scan audit log row: %w", err)
		}
		if len(metadataRaw) > 0 {
			if err := json.Unmarshal(metadataRaw, &entry.Metadata); err != nil {
				return AuditListResult{}, fmt.Errorf("unmarshal audit metadata: %w", err)
			}
		}
		if entry.Metadata == nil {
			entry.Metadata = map[string]any{}
		}
		entries = append(entries, AuditEventRecord{
			Event:     entry,
			UserEmail: userEmailRaw,
		})
	}
	if err := rows.Err(); err != nil {
		return AuditListResult{}, fmt.Errorf("iterate audit log rows: %w", err)
	}

	return AuditListResult{
		Events: entries,
		Total:  total,
		Page:   page,
		Limit:  limit,
	}, nil
}

func (r *AuditRepository) ListRecentByEventTypes(ctx context.Context, limit int, eventTypes []string) ([]auth.SecurityEvent, error) {
	result, err := r.ListSecurityEvents(ctx, AuditListParams{
		Limit:      limit,
		Page:       1,
		EventTypes: eventTypes,
	})
	if err != nil {
		return nil, err
	}
	events := make([]auth.SecurityEvent, 0, len(result.Events))
	for _, entry := range result.Events {
		events = append(events, entry.Event)
	}
	return events, nil
}
