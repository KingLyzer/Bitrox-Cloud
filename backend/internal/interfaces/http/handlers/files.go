package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	appauth "cloud/backend/internal/application/auth"
	appfiles "cloud/backend/internal/application/files"
	domainfiles "cloud/backend/internal/domain/files"
	"cloud/backend/internal/interfaces/http/authctx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type FilesHandler struct {
	log     *slog.Logger
	service *appfiles.Service
	audit   appauth.AuditRecorder
}

func NewFilesHandler(log *slog.Logger, service *appfiles.Service, audit appauth.AuditRecorder) *FilesHandler {
	if log == nil {
		log = slog.Default()
	}
	return &FilesHandler{
		log:     log,
		service: service,
		audit:   audit,
	}
}

type createFolderRequest struct {
	ParentID *string `json:"parent_id"`
	Name     string  `json:"name"`
}

func (h *FilesHandler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req createFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	parentID, err := parseOptionalUUID(req.ParentID)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid parent_id")
		return
	}

	node, err := h.service.CreateFolder(r.Context(), appfiles.CreateFolderInput{
		OwnerUserID: authValue.UserID,
		ParentID:    parentID,
		Name:        req.Name,
	})
	if err != nil {
		h.writeFilesError(w, err, "create folder failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"node": nodeToResponse(node),
	})
}

func (h *FilesHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	parentIDRaw := strings.TrimSpace(r.URL.Query().Get("parent_id"))
	var parentID *uuid.UUID
	if parentIDRaw != "" {
		parsed, err := uuid.Parse(parentIDRaw)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid parent_id")
			return
		}
		parentID = &parsed
	}

	nodes, err := h.service.ListNodes(r.Context(), authValue.UserID, parentID)
	if err != nil {
		h.writeFilesError(w, err, "list nodes failed")
		return
	}

	resp := make([]map[string]any, 0, len(nodes))
	for _, node := range nodes {
		resp = append(resp, nodeToResponse(node))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"nodes": resp,
	})
}

func (h *FilesHandler) GetNode(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	nodeID, err := parseURLParamUUID(r, "nodeID")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid node id")
		return
	}

	node, err := h.service.GetNode(r.Context(), authValue.UserID, nodeID)
	if err != nil {
		h.writeFilesError(w, err, "get node failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"node": nodeToResponse(node),
	})
}

type renameNodeRequest struct {
	Name string `json:"name"`
}

func (h *FilesHandler) RenameNode(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	nodeID, err := parseURLParamUUID(r, "nodeID")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid node id")
		return
	}

	var req renameNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	node, err := h.service.RenameNode(r.Context(), authValue.UserID, nodeID, req.Name)
	if err != nil {
		h.writeFilesError(w, err, "rename node failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"node": nodeToResponse(node),
	})
}

type moveNodeRequest struct {
	ParentID *string `json:"parent_id"`
}

func (h *FilesHandler) MoveNode(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	nodeID, err := parseURLParamUUID(r, "nodeID")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid node id")
		return
	}

	var req moveNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	parentID, err := parseOptionalUUID(req.ParentID)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid parent_id")
		return
	}

	node, err := h.service.MoveNode(r.Context(), authValue.UserID, nodeID, parentID)
	if err != nil {
		h.writeFilesError(w, err, "move node failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"node": nodeToResponse(node),
	})
}

func (h *FilesHandler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	nodeID, err := parseURLParamUUID(r, "nodeID")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid node id")
		return
	}

	if err := h.service.DeleteNode(r.Context(), authValue.UserID, nodeID); err != nil {
		h.writeFilesError(w, err, "delete node failed")
		return
	}
	h.recordAudit(r, authValue, "files.node.deleted", "medium", map[string]any{
		"node_id": nodeID.String(),
	})
	w.WriteHeader(http.StatusNoContent)
}

type createUploadSessionRequest struct {
	TargetType        string  `json:"target_type"`
	TargetNodeID      *string `json:"target_node_id"`
	ParentID          *string `json:"parent_id"`
	FileName          string  `json:"file_name"`
	ExpectedSizeBytes int64   `json:"expected_size_bytes"`
	ChunkSizeBytes    int     `json:"chunk_size_bytes"`
}

