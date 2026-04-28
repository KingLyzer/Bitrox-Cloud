package files

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"sort"
	"strings"
	"time"

	domainfiles "cloud/backend/internal/domain/files"
	"cloud/backend/internal/infrastructure/content"
	"cloud/backend/internal/infrastructure/storage"

	"github.com/google/uuid"
)

const (
	defaultUploadSessionTTL        = 24 * time.Hour
	defaultMaxChunkBytes           = 16 * 1024 * 1024
	defaultMaxFileBytes            = int64(100 * 1024 * 1024 * 1024) // 100 GiB
	defaultMaxActiveUploadSessions = 64
	defaultMaxUserStagedBytes      = int64(200 * 1024 * 1024 * 1024) // 200 GiB
	maxIdempotencyKeyLength        = 200
)

type Repository interface {
	CreateFolder(ctx context.Context, input domainfiles.CreateFolderInput) (domainfiles.Node, error)
	GetNodeByID(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID) (domainfiles.Node, error)
	GetActiveNodeByID(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID) (domainfiles.Node, error)
	ListActiveNodesByParent(ctx context.Context, ownerUserID uuid.UUID, parentID *uuid.UUID) ([]domainfiles.Node, error)
	SearchActiveNodesByName(ctx context.Context, ownerUserID uuid.UUID, query string, nodeType *domainfiles.NodeType, limit int) ([]domainfiles.Node, error)
	ListDeletedNodes(ctx context.Context, ownerUserID uuid.UUID) ([]domainfiles.Node, error)
	RestoreNodeTree(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID) (domainfiles.Node, error)
	ListStorageKeysForNodeTree(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID) ([]string, error)
	HardDeleteNodeTree(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID) error
	ListDeletedRootNodesBefore(ctx context.Context, before time.Time, limit int) ([]domainfiles.Node, error)
	IsNodeInSubtree(ctx context.Context, ownerUserID, ancestorNodeID, candidateNodeID uuid.UUID) (bool, error)
	UpdateNodeNameAndParent(ctx context.Context, input domainfiles.UpdateNodeInput) (domainfiles.Node, error)
	SoftDeleteNodeTree(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID, now time.Time) error
	CreateUploadSession(ctx context.Context, input domainfiles.CreateUploadSessionInput) (domainfiles.UploadSession, error)
	CountActiveUploadSessions(ctx context.Context, ownerUserID uuid.UUID) (int, error)
	GetUserStagedBytes(ctx context.Context, ownerUserID uuid.UUID) (int64, error)
	GetUploadSessionByID(ctx context.Context, ownerUserID uuid.UUID, uploadSessionID uuid.UUID) (domainfiles.UploadSession, error)
	ListExpiredActiveUploadSessions(ctx context.Context, now time.Time, limit int) ([]domainfiles.UploadSession, error)
	ListUploadChunks(ctx context.Context, uploadSessionID uuid.UUID) ([]domainfiles.UploadChunk, error)
	UpsertUploadChunk(ctx context.Context, input domainfiles.UpsertUploadChunkInput) (domainfiles.UploadSession, error)
	AbortUploadSession(ctx context.Context, ownerUserID uuid.UUID, uploadSessionID uuid.UUID, now time.Time) (domainfiles.UploadSession, error)
	BeginFinalizeUploadSession(ctx context.Context, input domainfiles.BeginFinalizeUploadInput) (domainfiles.UploadSession, error)
	FinalizeUploadSession(ctx context.Context, input domainfiles.FinalizeUploadInput) (domainfiles.FinalizeUploadResult, error)
	GetCompletedFinalizeResult(ctx context.Context, ownerUserID, uploadSessionID uuid.UUID) (domainfiles.FinalizeUploadResult, error)
	MarkUploadSessionCleanupInProgress(ctx context.Context, ownerUserID, uploadSessionID uuid.UUID, now time.Time) (domainfiles.UploadSession, error)
	MarkUploadSessionCleanupResult(ctx context.Context, ownerUserID, uploadSessionID uuid.UUID, now time.Time, cleanupErr error) error
}

type Clock interface {
	Now() time.Time
}

type Config struct {
	UploadSessionTTL        time.Duration
	MaxChunkBytes           int
	MaxFileBytes            int64
	MaxActiveUploadSessions int
	MaxUserStagedBytes      int64
}

type Service struct {
	repo    Repository
	storage storage.ObjectStorage
	clock   Clock
	cfg     Config
	log     *slog.Logger
}

