package files

import "errors"

var (
	ErrNodeNotFound         = errors.New("node not found")
	ErrUploadSessionNotFound = errors.New("upload session not found")
	ErrNameConflict         = errors.New("name conflict")
	ErrInvalidOperation     = errors.New("invalid operation")
	ErrInvalidName          = errors.New("invalid name")
	ErrUploadExpired        = errors.New("upload session expired")
	ErrUploadIncomplete     = errors.New("upload incomplete")
	ErrUploadAborted        = errors.New("upload aborted")
	ErrFinalizeInProgress   = errors.New("finalize in progress")
	ErrChunkConflict        = errors.New("upload chunk conflict")
	ErrChunkHashRequired    = errors.New("chunk hash required")
	ErrChunkHashMismatch    = errors.New("chunk hash mismatch")
	ErrStagedQuotaExceeded  = errors.New("staged quota exceeded")
	ErrTooManyActiveUploads = errors.New("too many active upload sessions")
	ErrQuotaExceeded        = errors.New("quota exceeded")
	ErrIdempotencyConflict  = errors.New("idempotency conflict")
)

type CommitUnknownError struct {
	Operation string
	Cause     error
}

func (e *CommitUnknownError) Error() string {
	if e == nil {
		return "commit unknown"
	}
	if e.Operation == "" {
		if e.Cause != nil {
			return "commit unknown: " + e.Cause.Error()
		}
		return "commit unknown"
	}
	if e.Cause != nil {
		return e.Operation + ": commit unknown: " + e.Cause.Error()
	}
	return e.Operation + ": commit unknown"
}

func (e *CommitUnknownError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