func (h *FilesHandler) CreateUploadSession(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req createUploadSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	targetNodeID, err := parseOptionalUUID(req.TargetNodeID)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid target_node_id")
		return
	}
	parentID, err := parseOptionalUUID(req.ParentID)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid parent_id")
		return
	}

	var idempotencyKey *string
	if raw := strings.TrimSpace(r.Header.Get("Idempotency-Key")); raw != "" {
		idempotencyKey = &raw
	}

	session, err := h.service.CreateUploadSession(r.Context(), appfiles.CreateUploadSessionInput{
		OwnerUserID:       authValue.UserID,
		TargetType:        domainfiles.UploadTargetType(strings.TrimSpace(req.TargetType)),
		TargetNodeID:      targetNodeID,
		ParentID:          parentID,
		FileName:          req.FileName,
		ExpectedSizeBytes: req.ExpectedSizeBytes,
		ChunkSizeBytes:    req.ChunkSizeBytes,
		IdempotencyKey:    idempotencyKey,
	})
	if err != nil {
		h.writeFilesError(w, err, "create upload session failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"upload_session": uploadSessionToResponse(session),
	})
}

func (h *FilesHandler) UploadChunk(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	uploadSessionID, err := parseURLParamUUID(r, "uploadSessionID")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid upload session id")
		return
	}
	chunkIndexRaw := strings.TrimSpace(chi.URLParam(r, "chunkIndex"))
	chunkIndex, err := strconv.Atoi(chunkIndexRaw)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid chunk index")
		return
	}

	result, err := h.service.UploadChunk(r.Context(), appfiles.UploadChunkInput{
		OwnerUserID:     authValue.UserID,
		UploadSessionID: uploadSessionID,
		ChunkIndex:      chunkIndex,
		ContentHash:     strings.TrimSpace(r.Header.Get("X-Chunk-SHA256")),
		Body:            r.Body,
	})
	if err != nil {
		h.writeFilesError(w, err, "upload chunk failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"upload_session":   uploadSessionToResponse(result.Session),
		"chunk_hash":       result.ChunkHash,
		"chunk_size_bytes": result.ChunkSizeBytes,
	})
}

func (h *FilesHandler) GetUploadSession(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	uploadSessionID, err := parseURLParamUUID(r, "uploadSessionID")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid upload session id")
		return
	}

	details, err := h.service.GetUploadSession(r.Context(), authValue.UserID, uploadSessionID)
	if err != nil {
		h.writeFilesError(w, err, "get upload session failed")
		return
	}

	chunks := make([]map[string]any, 0, len(details.Chunks))
	for _, chunk := range details.Chunks {
		chunks = append(chunks, map[string]any{
			"chunk_index":  chunk.ChunkIndex,
			"size_bytes":   chunk.SizeBytes,
			"content_hash": chunk.ContentHash,
			"created_at":   chunk.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"upload_session": uploadSessionToResponse(details.Session),
		"chunks":         chunks,
	})
}

func (h *FilesHandler) FinalizeUploadSession(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	uploadSessionID, err := parseURLParamUUID(r, "uploadSessionID")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid upload session id")
		return
	}

	result, err := h.service.FinalizeUploadSession(r.Context(), authValue.UserID, uploadSessionID)
	if err != nil {
		h.writeFilesError(w, err, "finalize upload session failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"upload_session": uploadSessionToResponse(result.Session),
		"node":           nodeToResponse(result.Node),
		"version": map[string]any{
			"id":                 result.Version.ID.String(),
			"node_id":            result.Version.NodeID.String(),
			"version_no":         result.Version.VersionNo,
			"storage_key":        result.Version.StorageKey,
			"size_bytes":         result.Version.SizeBytes,
			"mime_type":          result.Version.MIMEType,
			"content_hash":       result.Version.ContentHash,
			"created_by_user_id": result.Version.CreatedByUserID.String(),
			"created_at":         result.Version.CreatedAt,
		},
		"already_finalized": result.WasAlreadyFinal,
	})

	h.recordAudit(r, authValue, "files.upload.finalized", "info", map[string]any{
		"upload_session_id": uploadSessionID.String(),
		"node_id":           result.Node.ID.String(),
		"size_bytes":        result.Version.SizeBytes,
		"mime_type":         result.Version.MIMEType,
	})
}