func NewService(repo Repository, objectStorage storage.ObjectStorage, clock Clock, cfg Config, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	if clock == nil {
		clock = RealClock{}
	}
	if cfg.UploadSessionTTL <= 0 {
		cfg.UploadSessionTTL = defaultUploadSessionTTL
	}
	if cfg.MaxChunkBytes <= 0 {
		cfg.MaxChunkBytes = defaultMaxChunkBytes
	}
	if cfg.MaxFileBytes <= 0 {
		cfg.MaxFileBytes = defaultMaxFileBytes
	}
	if cfg.MaxActiveUploadSessions <= 0 {
		cfg.MaxActiveUploadSessions = defaultMaxActiveUploadSessions
	}
	if cfg.MaxUserStagedBytes <= 0 {
		cfg.MaxUserStagedBytes = defaultMaxUserStagedBytes
	}
	return &Service{
		repo:    repo,
		storage: objectStorage,
		clock:   clock,
		cfg:     cfg,
		log:     log,
	}
}

type CreateFolderInput struct {
	OwnerUserID uuid.UUID
	ParentID    *uuid.UUID
	Name        string
}

func (s *Service) CreateFolder(ctx context.Context, input CreateFolderInput) (domainfiles.Node, error) {
	name := domainfiles.NormalizeNodeName(input.Name)
	if err := domainfiles.ValidateNodeName(name); err != nil {
		return domainfiles.Node{}, err
	}

	if input.ParentID != nil {
		parent, err := s.repo.GetActiveNodeByID(ctx, input.OwnerUserID, *input.ParentID)
		if err != nil {
			return domainfiles.Node{}, err
		}
		if parent.Type != domainfiles.NodeTypeFolder {
			return domainfiles.Node{}, domainfiles.ErrInvalidOperation
		}
	}

	return s.repo.CreateFolder(ctx, domainfiles.CreateFolderInput{
		OwnerUserID: input.OwnerUserID,
		ParentID:    input.ParentID,
		Name:        name,
	})
}

func (s *Service) ListNodes(ctx context.Context, ownerUserID uuid.UUID, parentID *uuid.UUID) ([]domainfiles.Node, error) {
	if parentID != nil {
		parent, err := s.repo.GetActiveNodeByID(ctx, ownerUserID, *parentID)
		if err != nil {
			return nil, err
		}
		if parent.Type != domainfiles.NodeTypeFolder {
			return nil, domainfiles.ErrInvalidOperation
		}
	}
	return s.repo.ListActiveNodesByParent(ctx, ownerUserID, parentID)
}

func (s *Service) SearchNodes(
	ctx context.Context,
	ownerUserID uuid.UUID,
	query string,
	nodeType *domainfiles.NodeType,
	limit int,
) ([]domainfiles.Node, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return []domainfiles.Node{}, nil
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	return s.repo.SearchActiveNodesByName(ctx, ownerUserID, trimmed, nodeType, limit)
}

func (s *Service) GetNode(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID) (domainfiles.Node, error) {
	return s.repo.GetActiveNodeByID(ctx, ownerUserID, nodeID)
}

func (s *Service) RenameNode(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID, newName string) (domainfiles.Node, error) {
	normalized := domainfiles.NormalizeNodeName(newName)
	if err := domainfiles.ValidateNodeName(normalized); err != nil {
		return domainfiles.Node{}, err
	}

	node, err := s.repo.GetActiveNodeByID(ctx, ownerUserID, nodeID)
	if err != nil {
		return domainfiles.Node{}, err
	}
	if node.Name == normalized {
		return node, nil
	}

	return s.repo.UpdateNodeNameAndParent(ctx, domainfiles.UpdateNodeInput{
		OwnerUserID: ownerUserID,
		NodeID:      nodeID,
		Name:        normalized,
		ParentID:    node.ParentID,
	})
}

func (s *Service) MoveNode(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID, newParentID *uuid.UUID) (domainfiles.Node, error) {
	node, err := s.repo.GetActiveNodeByID(ctx, ownerUserID, nodeID)
	if err != nil {
		return domainfiles.Node{}, err
	}

	if newParentID != nil {
		if *newParentID == node.ID {
			return domainfiles.Node{}, domainfiles.ErrInvalidOperation
		}

		parent, err := s.repo.GetActiveNodeByID(ctx, ownerUserID, *newParentID)
		if err != nil {
			return domainfiles.Node{}, err
		}
		if parent.Type != domainfiles.NodeTypeFolder {
			return domainfiles.Node{}, domainfiles.ErrInvalidOperation
		}

		if node.Type == domainfiles.NodeTypeFolder {
			isDescendant, err := s.repo.IsNodeInSubtree(ctx, ownerUserID, node.ID, *newParentID)
			if err != nil {
				return domainfiles.Node{}, err
			}
			if isDescendant {
				return domainfiles.Node{}, domainfiles.ErrInvalidOperation
			}
		}
	}

	if sameParent(node.ParentID, newParentID) {
		return node, nil
	}

	return s.repo.UpdateNodeNameAndParent(ctx, domainfiles.UpdateNodeInput{
		OwnerUserID: ownerUserID,
		NodeID:      node.ID,
		Name:        node.Name,
		ParentID:    newParentID,
	})
}

