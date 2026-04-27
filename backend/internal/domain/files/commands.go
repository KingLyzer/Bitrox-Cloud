package files

import (
	"time"

	"github.com/google/uuid"
)

type CreateFolderInput struct {
	OwnerUserID uuid.UUID
	ParentID    *uuid.UUID
	Name        string
}

type UpdateNodeInput struct {
	OwnerUserID uuid.UUID
	NodeID      uuid.UUID
	Name        string
	ParentID    *uuid.UUID
}

type CreateUploadSessionInput struct {
	OwnerUserID        uuid.UUID
	TargetType         UploadTargetType
	TargetNodeID       *uuid.UUID
	ParentID           *uuid.UUID
	FileName           string
	ExpectedSizeBytes  int64
	ChunkSizeBytes     int
	ExpectedChunks     int
	IdempotencyKey     *string
	RequestFingerprint *string
	ExpiresAt          time.Time
}

type UpsertUploadChunkInput struct {
	OwnerUserID     uuid.UUID
	UploadSessionID uuid.UUID
	ChunkIndex      int
	SizeBytes       int
	ContentHash     string
	StagingKey      string
	MaxUserStagedBytes int64
	Now             time.Time
}

type BeginFinalizeUploadInput struct {
	OwnerUserID     uuid.UUID
	UploadSessionID uuid.UUID
	ObjectKey       string
	Now             time.Time
}

type FinalizeUploadInput struct {
	OwnerUserID     uuid.UUID
	UploadSessionID uuid.UUID
	ObjectKey       string
	MIMEType        string
	ContentHash     string
	SizeBytes       int64
	Now             time.Time
}

type FinalizeUploadResult struct {
	Session         UploadSession
	Node            Node
	Version         FileVersion
	WasAlreadyFinal bool
}
