package http

import (
	"context"
	"fmt"
	"log/slog"
	nethttp "net/http"
	"strings"
	"time"

	appauth "cloud/backend/internal/application/auth"
	appfiles "cloud/backend/internal/application/files"
	appquota "cloud/backend/internal/application/quota"
	"cloud/backend/internal/infrastructure/config"
	infraPostgres "cloud/backend/internal/infrastructure/postgres"
	"cloud/backend/internal/infrastructure/security/password"
	"cloud/backend/internal/infrastructure/security/token"
	"cloud/backend/internal/infrastructure/storage/localfs"
	"cloud/backend/internal/interfaces/http/clientip"
	"cloud/backend/internal/interfaces/http/handlers"
	appmw "cloud/backend/internal/interfaces/http/middleware"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	redisv9 "github.com/redis/go-redis/v9"
)

const apiVersion = "0.2.1"

func NewRouter(
	log *slog.Logger,
	cfg config.Config,
	pgPool *pgxpool.Pool,
	redisClient *redisv9.Client,
) (nethttp.Handler, error) {
	ipResolver, err := clientip.NewResolver(cfg.TrustedProxyCIDRs)
	if err != nil {
		return nil, fmt.Errorf("create client ip resolver: %w", err)
	}

	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.Use(appmw.WithClientIP(ipResolver))
	router.Use(chimw.Recoverer)
	router.Use(appmw.SecurityHeaders)
	router.Use(appmw.CORS(cfg.CORSOrigins))
	router.Use(appmw.RequestLogger(log))

	usersRepo := infraPostgres.NewUserRepository(pgPool)
	sessionsRepo := infraPostgres.NewSessionRepository(pgPool)
	quotaRepo := infraPostgres.NewQuotaRepository(pgPool)
	auditRepo := infraPostgres.NewAuditRepository(pgPool)
	filesRepo := infraPostgres.NewFilesRepository(pgPool)
	settingsRepo := infraPostgres.NewSettingsRepository(pgPool)
	calendarRepo := infraPostgres.NewCalendarRepository(pgPool)
	shareRepo := infraPostgres.NewShareRepository(pgPool)
	passwordHasher := password.NewArgon2IDHasher()
	tokenManager := token.NewJWTManager(cfg.JWTSigningKey, cfg.JWTIssuer, cfg.JWTAudience, cfg.AccessTokenTTL)
	authService := appauth.NewService(usersRepo, sessionsRepo, tokenManager, passwordHasher, appauth.RealClock{}, cfg.RefreshTokenTTL, log, auditRepo)
	quotaService := appquota.NewService(quotaRepo, filesRepo)
	uploadStorage := localfs.New(cfg.StorageLocalRoot)
	filesService := appfiles.NewService(filesRepo, uploadStorage, appfiles.RealClock{}, appfiles.Config{
		UploadSessionTTL:        cfg.UploadSessionTTL,
		MaxChunkBytes:           cfg.UploadMaxChunkBytes,
		MaxFileBytes:            cfg.UploadMaxFileBytes,
		MaxActiveUploadSessions: cfg.UploadMaxActiveSessions,
		MaxUserStagedBytes:      cfg.UploadMaxStagedBytes,
	}, log)

	healthHandler := handlers.NewHealthHandler([]handlers.ReadinessCheck{
		{
			Name: "postgres",
			Check: func(ctx context.Context) error {
				return pgPool.Ping(ctx)
			},
		},
		{
			Name: "redis",
			Check: func(ctx context.Context) error {
				if redisClient == nil {
					return fmt.Errorf("redis client is nil")
				}
				return redisClient.Ping(ctx).Err()
			},
		},
	})
	metaHandler := handlers.NewMetaHandler(apiVersion)
	publicSettingsHandler := handlers.NewPublicSettingsHandler(adminSettingsAdapter{repo: settingsRepo})
	authHandler := handlers.NewAuthHandler(log, authService, handlers.AuthCookieConfig{
		Domain:         cfg.CookieDomain,
		Secure:         cfg.CookieSecure,
		SameSite:       parseSameSite(cfg.CookieSameSite),
		AccessTTL:      cfg.AccessTokenTTL,
		RefreshTTL:     cfg.RefreshTokenTTL,
		AllowedOrigins: cfg.CORSOrigins,
	}, ipResolver)
	meHandler := handlers.NewMeHandler(usersRepo, passwordHasher, adminSettingsAdapter{repo: settingsRepo})
	quotaHandler := handlers.NewQuotaHandler(quotaService)
	filesHandler := handlers.NewFilesHandler(log, filesService, auditRepo)
	calendarHandler := handlers.NewCalendarHandler(calendarAdapter{repo: calendarRepo})
	sharesHandler := handlers.NewSharesHandler(
		shareAdapter{repo: shareRepo},
		passwordHasher,
		adminSettingsAdapter{repo: settingsRepo},
		filesDownloadAdapter{service: filesService},
	)
	adminSettingsService := handlers.NewAdminSettingsService(adminSettingsAdapter{repo: settingsRepo}, quotaRepo, cfg.StorageLocalRoot)
	adminUsersHandler := handlers.NewAdminUsersHandler(
		log,
		usersRepo,
		passwordHasher,
		quotaRepo,
		filesRepo,
		filesRepo,
		adminAuditReaderAdapter{repo: auditRepo},
		adminSettingsAdapter{repo: settingsRepo},
		cfg.StorageLocalRoot,
	)
	adminSettingsHandler := handlers.NewAdminSettingsHandler(adminSettingsService)
	authLimiter := appmw.NewRateLimiter(cfg.AuthRateLimit, cfg.AuthRateWindow, ipResolver)
	requireAuth := appmw.RequireAuth(authService)

	router.Get("/health/live", healthHandler.Live)
	router.Get("/health/ready", healthHandler.Ready)
	router.Get("/s/{token}", sharesHandler.PublicLanding)
	router.Get("/api/v1/public/settings", publicSettingsHandler.Get)

	router.Route("/api/public", func(pr chi.Router) {
		pr.Get("/shares/{token}", sharesHandler.PublicGetShare)
		pr.Post("/shares/{token}/unlock", sharesHandler.PublicUnlockShare)
		pr.Get("/shares/{token}/download", sharesHandler.PublicDownloadShare)
	})

	router.Route("/api/v1", func(r chi.Router) {
		r.Get("/version", metaHandler.Version)

		r.Route("/auth", func(ar chi.Router) {
			ar.Use(authLimiter.Middleware)
			ar.Post("/login", authHandler.Login)
			ar.Post("/refresh", authHandler.Refresh)
			ar.With(requireAuth).Post("/logout", authHandler.Logout)
		})

		r.With(requireAuth).Get("/me", meHandler.GetMe)
		r.With(requireAuth).Patch("/me", meHandler.UpdateMe)
		r.With(requireAuth).Patch("/me/password", meHandler.UpdateMyPassword)
		r.With(requireAuth).Get("/quota", quotaHandler.GetMyQuota)
		r.With(requireAuth).Get("/notifications", calendarHandler.ListNotifications)
		r.With(requireAuth).Patch("/notifications/{notificationID}/read", calendarHandler.MarkNotificationRead)

		r.Route("/files", func(fr chi.Router) {
			fr.With(requireAuth).Post("/folders", filesHandler.CreateFolder)
			fr.With(requireAuth).Get("/nodes", filesHandler.ListNodes)
			fr.With(requireAuth).Get("/search", filesHandler.SearchNodes)
			fr.With(requireAuth).Get("/trash", filesHandler.ListTrash)
			fr.With(requireAuth).Post("/trash/{nodeID}/restore", filesHandler.RestoreTrashNode)
			fr.With(requireAuth).Delete("/trash/{nodeID}", filesHandler.PermanentlyDeleteTrashNode)
			fr.With(requireAuth).Delete("/trash", filesHandler.EmptyTrash)
			fr.With(requireAuth).Get("/nodes/{nodeID}", filesHandler.GetNode)
			fr.With(requireAuth).Patch("/nodes/{nodeID}/rename", filesHandler.RenameNode)
			fr.With(requireAuth).Patch("/nodes/{nodeID}/move", filesHandler.MoveNode)
			fr.With(requireAuth).Delete("/nodes/{nodeID}", filesHandler.DeleteNode)
			fr.With(requireAuth).Get("/nodes/{nodeID}/download", filesHandler.DownloadNode)
			fr.With(requireAuth).Get("/download", filesHandler.DownloadByQuery)

			fr.With(requireAuth).Post("/uploads", filesHandler.CreateUploadSession)
			fr.With(requireAuth).Get("/uploads/{uploadSessionID}", filesHandler.GetUploadSession)
			fr.With(requireAuth).Put("/uploads/{uploadSessionID}/chunks/{chunkIndex}", filesHandler.UploadChunk)
			fr.With(requireAuth).Post("/uploads/{uploadSessionID}/finalize", filesHandler.FinalizeUploadSession)
			fr.With(requireAuth).Post("/uploads/{uploadSessionID}/abort", filesHandler.AbortUploadSession)
		})

		r.Route("/calendar", func(cr chi.Router) {
			cr.Use(requireAuth)
			cr.Get("/events", calendarHandler.ListEvents)
			cr.Post("/events", calendarHandler.CreateEvent)
			cr.Patch("/events/{eventID}", calendarHandler.UpdateEvent)
			cr.Delete("/events/{eventID}", calendarHandler.DeleteEvent)
		})

		r.Route("/shares", func(sr chi.Router) {
			sr.Use(requireAuth)
			sr.Get("/", sharesHandler.ListShares)
			sr.Post("/", sharesHandler.CreateShare)
			sr.Patch("/{shareID}", sharesHandler.UpdateShare)
			sr.Delete("/{shareID}", sharesHandler.DeleteShare)
		})

		r.Route("/admin", func(ar chi.Router) {
			ar.Use(requireAuth)
			ar.Get("/users", adminUsersHandler.ListUsers)
			ar.Post("/users", adminUsersHandler.CreateUser)
			ar.Patch("/users/{userID}", adminUsersHandler.UpdateUser)
			ar.Delete("/users/{userID}", adminUsersHandler.DeleteUser)
			ar.Post("/users/{userID}/password", adminUsersHandler.UpdateUserPassword)
			ar.Get("/audit", adminUsersHandler.ListAudit)
			ar.Get("/settings", adminSettingsHandler.GetSettings)
			ar.Patch("/settings/{section}", adminSettingsHandler.UpdateSection)
		})
	})

	return router, nil
}