func (s *Service) DeleteNode(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID) error {
	return s.repo.SoftDeleteNodeTree(ctx, ownerUserID, nodeID, s.clock.Now().UTC())
}

func (s *Service) ListTrash(ctx context.Context, ownerUserID uuid.UUID) ([]domainfiles.Node, error) {
	return s.repo.ListDeletedNodes(ctx, ownerUserID)
}

func (s *Service) RestoreNode(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID) (domainfiles.Node, error) {
	return s.repo.RestoreNodeTree(ctx, ownerUserID, nodeID)
}

func (s *Service) PermanentlyDeleteNode(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID) error {
	keys, err := s.repo.ListStorageKeysForNodeTree(ctx, ownerUserID, nodeID)
	if err != nil {
		return err
	}
	if err := s.repo.HardDeleteNodeTree(ctx, ownerUserID, nodeID); err != nil {
		return err
	}
	for _, key := range keys {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if delErr := s.storage.Delete(ctx, key); delErr != nil {
			s.log.Warn("failed to cleanup object storage key for hard deleted node", slog.String("storage_key", key), slog.Any("error", delErr))
		}
	}
	return nil
}

func (s *Service) PermanentlyDeleteAllDeletedNodes(ctx context.Context, ownerUserID uuid.UUID) (int, error) {
	deletedNodes, err := s.repo.ListDeletedNodes(ctx, ownerUserID)
	if err != nil {
		return 0, err
	}
	if len(deletedNodes) == 0 {
		return 0, nil
	}

	deletedCount := 0
	for _, node := range deletedNodes {
		if err := s.PermanentlyDeleteNode(ctx, ownerUserID, node.ID); err != nil {
			if errors.Is(err, domainfiles.ErrNodeNotFound) {
				continue
			}
			return deletedCount, err
		}
		deletedCount++
	}
	return deletedCount, nil
}

func (s *Service) PurgeDeletedNodesBefore(ctx context.Context, before time.Time, limit int) (int, error) {
	roots, err := s.repo.ListDeletedRootNodesBefore(ctx, before, limit)
	if err != nil {
		return 0, err
	}
	purged := 0
	for _, node := range roots {
		if err := s.PermanentlyDeleteNode(ctx, node.OwnerUserID, node.ID); err != nil {
			s.log.Warn("failed to purge deleted node tree", slog.String("node_id", node.ID.String()), slog.Any("error", err))
			continue
		}
		purged += 1
	}
	return purged, nil
}

type CreateUploadSessionInput struct {
	OwnerUserID       uuid.UUID
	TargetType        domainfiles.UploadTargetType
	TargetNodeID      *uuid.UUID
	ParentID          *uuid.UUID
	FileName          string
	ExpectedSizeBytes int64
	ChunkSizeBytes    int
	IdempotencyKey    *string
}