func (h *FilesHandler) AbortUploadSession(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	uploadSessionID, err := parseURLParamUUID(r, "uploadSessionID")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid upload session id")
		return
	}

	session, err := h.service.AbortUploadSession(r.Context(), authValue.UserID, uploadSessionID)
	if err != nil {
		h.writeFilesError(w, err, "abort upload session failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"upload_session": uploadSessionToResponse(session),
	})
}

func (h *FilesHandler) DownloadNode(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	nodeID, err := parseURLParamUUID(r, "nodeID")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid node id")
		return
	}

	result, err := h.service.DownloadNode(r.Context(), authValue.UserID, nodeID)
	if err != nil {
		h.writeFilesError(w, err, "download node failed")
		return
	}
	defer result.Reader.Close()

	w.Header().Set("Content-Type", result.MIMEType)
	if result.SizeBytes >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(result.SizeBytes, 10))
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", result.FileName))

	if _, err := io.Copy(w, result.Reader); err != nil {
		h.log.Warn("download stream interrupted", slog.Any("error", err), slog.String("node_id", nodeID.String()))
	}
}

func (h *FilesHandler) DownloadByQuery(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	nodeIDRaw := strings.TrimSpace(r.URL.Query().Get("node_id"))
	if nodeIDRaw == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "node_id is required")
		return
	}

	nodeID, err := uuid.Parse(nodeIDRaw)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid node_id")
		return
	}

	result, err := h.service.DownloadNode(r.Context(), authValue.UserID, nodeID)
	if err != nil {
		h.writeFilesError(w, err, "download node by query failed")
		return
	}
	defer result.Reader.Close()

	w.Header().Set("Content-Type", result.MIMEType)
	if result.SizeBytes >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(result.SizeBytes, 10))
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", result.FileName))

	if _, err := io.Copy(w, result.Reader); err != nil {
		h.log.Warn("download-by-query stream interrupted", slog.Any("error", err), slog.String("node_id", nodeID.String()))
	}
}