func parseSameSite(raw string) nethttp.SameSite {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "strict":
		return nethttp.SameSiteStrictMode
	case "none":
		return nethttp.SameSiteNoneMode
	default:
		return nethttp.SameSiteLaxMode
	}
}

type adminAuditReaderAdapter struct {
	repo *infraPostgres.AuditRepository
}

func (a adminAuditReaderAdapter) ListSecurityEvents(
	ctx context.Context,
	query handlers.AdminAuditQuery,
) (handlers.AdminAuditQueryResult, error) {
	result, err := a.repo.ListSecurityEvents(ctx, infraPostgres.AuditListParams{
		Limit:      query.Limit,
		Page:       query.Page,
		EventTypes: query.EventTypes,
		Search:     query.Search,
	})
	if err != nil {
		return handlers.AdminAuditQueryResult{}, err
	}

	events := make([]handlers.AdminAuditEntry, 0, len(result.Events))
	for _, entry := range result.Events {
		events = append(events, handlers.AdminAuditEntry{
			Event:     entry.Event,
			UserEmail: entry.UserEmail,
		})
	}

	return handlers.AdminAuditQueryResult{
		Events: events,
		Total:  result.Total,
		Page:   result.Page,
		Limit:  result.Limit,
	}, nil
}

func (a adminAuditReaderAdapter) RecordSecurityEvent(ctx context.Context, event appauth.SecurityEvent) error {
	return a.repo.RecordSecurityEvent(ctx, event)
}