func (s *Service) CreateUploadSession(ctx context.Context, input CreateUploadSessionInput) (domainfiles.UploadSession, error) {
	activeCount, err := s.repo.CountActiveUploadSessions(ctx, input.OwnerUserID)
	if err != nil {
		return domainfiles.UploadSession{}, err
	}
	if activeCount >= s.cfg.MaxActiveUploadSessions {
		return domainfiles.UploadSession{}, domainfiles.ErrTooManyActiveUploads
	}

	stagedBytes, err := s.repo.GetUserStagedBytes(ctx, input.OwnerUserID)
	if err != nil {
		return domainfiles.UploadSession{}, err
	}
	if stagedBytes >= s.cfg.MaxUserStagedBytes {
		return domainfiles.UploadSession{}, domainfiles.ErrStagedQuotaExceeded
	}

	if input.ExpectedSizeBytes <= 0 || input.ExpectedSizeBytes > s.cfg.MaxFileBytes {
		return domainfiles.UploadSession{}, domainfiles.ErrInvalidOperation
	}
	if input.ChunkSizeBytes <= 0 || input.ChunkSizeBytes > s.cfg.MaxChunkBytes {
		return domainfiles.UploadSession{}, domainfiles.ErrInvalidOperation
	}

	expectedChunks := int((input.ExpectedSizeBytes + int64(input.ChunkSizeBytes) - 1) / int64(input.ChunkSizeBytes))
	if expectedChunks <= 0 {
		return domainfiles.UploadSession{}, domainfiles.ErrInvalidOperation
	}

	fileName := domainfiles.NormalizeNodeName(input.FileName)
	targetNodeID := input.TargetNodeID
	parentID := input.ParentID

	switch input.TargetType {
	case domainfiles.UploadTargetTypeNewFile:
		if err := domainfiles.ValidateNodeName(fileName); err != nil {
			return domainfiles.UploadSession{}, err
		}
		if parentID != nil {
			parent, err := s.repo.GetActiveNodeByID(ctx, input.OwnerUserID, *parentID)
			if err != nil {
				return domainfiles.UploadSession{}, err
			}
			if parent.Type != domainfiles.NodeTypeFolder {
				return domainfiles.UploadSession{}, domainfiles.ErrInvalidOperation
			}
		}
	case domainfiles.UploadTargetTypeNewVersion:
		if targetNodeID == nil {
			return domainfiles.UploadSession{}, domainfiles.ErrInvalidOperation
		}
		node, err := s.repo.GetActiveNodeByID(ctx, input.OwnerUserID, *targetNodeID)
		if err != nil {
			return domainfiles.UploadSession{}, err
		}
		if node.Type != domainfiles.NodeTypeFile {
			return domainfiles.UploadSession{}, domainfiles.ErrInvalidOperation
		}
		parentID = nil
		if fileName == "" {
			fileName = node.Name
		}
		if err := domainfiles.ValidateNodeName(fileName); err != nil {
			return domainfiles.UploadSession{}, err
		}
	default:
		return domainfiles.UploadSession{}, domainfiles.ErrInvalidOperation
	}

	var normalizedIdempotency *string
	if input.IdempotencyKey != nil {
		v := strings.TrimSpace(*input.IdempotencyKey)
		if v != "" {
			if len(v) > maxIdempotencyKeyLength {
				return domainfiles.UploadSession{}, domainfiles.ErrInvalidOperation
			}
			normalizedIdempotency = &v
		}
	}

	fingerprint := buildUploadRequestFingerprint(input.TargetType, targetNodeID, parentID, fileName, input.ExpectedSizeBytes, input.ChunkSizeBytes)
	expiresAt := s.clock.Now().UTC().Add(s.cfg.UploadSessionTTL)

	return s.repo.CreateUploadSession(ctx, domainfiles.CreateUploadSessionInput{
		OwnerUserID:        input.OwnerUserID,
		TargetType:         input.TargetType,
		TargetNodeID:       targetNodeID,
		ParentID:           parentID,
		FileName:           fileName,
		ExpectedSizeBytes:  input.ExpectedSizeBytes,
		ChunkSizeBytes:     input.ChunkSizeBytes,
		ExpectedChunks:     expectedChunks,
		IdempotencyKey:     normalizedIdempotency,
		RequestFingerprint: &fingerprint,
		ExpiresAt:          expiresAt,
	})
}

type UploadChunkInput struct {
	OwnerUserID     uuid.UUID
	UploadSessionID uuid.UUID
	ChunkIndex      int
	ContentHash     string
	Body            io.Reader
}

type UploadChunkResult struct {
	Session        domainfiles.UploadSession
	ChunkHash      string
	ChunkSizeBytes int
}