func (h *FilesHandler) writeFilesError(w http.ResponseWriter, err error, logMessage string) {
	switch {
	case errors.Is(err, domainfiles.ErrNodeNotFound):
		writeAPIError(w, http.StatusNotFound, "node_not_found", "node not found")
	case errors.Is(err, domainfiles.ErrUploadSessionNotFound):
		writeAPIError(w, http.StatusNotFound, "upload_session_not_found", "upload session not found")
	case errors.Is(err, domainfiles.ErrNameConflict):
		writeAPIError(w, http.StatusConflict, "name_conflict", "name conflict")
	case errors.Is(err, domainfiles.ErrIdempotencyConflict):
		writeAPIError(w, http.StatusConflict, "idempotency_conflict", "idempotency key conflict")
	case errors.Is(err, domainfiles.ErrQuotaExceeded):
		writeAPIError(w, http.StatusRequestEntityTooLarge, "quota_exceeded", "quota exceeded")
	case errors.Is(err, domainfiles.ErrUploadIncomplete):
		writeAPIError(w, http.StatusConflict, "upload_incomplete", "upload incomplete")
	case errors.Is(err, domainfiles.ErrUploadExpired):
		writeAPIError(w, http.StatusConflict, "upload_expired", "upload expired")
	case errors.Is(err, domainfiles.ErrUploadAborted):
		writeAPIError(w, http.StatusConflict, "upload_aborted", "upload aborted")
	case errors.Is(err, domainfiles.ErrFinalizeInProgress):
		writeAPIError(w, http.StatusConflict, "finalize_in_progress", "finalize in progress")
	case errors.Is(err, domainfiles.ErrChunkConflict):
		writeAPIError(w, http.StatusConflict, "chunk_conflict", "chunk payload conflict")
	case errors.Is(err, domainfiles.ErrChunkHashRequired):
		writeAPIError(w, http.StatusBadRequest, "chunk_hash_required", "chunk hash is required")
	case errors.Is(err, domainfiles.ErrChunkHashMismatch):
		writeAPIError(w, http.StatusBadRequest, "chunk_hash_mismatch", "chunk hash mismatch")
	case errors.Is(err, domainfiles.ErrStagedQuotaExceeded):
		writeAPIError(w, http.StatusRequestEntityTooLarge, "staged_quota_exceeded", "staged quota exceeded")
	case errors.Is(err, domainfiles.ErrTooManyActiveUploads):
		writeAPIError(w, http.StatusTooManyRequests, "too_many_active_uploads", "too many active upload sessions")
	case errors.Is(err, domainfiles.ErrInvalidName):
		writeAPIError(w, http.StatusBadRequest, "invalid_name", "invalid name")
	case errors.Is(err, domainfiles.ErrInvalidOperation):
		writeAPIError(w, http.StatusBadRequest, "invalid_operation", "invalid operation")
	default:
		h.log.Error(logMessage, slog.Any("error", err))
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func parseURLParamUUID(r *http.Request, name string) (uuid.UUID, error) {
	return uuid.Parse(strings.TrimSpace(chi.URLParam(r, name)))
}

func parseOptionalUUID(raw *string) (*uuid.UUID, error) {
	if raw == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func nodeToResponse(node domainfiles.Node) map[string]any {
	resp := map[string]any{
		"id":            node.ID.String(),
		"owner_user_id": node.OwnerUserID.String(),
		"type":          string(node.Type),
		"name":          node.Name,
		"size_bytes":    node.SizeBytes,
		"created_at":    node.CreatedAt,
		"updated_at":    node.UpdatedAt,
	}
	if node.ParentID != nil {
		resp["parent_id"] = node.ParentID.String()
	} else {
		resp["parent_id"] = nil
	}
	if node.MIMEType != nil {
		resp["mime_type"] = *node.MIMEType
	}
	if node.ContentHash != nil {
		resp["content_hash"] = *node.ContentHash
	}
	if node.CurrentVersionID != nil {
		resp["current_version_id"] = node.CurrentVersionID.String()
	}
	if node.CurrentVersionNo != nil {
		resp["current_version_no"] = *node.CurrentVersionNo
	}
	if node.DeletedAt != nil {
		resp["deleted_at"] = *node.DeletedAt
	}
	return resp
}

func uploadSessionToResponse(session domainfiles.UploadSession) map[string]any {
	resp := map[string]any{
		"id":                  session.ID.String(),
		"owner_user_id":       session.OwnerUserID.String(),
		"target_type":         string(session.TargetType),
		"file_name":           session.FileName,
		"expected_size_bytes": session.ExpectedSizeBytes,
		"chunk_size_bytes":    session.ChunkSizeBytes,
		"expected_chunks":     session.ExpectedChunks,
		"status":              string(session.Status),
		"uploaded_bytes":      session.UploadedBytes,
		"expires_at":          session.ExpiresAt,
		"created_at":          session.CreatedAt,
		"updated_at":          session.UpdatedAt,
	}
	if session.TargetNodeID != nil {
		resp["target_node_id"] = session.TargetNodeID.String()
	}
	if session.ParentID != nil {
		resp["parent_id"] = session.ParentID.String()
	} else {
		resp["parent_id"] = nil
	}
	if session.IdempotencyKey != nil {
		resp["idempotency_key"] = *session.IdempotencyKey
	}
	if session.FinalizedNodeID != nil {
		resp["finalized_node_id"] = session.FinalizedNodeID.String()
	}
	if session.FinalizedVersionID != nil {
		resp["finalized_version_id"] = session.FinalizedVersionID.String()
	}
	if session.MIMEType != nil {
		resp["mime_type"] = *session.MIMEType
	}
	if session.ContentHash != nil {
		resp["content_hash"] = *session.ContentHash
	}
	if session.CompletedAt != nil {
		resp["completed_at"] = *session.CompletedAt
	}
	return resp
}

func (h *FilesHandler) recordAudit(r *http.Request, authValue authctx.Context, eventType string, severity string, metadata map[string]any) {
	if h.audit == nil {
		return
	}
	userID := authValue.UserID
	sessionID := authValue.SessionID
	if metadata == nil {
		metadata = map[string]any{}
	}
	if err := h.audit.RecordSecurityEvent(r.Context(), appauth.SecurityEvent{
		EventType: eventType,
		Severity:  severity,
		UserID:    &userID,
		SessionID: &sessionID,
		IP:        requestAuditIP(r),
		UserAgent: strings.TrimSpace(r.UserAgent()),
		Metadata:  metadata,
	}); err != nil {
		h.log.Warn("audit record failed", slog.Any("error", err), slog.String("event_type", eventType))
	}
}