type adminSettingsAdapter struct {
	repo *infraPostgres.SettingsRepository
}

func (a adminSettingsAdapter) List(ctx context.Context) ([]handlers.AdminSettingRecord, error) {
	items, err := a.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]handlers.AdminSettingRecord, 0, len(items))
	for _, item := range items {
		out = append(out, handlers.AdminSettingRecord{
			Key:       item.Key,
			ValueJSON: item.ValueJSON,
		})
	}
	return out, nil
}

func (a adminSettingsAdapter) Get(ctx context.Context, key string) (handlers.AdminSettingRecord, error) {
	item, err := a.repo.Get(ctx, key)
	if err != nil {
		return handlers.AdminSettingRecord{}, err
	}
	return handlers.AdminSettingRecord{
		Key:       item.Key,
		ValueJSON: item.ValueJSON,
	}, nil
}

func (a adminSettingsAdapter) Upsert(ctx context.Context, key string, value any, updatedBy *uuid.UUID) (handlers.AdminSettingRecord, error) {
	item, err := a.repo.Upsert(ctx, key, value, updatedBy)
	if err != nil {
		return handlers.AdminSettingRecord{}, err
	}
	return handlers.AdminSettingRecord{
		Key:       item.Key,
		ValueJSON: item.ValueJSON,
	}, nil
}