func (s *Service) UploadChunk(ctx context.Context, input UploadChunkInput) (UploadChunkResult, error) {
	if strings.TrimSpace(input.ContentHash) == "" {
		return UploadChunkResult{}, domainfiles.ErrChunkHashRequired
	}
	expectedHash, err := normalizeSHA256Hex(input.ContentHash)
	if err != nil {
		return UploadChunkResult{}, domainfiles.ErrChunkHashMismatch
	}

	session, err := s.repo.GetUploadSessionByID(ctx, input.OwnerUserID, input.UploadSessionID)
	if err != nil {
		return UploadChunkResult{}, err
	}
	now := s.clock.Now().UTC()
	if session.Status == domainfiles.UploadSessionStatusCompleted {
		return UploadChunkResult{}, domainfiles.ErrInvalidOperation
	}
	if session.Status == domainfiles.UploadSessionStatusFinalizing {
		return UploadChunkResult{}, domainfiles.ErrFinalizeInProgress
	}
	if session.Status == domainfiles.UploadSessionStatusAborted {
		return UploadChunkResult{}, domainfiles.ErrUploadAborted
	}
	if !now.Before(session.ExpiresAt) {
		return UploadChunkResult{}, domainfiles.ErrUploadExpired
	}
	if input.ChunkIndex < 0 || input.ChunkIndex >= session.ExpectedChunks {
		return UploadChunkResult{}, domainfiles.ErrInvalidOperation
	}

	expectedSize := expectedChunkSize(session.ExpectedSizeBytes, session.ChunkSizeBytes, session.ExpectedChunks, input.ChunkIndex)
	payload, err := readChunkPayload(input.Body, s.cfg.MaxChunkBytes)
	if err != nil {
		return UploadChunkResult{}, err
	}
	if int64(len(payload)) != expectedSize {
		return UploadChunkResult{}, domainfiles.ErrInvalidOperation
	}

	hash := hashBytesHex(payload)
	if hash != expectedHash {
		return UploadChunkResult{}, domainfiles.ErrChunkHashMismatch
	}

	stagingKey := buildStagingObjectKey(input.OwnerUserID, input.UploadSessionID, input.ChunkIndex, hash)
	if _, err := s.storage.Put(ctx, stagingKey, bytes.NewReader(payload), int64(len(payload)), storage.PutOptions{
		ContentType: "application/octet-stream",
	}); err != nil {
		return UploadChunkResult{}, fmt.Errorf("store upload chunk: %w", err)
	}

	updatedSession, err := s.repo.UpsertUploadChunk(ctx, domainfiles.UpsertUploadChunkInput{
		OwnerUserID:        input.OwnerUserID,
		UploadSessionID:    input.UploadSessionID,
		ChunkIndex:         input.ChunkIndex,
		SizeBytes:          len(payload),
		ContentHash:        hash,
		StagingKey:         stagingKey,
		MaxUserStagedBytes: s.cfg.MaxUserStagedBytes,
		Now:                now,
	})
	if err != nil {
		_ = s.storage.Delete(ctx, stagingKey)
		return UploadChunkResult{}, err
	}

	return UploadChunkResult{
		Session:        updatedSession,
		ChunkHash:      hash,
		ChunkSizeBytes: len(payload),
	}, nil
}

type UploadSessionDetails struct {
	Session domainfiles.UploadSession
	Chunks  []domainfiles.UploadChunk
}

func (s *Service) GetUploadSession(ctx context.Context, ownerUserID uuid.UUID, uploadSessionID uuid.UUID) (UploadSessionDetails, error) {
	session, err := s.repo.GetUploadSessionByID(ctx, ownerUserID, uploadSessionID)
	if err != nil {
		return UploadSessionDetails{}, err
	}
	chunks, err := s.repo.ListUploadChunks(ctx, uploadSessionID)
	if err != nil {
		return UploadSessionDetails{}, err
	}
	return UploadSessionDetails{
		Session: session,
		Chunks:  chunks,
	}, nil
}

func (s *Service) AbortUploadSession(ctx context.Context, ownerUserID uuid.UUID, uploadSessionID uuid.UUID) (domainfiles.UploadSession, error) {
	session, err := s.repo.GetUploadSessionByID(ctx, ownerUserID, uploadSessionID)
	if err != nil {
		return domainfiles.UploadSession{}, err
	}
	if session.Status == domainfiles.UploadSessionStatusCompleted {
		return domainfiles.UploadSession{}, domainfiles.ErrInvalidOperation
	}
	if session.Status == domainfiles.UploadSessionStatusAborted {
		return session, nil
	}

	aborted, err := s.repo.AbortUploadSession(ctx, ownerUserID, uploadSessionID, s.clock.Now().UTC())
	if err != nil {
		return domainfiles.UploadSession{}, err
	}

	chunks, listErr := s.repo.ListUploadChunks(ctx, uploadSessionID)
	if listErr != nil {
		s.log.Warn("failed to list upload chunks during abort cleanup", slog.Any("error", listErr), slog.String("upload_session_id", uploadSessionID.String()))
		return aborted, nil
	}
	s.cleanupStagingObjects(ctx, chunks)
	return aborted, nil
}

