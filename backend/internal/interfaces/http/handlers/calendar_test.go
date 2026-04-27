package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud/backend/internal/domain/identity"
	"cloud/backend/internal/interfaces/http/authctx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type fakeCalendarRepo struct {
	lastDeleteUserID uuid.UUID
	lastDeleteEvent  uuid.UUID
	lastReadUserID   uuid.UUID
	lastReadNotifID  uuid.UUID
	deleteResult     bool
	listErr          error
	notificationsErr error
}

func (f *fakeCalendarRepo) ListEventsByUser(context.Context, uuid.UUID, *time.Time, *time.Time) ([]CalendarEventDTO, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return nil, nil
}

func (f *fakeCalendarRepo) CreateEvent(context.Context, CalendarEventInputDTO) (CalendarEventDTO, error) {
	return CalendarEventDTO{}, nil
}

func (f *fakeCalendarRepo) UpdateEvent(context.Context, uuid.UUID, uuid.UUID, CalendarEventInputDTO) (CalendarEventDTO, error) {
	return CalendarEventDTO{}, nil
}

func (f *fakeCalendarRepo) DeleteEvent(_ context.Context, userID, eventID uuid.UUID) (bool, error) {
	f.lastDeleteUserID = userID
	f.lastDeleteEvent = eventID
	return f.deleteResult, nil
}

func (f *fakeCalendarRepo) ListNotificationsByUser(context.Context, uuid.UUID, int) ([]CalendarNotificationDTO, error) {
	if f.notificationsErr != nil {
		return nil, f.notificationsErr
	}
	return nil, nil
}

func (f *fakeCalendarRepo) MarkNotificationRead(_ context.Context, userID, notificationID uuid.UUID) (CalendarNotificationDTO, error) {
	f.lastReadUserID = userID
	f.lastReadNotifID = notificationID
	return CalendarNotificationDTO{}, nil
}

func TestCalendarDeleteUsesAuthenticatedOwner(t *testing.T) {
	repo := &fakeCalendarRepo{deleteResult: true}
	h := NewCalendarHandler(repo)

	userID := uuid.New()
	eventID := uuid.New()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/calendar/events/"+eventID.String(), nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("eventID", eventID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	req = req.WithContext(authctx.WithContext(req.Context(), authctx.Context{
		UserID: userID,
		Role:   identity.RoleUser,
	}))
	rec := httptest.NewRecorder()

	h.DeleteEvent(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if repo.lastDeleteUserID != userID {
		t.Fatalf("expected delete user id %s, got %s", userID, repo.lastDeleteUserID)
	}
	if repo.lastDeleteEvent != eventID {
		t.Fatalf("expected delete event id %s, got %s", eventID, repo.lastDeleteEvent)
	}
}

func TestCalendarDeleteRequiresAuth(t *testing.T) {
	repo := &fakeCalendarRepo{deleteResult: true}
	h := NewCalendarHandler(repo)
	eventID := uuid.New()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/calendar/events/"+eventID.String(), nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("eventID", eventID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rec := httptest.NewRecorder()

	h.DeleteEvent(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestCalendarNotificationsListRequiresAuth(t *testing.T) {
	repo := &fakeCalendarRepo{}
	h := NewCalendarHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications?limit=50", nil)
	rec := httptest.NewRecorder()

	h.ListNotifications(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestCalendarMarkNotificationReadUsesAuthenticatedOwner(t *testing.T) {
	repo := &fakeCalendarRepo{}
	h := NewCalendarHandler(repo)

	userID := uuid.New()
	notificationID := uuid.New()

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/notifications/"+notificationID.String()+"/read", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("notificationID", notificationID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	req = req.WithContext(authctx.WithContext(req.Context(), authctx.Context{
		UserID: userID,
		Role:   identity.RoleUser,
	}))
	rec := httptest.NewRecorder()

	h.MarkNotificationRead(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if repo.lastReadUserID != userID {
		t.Fatalf("expected read user id %s, got %s", userID, repo.lastReadUserID)
	}
	if repo.lastReadNotifID != notificationID {
		t.Fatalf("expected notification id %s, got %s", notificationID, repo.lastReadNotifID)
	}
}

func TestCalendarListEventsSchemaNotReady(t *testing.T) {
	repo := &fakeCalendarRepo{
		listErr: errors.New(`relation "calendar_events" does not exist`),
	}
	h := NewCalendarHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/calendar/events", nil)
	req = req.WithContext(authctx.WithContext(req.Context(), authctx.Context{
		UserID: uuid.New(),
		Role:   identity.RoleUser,
	}))
	rec := httptest.NewRecorder()

	h.ListEvents(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestCalendarNotificationsSchemaNotReady(t *testing.T) {
	repo := &fakeCalendarRepo{
		notificationsErr: errors.New(`relation "notifications" does not exist`),
	}
	h := NewCalendarHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications?limit=50", nil)
	req = req.WithContext(authctx.WithContext(req.Context(), authctx.Context{
		UserID: uuid.New(),
		Role:   identity.RoleUser,
	}))
	rec := httptest.NewRecorder()

	h.ListNotifications(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}