type calendarAdapter struct {
	repo *infraPostgres.CalendarRepository
}

func (a calendarAdapter) ListEventsByUser(ctx context.Context, userID uuid.UUID, from *time.Time, to *time.Time) ([]handlers.CalendarEventDTO, error) {
	items, err := a.repo.ListEventsByUser(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]handlers.CalendarEventDTO, 0, len(items))
	for _, item := range items {
		out = append(out, handlers.CalendarEventDTO{
			ID:          item.ID,
			UserID:      item.UserID,
			Title:       item.Title,
			Description: item.Description,
			Location:    item.Location,
			StartAt:     item.StartAt,
			EndAt:       item.EndAt,
			Reminders:   item.Reminders,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}
	return out, nil
}

func (a calendarAdapter) CreateEvent(ctx context.Context, input handlers.CalendarEventInputDTO) (handlers.CalendarEventDTO, error) {
	item, err := a.repo.CreateEvent(ctx, infraPostgres.CalendarEventInput{
		UserID:      input.UserID,
		Title:       input.Title,
		Description: input.Description,
		Location:    input.Location,
		StartAt:     input.StartAt,
		EndAt:       input.EndAt,
		Reminders:   input.Reminders,
	})
	if err != nil {
		return handlers.CalendarEventDTO{}, err
	}
	return handlers.CalendarEventDTO{
		ID:          item.ID,
		UserID:      item.UserID,
		Title:       item.Title,
		Description: item.Description,
		Location:    item.Location,
		StartAt:     item.StartAt,
		EndAt:       item.EndAt,
		Reminders:   item.Reminders,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}, nil
}

func (a calendarAdapter) UpdateEvent(ctx context.Context, userID, eventID uuid.UUID, input handlers.CalendarEventInputDTO) (handlers.CalendarEventDTO, error) {
	item, err := a.repo.UpdateEvent(ctx, userID, eventID, infraPostgres.CalendarEventInput{
		UserID:      input.UserID,
		Title:       input.Title,
		Description: input.Description,
		Location:    input.Location,
		StartAt:     input.StartAt,
		EndAt:       input.EndAt,
		Reminders:   input.Reminders,
	})
	if err != nil {
		return handlers.CalendarEventDTO{}, err
	}
	return handlers.CalendarEventDTO{
		ID:          item.ID,
		UserID:      item.UserID,
		Title:       item.Title,
		Description: item.Description,
		Location:    item.Location,
		StartAt:     item.StartAt,
		EndAt:       item.EndAt,
		Reminders:   item.Reminders,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}, nil
}

func (a calendarAdapter) DeleteEvent(ctx context.Context, userID, eventID uuid.UUID) (bool, error) {
	return a.repo.DeleteEvent(ctx, userID, eventID)
}

func (a calendarAdapter) ListNotificationsByUser(ctx context.Context, userID uuid.UUID, limit int) ([]handlers.CalendarNotificationDTO, error) {
	items, err := a.repo.ListNotificationsByUser(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]handlers.CalendarNotificationDTO, 0, len(items))
	for _, item := range items {
		out = append(out, handlers.CalendarNotificationDTO{
			ID:        item.ID,
			UserID:    item.UserID,
			Type:      item.Type,
			Title:     item.Title,
			Message:   item.Message,
			Payload:   item.Payload,
			IsRead:    item.IsRead,
			ReadAt:    item.ReadAt,
			CreatedAt: item.CreatedAt,
		})
	}
	return out, nil
}