func (s *Service) FinalizeUploadSession(ctx context.Context, ownerUserID uuid.UUID, uploadSessionID uuid.UUID) (domainfiles.FinalizeUploadResult, error) {
	now := s.clock.Now().UTC()

	finalObjectKey := buildFinalObjectKey(ownerUserID, uploadSessionID, uuid.New())
	session, err := s.repo.BeginFinalizeUploadSession(ctx, domainfiles.BeginFinalizeUploadInput{
		OwnerUserID:     ownerUserID,
		UploadSessionID: uploadSessionID,
		ObjectKey:       finalObjectKey,
		Now:             now,
	})
	if err != nil {
		return domainfiles.FinalizeUploadResult{}, err
	}

	if session.Status == domainfiles.UploadSessionStatusCompleted {
		result, err := s.repo.GetCompletedFinalizeResult(ctx, ownerUserID, uploadSessionID)
		if err != nil {
			return domainfiles.FinalizeUploadResult{}, err
		}
		if session.ObjectKey != nil && strings.TrimSpace(*session.ObjectKey) != strings.TrimSpace(finalObjectKey) {
			_ = s.storage.Delete(ctx, finalObjectKey)
		}
		return result, nil
	}
	if session.Status != domainfiles.UploadSessionStatusFinalizing {
		return domainfiles.FinalizeUploadResult{}, domainfiles.ErrInvalidOperation
	}

	chunks, err := s.repo.ListUploadChunks(ctx, uploadSessionID)
	if err != nil {
		return domainfiles.FinalizeUploadResult{}, err
	}
	if len(chunks) != session.ExpectedChunks {
		return domainfiles.FinalizeUploadResult{}, domainfiles.ErrUploadIncomplete
	}

	if err := s.composeFinalObject(ctx, finalObjectKey, chunks, session.ExpectedSizeBytes, session.ExpectedChunks); err != nil {
		return domainfiles.FinalizeUploadResult{}, err
	}

	analysis, err := s.analyzeObject(ctx, finalObjectKey)
	if err != nil {
		_ = s.storage.Delete(ctx, finalObjectKey)
		return domainfiles.FinalizeUploadResult{}, err
	}
	if analysis.SizeBytes != session.ExpectedSizeBytes {
		_ = s.storage.Delete(ctx, finalObjectKey)
		return domainfiles.FinalizeUploadResult{}, domainfiles.ErrUploadIncomplete
	}

	result, err := s.repo.FinalizeUploadSession(ctx, domainfiles.FinalizeUploadInput{
		OwnerUserID:     ownerUserID,
		UploadSessionID: uploadSessionID,
		ObjectKey:       finalObjectKey,
		MIMEType:        analysis.MIMEType,
		ContentHash:     analysis.HashHex,
		SizeBytes:       analysis.SizeBytes,
		Now:             now,
	})
	if err != nil {
		var commitUnknownErr *domainfiles.CommitUnknownError
		if errors.As(err, &commitUnknownErr) {
			resolved, resolveErr := s.resolveFinalizeCommitUnknown(ctx, ownerUserID, uploadSessionID, finalObjectKey, commitUnknownErr)
			if resolveErr == nil {
				s.cleanupStagingObjects(ctx, chunks)
				return resolved, nil
			}
		}

		afterSession, loadErr := s.repo.GetUploadSessionByID(ctx, ownerUserID, uploadSessionID)
		if loadErr == nil {
			if afterSession.Status == domainfiles.UploadSessionStatusCompleted {
				if afterSession.ObjectKey != nil && strings.TrimSpace(*afterSession.ObjectKey) != strings.TrimSpace(finalObjectKey) {
					_ = s.storage.Delete(ctx, finalObjectKey)
				}
			} else {
				// Not committed to completed state, safe to remove local attempt object.
				_ = s.storage.Delete(ctx, finalObjectKey)
			}
		}
		return domainfiles.FinalizeUploadResult{}, err
	}
	if result.WasAlreadyFinal {
		if result.Session.ObjectKey == nil || strings.TrimSpace(*result.Session.ObjectKey) != strings.TrimSpace(finalObjectKey) {
			_ = s.storage.Delete(ctx, finalObjectKey)
		}
	}

	s.cleanupStagingObjects(ctx, chunks)
	return result, nil
}

type DownloadResult struct {
	Reader    io.ReadCloser
	SizeBytes int64
	FileName  string
	MIMEType  string
}

func (s *Service) DownloadNode(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID) (DownloadResult, error) {
	node, err := s.repo.GetActiveNodeByID(ctx, ownerUserID, nodeID)
	if err != nil {
		return DownloadResult{}, err
	}
	if node.Type == domainfiles.NodeTypeFolder {
		reader, err := s.streamFolderArchive(ctx, ownerUserID, node)
		if err != nil {
			return DownloadResult{}, err
		}
		return DownloadResult{
			Reader:    reader,
			SizeBytes: -1,
			FileName:  safeDownloadFileName(node.Name) + ".zip",
			MIMEType:  "application/zip",
		}, nil
	}

	if node.Type != domainfiles.NodeTypeFile || node.StorageKey == nil {
		return DownloadResult{}, domainfiles.ErrInvalidOperation
	}

	reader, meta, err := s.storage.Open(ctx, *node.StorageKey)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("open node object: %w", err)
	}

	mime := "application/octet-stream"
	if node.MIMEType != nil && strings.TrimSpace(*node.MIMEType) != "" {
		mime = strings.TrimSpace(*node.MIMEType)
	} else if guessedTextMime := inferTextMimeType(node.Name); guessedTextMime != "" {
		mime = guessedTextMime
	}
	return DownloadResult{
		Reader:    reader,
		SizeBytes: meta.SizeBytes,
		FileName:  safeDownloadFileName(node.Name),
		MIMEType:  mime,
	}, nil
}

