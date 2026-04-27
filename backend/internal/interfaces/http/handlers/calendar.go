package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloud/backend/internal/interfaces/http/authctx"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CalendarRepository interface {
	ListEventsByUser(ctx context.Context, userID uuid.UUID, from *time.Time, to *time.Time) ([]CalendarEventDTO, error)
	CreateEvent(ctx context.Context, input CalendarEventInputDTO) (CalendarEventDTO, error)
	UpdateEvent(ctx context.Context, userID, eventID uuid.UUID, input CalendarEventInputDTO) (CalendarEventDTO, error)
	DeleteEvent(ctx context.Context, userID, eventID uuid.UUID) (bool, error)
	ListNotificationsByUser(ctx context.Context, userID uuid.UUID, limit int) ([]CalendarNotificationDTO, error)
	MarkNotificationRead(ctx context.Context, userID, notificationID uuid.UUID) (CalendarNotificationDTO, error)
}

type CalendarEventDTO struct {
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

type CalendarEventInputDTO struct {
	UserID      uuid.UUID
	Title       string
	Description string
	Location    *string
	StartAt     time.Time
	EndAt       time.Time
	Reminders   []string
}

type CalendarNotificationDTO struct {
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

type CalendarHandler struct {
	repo CalendarRepository
}

func NewCalendarHandler(repo CalendarRepository) *CalendarHandler {
	return &CalendarHandler{repo: repo}
}

type calendarEventRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Location    *string  `json:"location"`
	StartAt     string   `json:"start_datetime"`
	EndAt       string   `json:"end_datetime"`
	Reminders   []string `json:"reminders"`
}

func (h *CalendarHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var from *time.Time
	var to *time.Time
	if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
		parsed, err := parseRFC3339(raw)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid from datetime")
			return
		}
		from = &parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
		parsed, err := parseRFC3339(raw)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid to datetime")
			return
		}
		to = &parsed
	}

	events, err := h.repo.ListEventsByUser(r.Context(), authValue.UserID, from, to)
	if err != nil {
		writeInternalOrSchemaError(w, err, "failed to list calendar events")
		return
	}

	out := make([]map[string]any, 0, len(events))
	for _, event := range events {
		out = append(out, calendarEventToResponse(event))
	}

	writeJSON(w, http.StatusOK, map[string]any{"events": out})
}

func (h *CalendarHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req calendarEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	input, err := buildCalendarInput(authValue.UserID, req)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	event, err := h.repo.CreateEvent(r.Context(), input)
	if err != nil {
		writeInternalOrSchemaError(w, err, "failed to create calendar event")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"event": calendarEventToResponse(event)})
}

func (h *CalendarHandler) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	eventID, err := parseURLParamUUID(r, "eventID")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid event id")
		return
	}

	var req calendarEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	input, err := buildCalendarInput(authValue.UserID, req)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	event, err := h.repo.UpdateEvent(r.Context(), authValue.UserID, eventID, input)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, "event_not_found", "event not found")
			return
		}
		writeInternalOrSchemaError(w, err, "failed to update calendar event")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"event": calendarEventToResponse(event)})
}

func (h *CalendarHandler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	eventID, err := parseURLParamUUID(r, "eventID")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid event id")
		return
	}

	deleted, err := h.repo.DeleteEvent(r.Context(), authValue.UserID, eventID)
	if err != nil {
		writeInternalOrSchemaError(w, err, "failed to delete calendar event")
		return
	}
	if !deleted {
		writeAPIError(w, http.StatusNotFound, "event_not_found", "event not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CalendarHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid limit")
			return
		}
		limit = parsed
	}

	items, err := h.repo.ListNotificationsByUser(r.Context(), authValue.UserID, limit)
	if err != nil {
		writeInternalOrSchemaError(w, err, "failed to list notifications")
		return
	}

	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, calendarNotificationToResponse(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": out})
}

func (h *CalendarHandler) MarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	notificationID, err := parseURLParamUUID(r, "notificationID")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid notification id")
		return
	}

	item, err := h.repo.MarkNotificationRead(r.Context(), authValue.UserID, notificationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, "notification_not_found", "notification not found")
			return
		}
		writeInternalOrSchemaError(w, err, "failed to update notification")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"notification": calendarNotificationToResponse(item)})
}

func buildCalendarInput(userID uuid.UUID, req calendarEventRequest) (CalendarEventInputDTO, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return CalendarEventInputDTO{}, fmt.Errorf("title is required")
	}
	startAt, err := parseRFC3339(req.StartAt)
	if err != nil {
		return CalendarEventInputDTO{}, fmt.Errorf("invalid start_datetime")
	}
	endAt, err := parseRFC3339(req.EndAt)
	if err != nil {
		return CalendarEventInputDTO{}, fmt.Errorf("invalid end_datetime")
	}
	if !endAt.After(startAt) {
		return CalendarEventInputDTO{}, fmt.Errorf("end_datetime must be after start_datetime")
	}

	return CalendarEventInputDTO{
		UserID:      userID,
		Title:       title,
		Description: strings.TrimSpace(req.Description),
		Location:    req.Location,
		StartAt:     startAt,
		EndAt:       endAt,
		Reminders:   req.Reminders,
	}, nil
}

func parseRFC3339(raw string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func calendarEventToResponse(event CalendarEventDTO) map[string]any {
	response := map[string]any{
		"id":             event.ID.String(),
		"user_id":        event.UserID.String(),
		"title":          event.Title,
		"description":    event.Description,
		"start_datetime": event.StartAt,
		"end_datetime":   event.EndAt,
		"reminders":      event.Reminders,
		"created_at":     event.CreatedAt,
		"updated_at":     event.UpdatedAt,
	}
	if event.Location != nil {
		response["location"] = *event.Location
	} else {
		response["location"] = nil
	}
	return response
}

func calendarNotificationToResponse(item CalendarNotificationDTO) map[string]any {
	response := map[string]any{
		"id":         item.ID.String(),
		"user_id":    item.UserID.String(),
		"type":       item.Type,
		"title":      item.Title,
		"message":    item.Message,
		"payload":    item.Payload,
		"is_read":    item.IsRead,
		"created_at": item.CreatedAt,
	}
	if item.ReadAt != nil {
		response["read_at"] = *item.ReadAt
	} else {
		response["read_at"] = nil
	}
	return response
}