func (a calendarAdapter) MarkNotificationRead(ctx context.Context, userID, notificationID uuid.UUID) (handlers.CalendarNotificationDTO, error) {
	item, err := a.repo.MarkNotificationRead(ctx, userID, notificationID)
	if err != nil {
		return handlers.CalendarNotificationDTO{}, err
	}
	return handlers.CalendarNotificationDTO{
		ID:        item.ID,
		UserID:    item.UserID,
		Type:      item.Type,
		Title:     item.Title,
		Message:   item.Message,
		Payload:   item.Payload,
		IsRead:    item.IsRead,
		ReadAt:    item.ReadAt,
		CreatedAt: item.CreatedAt,
	}, nil
}

type shareAdapter struct {
	repo *infraPostgres.ShareRepository
}

func (a shareAdapter) Create(ctx context.Context, input handlers.ShareInputDTO) (handlers.ShareDTO, error) {
	item, err := a.repo.Create(ctx, infraPostgres.ShareInput{
		NodeID:        input.NodeID,
		OwnerUserID:   input.OwnerUserID,
		TokenHash:     input.TokenHash,
		PasswordHash:  input.PasswordHash,
		ExpiresAt:     input.ExpiresAt,
		MaxDownloads:  input.MaxDownloads,
		AllowDownload: input.AllowDownload,
		AllowPreview:  input.AllowPreview,
	})
	if err != nil {
		return handlers.ShareDTO{}, err
	}
	return mapShare(item), nil
}

func (a shareAdapter) ListByOwner(ctx context.Context, ownerUserID uuid.UUID, nodeID *uuid.UUID) ([]handlers.ShareDTO, error) {
	items, err := a.repo.ListByOwner(ctx, ownerUserID, nodeID)
	if err != nil {
		return nil, err
	}
	out := make([]handlers.ShareDTO, 0, len(items))
	for _, item := range items {
		out = append(out, mapShare(item))
	}
	return out, nil
}

func (a shareAdapter) GetByIDForOwner(ctx context.Context, ownerUserID, shareID uuid.UUID) (handlers.ShareDTO, error) {
	item, err := a.repo.GetByIDForOwner(ctx, ownerUserID, shareID)
	if err != nil {
		return handlers.ShareDTO{}, err
	}
	return mapShare(item), nil
}

func (a shareAdapter) UpdateForOwner(ctx context.Context, ownerUserID, shareID uuid.UUID, expiresAt *time.Time, maxDownloads *int, allowDownload *bool, passwordHash *string, clearPassword bool) (handlers.ShareDTO, error) {
	item, err := a.repo.UpdateForOwner(ctx, ownerUserID, shareID, expiresAt, maxDownloads, allowDownload, passwordHash, clearPassword)
	if err != nil {
		return handlers.ShareDTO{}, err
	}
	return mapShare(item), nil
}