func (s *Service) streamFolderArchive(ctx context.Context, ownerUserID uuid.UUID, root domainfiles.Node) (io.ReadCloser, error) {
	reader, writer := io.Pipe()

	go func() {
		zipWriter := zip.NewWriter(writer)
		baseFolderName := safeZipSegment(root.Name)
		if baseFolderName == "" {
			baseFolderName = "folder"
		}

		err := s.writeFolderToZip(ctx, ownerUserID, root.ID, baseFolderName, zipWriter)
		closeErr := zipWriter.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
		if err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		_ = writer.Close()
	}()

	return reader, nil
}

func (s *Service) writeFolderToZip(
	ctx context.Context,
	ownerUserID uuid.UUID,
	parentID uuid.UUID,
	basePath string,
	zipWriter *zip.Writer,
) error {
	children, err := s.repo.ListActiveNodesByParent(ctx, ownerUserID, &parentID)
	if err != nil {
		return fmt.Errorf("list folder children: %w", err)
	}

	for _, child := range children {
		childName := safeZipSegment(child.Name)
		if childName == "" {
			childName = child.ID.String()
		}
		entryPath := path.Join(basePath, childName)
		if child.Type == domainfiles.NodeTypeFolder {
			if _, err := zipWriter.Create(entryPath + "/"); err != nil {
				return fmt.Errorf("create folder archive entry: %w", err)
			}
			if err := s.writeFolderToZip(ctx, ownerUserID, child.ID, entryPath, zipWriter); err != nil {
				return err
			}
			continue
		}

		if child.StorageKey == nil {
			continue
		}

		source, _, err := s.storage.Open(ctx, *child.StorageKey)
		if err != nil {
			return fmt.Errorf("open file for archive: %w", err)
		}

		entryWriter, err := zipWriter.Create(entryPath)
		if err != nil {
			_ = source.Close()
			return fmt.Errorf("create file archive entry: %w", err)
		}
		if _, err := io.Copy(entryWriter, source); err != nil {
			_ = source.Close()
			return fmt.Errorf("copy file to archive: %w", err)
		}
		if err := source.Close(); err != nil {
			return fmt.Errorf("close archived file reader: %w", err)
		}
	}

	return nil
}

func (s *Service) composeFinalObject(
	ctx context.Context,
	finalObjectKey string,
	chunks []domainfiles.UploadChunk,
	expectedSizeBytes int64,
	expectedChunkCount int,
) error {
	if len(chunks) != expectedChunkCount {
		return domainfiles.ErrUploadIncomplete
	}

	sorted := append([]domainfiles.UploadChunk(nil), chunks...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ChunkIndex < sorted[j].ChunkIndex
	})
	for i := 0; i < len(sorted); i++ {
		if sorted[i].ChunkIndex != i {
			return domainfiles.ErrUploadIncomplete
		}
	}

	readClosers := make([]io.ReadCloser, 0, len(sorted))
	readers := make([]io.Reader, 0, len(sorted))
	for _, chunk := range sorted {
		reader, _, err := s.storage.Open(ctx, chunk.StagingKey)
		if err != nil {
			for _, closer := range readClosers {
				_ = closer.Close()
			}
			return fmt.Errorf("open staging chunk object: %w", err)
		}
		readClosers = append(readClosers, reader)
		readers = append(readers, reader)
	}
	defer func() {
		for _, closer := range readClosers {
			_ = closer.Close()
		}
	}()

	multiReader := io.MultiReader(readers...)
	if _, err := s.storage.Put(ctx, finalObjectKey, multiReader, expectedSizeBytes, storage.PutOptions{
		ContentType: "application/octet-stream",
	}); err != nil {
		return fmt.Errorf("store final object: %w", err)
	}
	return nil
}

func (s *Service) analyzeObject(ctx context.Context, objectKey string) (content.AnalyzeResult, error) {
	reader, _, err := s.storage.Open(ctx, objectKey)
	if err != nil {
		return content.AnalyzeResult{}, fmt.Errorf("open object for analysis: %w", err)
	}
	defer reader.Close()

	result, err := content.AnalyzeReader(reader)
	if err != nil {
		return content.AnalyzeResult{}, fmt.Errorf("analyze object body: %w", err)
	}
	return result, nil
}

func (s *Service) cleanupStagingObjects(ctx context.Context, chunks []domainfiles.UploadChunk) {
	for _, chunk := range chunks {
		if err := s.storage.Delete(ctx, chunk.StagingKey); err != nil {
			s.log.Warn("failed to cleanup staging chunk object", slog.Any("error", err), slog.String("staging_key", chunk.StagingKey))
		}
	}
}

