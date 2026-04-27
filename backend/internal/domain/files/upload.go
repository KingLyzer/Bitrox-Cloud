package files

import (
	"time"

	"github.com/google/uuid"
)

type UploadTargetType string

const (
	UploadTargetTypeNewFile    UploadTargetType = "new_file"
	UploadTargetTypeNewVersion UploadTargetType = "new_version"
)

type UploadSessionStatus string

const (
	UploadSessionStatusPending   UploadSessionStatus = "pending"
	UploadSessionStatusFinalizing UploadSessionStatus = "finalizing"
	UploadSessionStatusCompleted UploadSessionStatus = "completed"
	UploadSessionStatusAborted   UploadSessionStatus = "aborted"
)

type UploadSession struct {
	ID                 uuid.UUID
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
	Status             UploadSessionStatus
	UploadedBytes      int64
	FinalizedNodeID    *uuid.UUID
	FinalizedVersionID *uuid.UUID
	ObjectKey          *string
	MIMEType           *string
	ContentHash        *string
	ExpiresAt          time.Time
	CompletedAt        *time.Time
	CleanupStatus      string
	CleanupAttempts    int
	CleanupLastError   *string
	CleanupUpdatedAt   *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type UploadChunk struct {
	UploadSessionID uuid.UUID
	ChunkIndex      int
	SizeBytes       int
	ContentHash     string
	StagingKey      string
	CreatedAt       time.Time
}