func (a shareAdapter) RevokeForOwner(ctx context.Context, ownerUserID, shareID uuid.UUID) (bool, error) {
	return a.repo.RevokeForOwner(ctx, ownerUserID, shareID)
}

func (a shareAdapter) GetPublicByTokenHash(ctx context.Context, tokenHash string) (handlers.PublicShareDTO, error) {
	item, err := a.repo.GetPublicByTokenHash(ctx, tokenHash)
	if err != nil {
		return handlers.PublicShareDTO{}, err
	}
	return handlers.PublicShareDTO{
		Share: mapShare(item.Share),
		Node: handlers.ShareNodeSummaryDTO{
			ID:          item.Node.ID,
			OwnerUserID: item.Node.OwnerUserID,
			Type:        item.Node.Type,
			Name:        item.Node.Name,
			SizeBytes:   item.Node.SizeBytes,
			MIMEType:    item.Node.MIMEType,
			UpdatedAt:   item.Node.UpdatedAt,
			DeletedAt:   item.Node.DeletedAt,
		},
	}, nil
}

func (a shareAdapter) IncrementDownloadCount(ctx context.Context, shareID uuid.UUID) (handlers.ShareDTO, error) {
	item, err := a.repo.IncrementDownloadCount(ctx, shareID)
	if err != nil {
		return handlers.ShareDTO{}, err
	}
	return mapShare(item), nil
}

func (a shareAdapter) CreateAccessGrant(ctx context.Context, shareID uuid.UUID, grantHash string, expiresAt time.Time) error {
	return a.repo.CreateAccessGrant(ctx, shareID, grantHash, expiresAt)
}

func (a shareAdapter) ValidateAccessGrant(ctx context.Context, shareID uuid.UUID, grantHash string, now time.Time) (bool, error) {
	return a.repo.ValidateAccessGrant(ctx, shareID, grantHash, now)
}

func (a shareAdapter) GetNodeSummaryForOwner(ctx context.Context, ownerUserID, nodeID uuid.UUID) (handlers.ShareNodeSummaryDTO, error) {
	item, err := a.repo.GetNodeSummaryForOwner(ctx, ownerUserID, nodeID)
	if err != nil {
		return handlers.ShareNodeSummaryDTO{}, err
	}
	return handlers.ShareNodeSummaryDTO{
		ID:          item.ID,
		OwnerUserID: item.OwnerUserID,
		Type:        item.Type,
		Name:        item.Name,
		SizeBytes:   item.SizeBytes,
		MIMEType:    item.MIMEType,
		UpdatedAt:   item.UpdatedAt,
		DeletedAt:   item.DeletedAt,
	}, nil
}

type filesDownloadAdapter struct {
	service *appfiles.Service
}

func (a filesDownloadAdapter) DownloadNode(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID) (handlers.ShareDownloadResult, error) {
	item, err := a.service.DownloadNode(ctx, ownerUserID, nodeID)
	if err != nil {
		return handlers.ShareDownloadResult{}, err
	}
	return handlers.ShareDownloadResult{
		FileName:  item.FileName,
		SizeBytes: item.SizeBytes,
		MIMEType:  item.MIMEType,
		Reader:    item.Reader,
	}, nil
}

func mapShare(item infraPostgres.ShareRecord) handlers.ShareDTO {
	return handlers.ShareDTO{
		ID:            item.ID,
		NodeID:        item.NodeID,
		OwnerUserID:   item.OwnerUserID,
		TokenHash:     item.TokenHash,
		PasswordHash:  item.PasswordHash,
		ExpiresAt:     item.ExpiresAt,
		MaxDownloads:  item.MaxDownloads,
		DownloadCount: item.DownloadCount,
		AllowDownload: item.AllowDownload,
		AllowPreview:  item.AllowPreview,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
		RevokedAt:     item.RevokedAt,
	}
}