func (s *Service) resolveFinalizeCommitUnknown(
	ctx context.Context,
	ownerUserID uuid.UUID,
	uploadSessionID uuid.UUID,
	localObjectKey string,
	commitUnknownErr *domainfiles.CommitUnknownError,
) (domainfiles.FinalizeUploadResult, error) {
	session, err := s.repo.GetUploadSessionByID(ctx, ownerUserID, uploadSessionID)
	if err != nil {
		return domainfiles.FinalizeUploadResult{}, fmt.Errorf("resolve commit unknown by loading session: %w", err)
	}
	if session.Status != domainfiles.UploadSessionStatusCompleted {
		return domainfiles.FinalizeUploadResult{}, commitUnknownErr
	}
	if session.ObjectKey == nil || strings.TrimSpace(*session.ObjectKey) != strings.TrimSpace(localObjectKey) {
		return domainfiles.FinalizeUploadResult{}, commitUnknownErr
	}
	result, err := s.repo.GetCompletedFinalizeResult(ctx, ownerUserID, uploadSessionID)
	if err != nil {
		return domainfiles.FinalizeUploadResult{}, fmt.Errorf("resolve commit unknown by loading finalize result: %w", err)
	}
	return result, nil
}

func expectedChunkSize(expectedSizeBytes int64, chunkSizeBytes int, expectedChunks int, chunkIndex int) int64 {
	if chunkIndex < expectedChunks-1 {
		return int64(chunkSizeBytes)
	}
	prefixBytes := int64(expectedChunks-1) * int64(chunkSizeBytes)
	return expectedSizeBytes - prefixBytes
}

func readChunkPayload(reader io.Reader, maxBytes int) ([]byte, error) {
	limited := io.LimitReader(reader, int64(maxBytes+1))
	payload, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read chunk payload: %w", err)
	}
	if len(payload) == 0 || len(payload) > maxBytes {
		return nil, domainfiles.ErrInvalidOperation
	}
	return payload, nil
}

func hashBytesHex(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func normalizeSHA256Hex(raw string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if len(trimmed) != 64 {
		return "", domainfiles.ErrChunkHashMismatch
	}
	if _, err := hex.DecodeString(trimmed); err != nil {
		return "", domainfiles.ErrChunkHashMismatch
	}
	return trimmed, nil
}

func buildUploadRequestFingerprint(
	targetType domainfiles.UploadTargetType,
	targetNodeID *uuid.UUID,
	parentID *uuid.UUID,
	fileName string,
	expectedSizeBytes int64,
	chunkSizeBytes int,
) string {
	targetNode := ""
	if targetNodeID != nil {
		targetNode = targetNodeID.String()
	}
	parent := ""
	if parentID != nil {
		parent = parentID.String()
	}
	payload := fmt.Sprintf("%s|%s|%s|%s|%d|%d", targetType, targetNode, parent, fileName, expectedSizeBytes, chunkSizeBytes)
	return hashBytesHex([]byte(payload))
}

func buildStagingObjectKey(ownerUserID uuid.UUID, uploadSessionID uuid.UUID, chunkIndex int, chunkHash string) string {
	return fmt.Sprintf("uploads/%s/%s/chunks/%08d/%s", ownerUserID.String(), uploadSessionID.String(), chunkIndex, chunkHash)
}

func buildFinalObjectKey(ownerUserID uuid.UUID, uploadSessionID uuid.UUID, objectID uuid.UUID) string {
	return fmt.Sprintf("objects/%s/%s/%s", ownerUserID.String(), uploadSessionID.String(), objectID.String())
}

func sameParent(a *uuid.UUID, b *uuid.UUID) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func safeZipSegment(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	trimmed = path.Base(trimmed)
	trimmed = strings.ReplaceAll(trimmed, "..", "")
	trimmed = strings.Trim(trimmed, "/")
	return trimmed
}

func safeDownloadFileName(raw string) string {
	safe := safeZipSegment(raw)
	safe = strings.ReplaceAll(safe, "\"", "")
	safe = strings.ReplaceAll(safe, "\n", "")
	safe = strings.ReplaceAll(safe, "\r", "")
	if safe == "" {
		return "download"
	}
	return safe
}

func inferTextMimeType(fileName string) string {
	normalized := strings.ToLower(strings.TrimSpace(fileName))
	switch {
	case strings.HasSuffix(normalized, ".txt"), strings.HasSuffix(normalized, ".log"), strings.HasSuffix(normalized, ".md"):
		return "text/plain; charset=utf-8"
	case strings.HasSuffix(normalized, ".csv"):
		return "text/csv; charset=utf-8"
	case strings.HasSuffix(normalized, ".json"):
		return "application/json; charset=utf-8"
	default:
		return ""
	}
}

type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}
