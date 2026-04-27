package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	appfiles "cloud/backend/internal/application/files"
	domainfiles "cloud/backend/internal/domain/files"
	infraPostgres "cloud/backend/internal/infrastructure/postgres"
	"cloud/backend/internal/infrastructure/storage/localfs"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var migrateOnce sync.Once

func setupIntegrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL integration tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(pool.Close)

	migrateOnce.Do(func() {
		applyMigrations(t, pool)
	})

	resetDatabase(t, pool)
	return pool
}

func applyMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	pattern := filepath.Clean("../../migrations/*.up.sql")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob migration files: %v", err)
	}
	sort.Strings(files)

	ctx := context.Background()
	for _, migrationFile := range files {
		sqlBytes, err := os.ReadFile(migrationFile)
		if err != nil {
			t.Fatalf("read migration %s: %v", migrationFile, err)
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			t.Fatalf("apply migration %s: %v", migrationFile, err)
		}
	}
}

func resetDatabase(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	const query = `
TRUNCATE TABLE
    upload_chunks,
    upload_sessions,
    file_versions,
    nodes,
    app_passwords,
    sessions,
    users
RESTART IDENTITY CASCADE
`
	if _, err := pool.Exec(ctx, query); err != nil {
		t.Fatalf("reset integration database: %v", err)
	}
}

func createUser(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var userID uuid.UUID
	const query = `
INSERT INTO users (email, display_name, role, password_hash, is_active)
VALUES ($1, 'Integration User', 'owner', 'not-used', TRUE)
RETURNING id
`
	if err := pool.QueryRow(ctx, query, email).Scan(&userID); err != nil {
		t.Fatalf("create test user %s: %v", email, err)
	}
	return userID
}

func TestIntegrationChunkConflict(t *testing.T) {
	pool := setupIntegrationDB(t)
	repo := infraPostgres.NewFilesRepository(pool)
	userID := createUser(t, pool, "chunk-conflict@example.com")
	ctx := context.Background()

	idempotency := "chunk-conflict-session"
	fingerprint := "fp-chunk-conflict"
	session, err := repo.CreateUploadSession(ctx, domainfiles.CreateUploadSessionInput{
		OwnerUserID:        userID,
		TargetType:         domainfiles.UploadTargetTypeNewFile,
		FileName:           "doc.txt",
		ExpectedSizeBytes:  10,
		ChunkSizeBytes:     10,
		ExpectedChunks:     1,
		IdempotencyKey:     &idempotency,
		RequestFingerprint: &fingerprint,
		ExpiresAt:          time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create upload session: %v", err)
	}

	if _, err := repo.UpsertUploadChunk(ctx, domainfiles.UpsertUploadChunkInput{
		OwnerUserID:        userID,
		UploadSessionID:    session.ID,
		ChunkIndex:         0,
		SizeBytes:          10,
		ContentHash:        strings.Repeat("a", 64),
		StagingKey:         "uploads/a",
		MaxUserStagedBytes: 1024,
		Now:                time.Now().UTC(),
	}); err != nil {
		t.Fatalf("first chunk upsert should succeed: %v", err)
	}

	_, err = repo.UpsertUploadChunk(ctx, domainfiles.UpsertUploadChunkInput{
		OwnerUserID:        userID,
		UploadSessionID:    session.ID,
		ChunkIndex:         0,
		SizeBytes:          10,
		ContentHash:        strings.Repeat("b", 64),
		StagingKey:         "uploads/b",
		MaxUserStagedBytes: 1024,
		Now:                time.Now().UTC(),
	})
	if !errors.Is(err, domainfiles.ErrChunkConflict) {
		t.Fatalf("expected chunk conflict, got %v", err)
	}
}

func TestIntegrationConcurrentBeginFinalizeCAS(t *testing.T) {
	pool := setupIntegrationDB(t)
	repo := infraPostgres.NewFilesRepository(pool)
	userID := createUser(t, pool, "begin-finalize@example.com")
	ctx := context.Background()

	session, err := repo.CreateUploadSession(ctx, domainfiles.CreateUploadSessionInput{
		OwnerUserID:       userID,
		TargetType:        domainfiles.UploadTargetTypeNewFile,
		FileName:          "doc.txt",
		ExpectedSizeBytes: 5,
		ChunkSizeBytes:    5,
		ExpectedChunks:    1,
		ExpiresAt:         time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create upload session: %v", err)
	}
	if _, err := repo.UpsertUploadChunk(ctx, domainfiles.UpsertUploadChunkInput{
		OwnerUserID:        userID,
		UploadSessionID:    session.ID,
		ChunkIndex:         0,
		SizeBytes:          5,
		ContentHash:        strings.Repeat("c", 64),
		StagingKey:         "uploads/chunk-finalize",
		MaxUserStagedBytes: 1024,
		Now:                time.Now().UTC(),
	}); err != nil {
		t.Fatalf("insert finalize chunk: %v", err)
	}

	var wg sync.WaitGroup
	type out struct{ err error }
	results := make(chan out, 2)

	begin := func(key string) {
		defer wg.Done()
		_, err := repo.BeginFinalizeUploadSession(ctx, domainfiles.BeginFinalizeUploadInput{
			OwnerUserID:     userID,
			UploadSessionID: session.ID,
			ObjectKey:       key,
			Now:             time.Now().UTC(),
		})
		results <- out{err: err}
	}

	wg.Add(2)
	go begin("objects/a")
	go begin("objects/b")
	wg.Wait()
	close(results)

	var (
		successCount int
		conflictCount int
	)
	for result := range results {
		if result.err == nil {
			successCount++
			continue
		}
		if errors.Is(result.err, domainfiles.ErrFinalizeInProgress) {
			conflictCount++
		}
	}
	if successCount != 1 || conflictCount != 1 {
		t.Fatalf("expected one success and one finalize-in-progress conflict, got success=%d conflict=%d", successCount, conflictCount)
	}
}

func TestIntegrationIdempotencyReplay(t *testing.T) {
	pool := setupIntegrationDB(t)
	repo := infraPostgres.NewFilesRepository(pool)
	userID := createUser(t, pool, "idempotency@example.com")
	ctx := context.Background()

	key := "idempotency-replay-key"
	fp := "same-fingerprint"
	first, err := repo.CreateUploadSession(ctx, domainfiles.CreateUploadSessionInput{
		OwnerUserID:        userID,
		TargetType:         domainfiles.UploadTargetTypeNewFile,
		FileName:           "a.txt",
		ExpectedSizeBytes:  10,
		ChunkSizeBytes:     10,
		ExpectedChunks:     1,
		IdempotencyKey:     &key,
		RequestFingerprint: &fp,
		ExpiresAt:          time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("first upload session create failed: %v", err)
	}

	second, err := repo.CreateUploadSession(ctx, domainfiles.CreateUploadSessionInput{
		OwnerUserID:        userID,
		TargetType:         domainfiles.UploadTargetTypeNewFile,
		FileName:           "a.txt",
		ExpectedSizeBytes:  10,
		ChunkSizeBytes:     10,
		ExpectedChunks:     1,
		IdempotencyKey:     &key,
		RequestFingerprint: &fp,
		ExpiresAt:          time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("second upload session create failed: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected idempotent replay to return same upload session id, got %s and %s", first.ID, second.ID)
	}
}

func TestIntegrationNodeParentInvariants(t *testing.T) {
	pool := setupIntegrationDB(t)
	repo := infraPostgres.NewFilesRepository(pool)
	ctx := context.Background()
	userA := createUser(t, pool, "parent-a@example.com")
	userB := createUser(t, pool, "parent-b@example.com")

	folderA, err := repo.CreateFolder(ctx, domainfiles.CreateFolderInput{
		OwnerUserID: userA,
		Name:        "user-a-folder",
	})
	if err != nil {
		t.Fatalf("create user A folder: %v", err)
	}

	if _, err := repo.CreateFolder(ctx, domainfiles.CreateFolderInput{
		OwnerUserID: userB,
		ParentID:    &folderA.ID,
		Name:        "cross-owner-folder",
	}); err == nil {
		t.Fatalf("expected cross-owner parent creation to fail due DB invariant")
	}

	folderB, err := repo.CreateFolder(ctx, domainfiles.CreateFolderInput{
		OwnerUserID: userB,
		Name:        "self-parent-folder",
	})
	if err != nil {
		t.Fatalf("create user B folder: %v", err)
	}

	if _, err := repo.UpdateNodeNameAndParent(ctx, domainfiles.UpdateNodeInput{
		OwnerUserID: userB,
		NodeID:      folderB.ID,
		Name:        folderB.Name,
		ParentID:    &folderB.ID,
	}); err == nil {
		t.Fatalf("expected self-parent update to fail due DB invariant")
	}

	var fileNodeID uuid.UUID
	const createFileNode = `
INSERT INTO nodes (owner_user_id, type, name, size_bytes, mime_type, content_hash, storage_key, current_version_no)
VALUES ($1, 'file', 'plain-file.txt', 4, 'text/plain', 'abc', 'objects/test/plain-file', 1)
RETURNING id
`
	if err := pool.QueryRow(ctx, createFileNode, userA).Scan(&fileNodeID); err != nil {
		t.Fatalf("create file node for invariant test: %v", err)
	}

	if _, err := repo.CreateFolder(ctx, domainfiles.CreateFolderInput{
		OwnerUserID: userA,
		ParentID:    &fileNodeID,
		Name:        "child-under-file-should-fail",
	}); err == nil {
		t.Fatalf("expected parent-must-be-folder invariant failure")
	}
}

func TestIntegrationStagedQuotaRace(t *testing.T) {
	pool := setupIntegrationDB(t)
	repo := infraPostgres.NewFilesRepository(pool)
	userID := createUser(t, pool, "staged-race@example.com")
	ctx := context.Background()

	makeSession := func(name string) uuid.UUID {
		session, err := repo.CreateUploadSession(ctx, domainfiles.CreateUploadSessionInput{
			OwnerUserID:       userID,
			TargetType:        domainfiles.UploadTargetTypeNewFile,
			FileName:          name,
			ExpectedSizeBytes: 10,
			ChunkSizeBytes:    10,
			ExpectedChunks:    1,
			ExpiresAt:         time.Now().UTC().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("create upload session %s: %v", name, err)
		}
		return session.ID
	}

	sessionA := makeSession("a.bin")
	sessionB := makeSession("b.bin")

	var wg sync.WaitGroup
	results := make(chan error, 2)
	runChunk := func(sessionID uuid.UUID, key string) {
		defer wg.Done()
		_, err := repo.UpsertUploadChunk(ctx, domainfiles.UpsertUploadChunkInput{
			OwnerUserID:        userID,
			UploadSessionID:    sessionID,
			ChunkIndex:         0,
			SizeBytes:          10,
			ContentHash:        strings.Repeat("d", 64),
			StagingKey:         key,
			MaxUserStagedBytes: 10,
			Now:                time.Now().UTC(),
		})
		results <- err
	}

	wg.Add(2)
	go runChunk(sessionA, "uploads/a")
	go runChunk(sessionB, "uploads/b")
	wg.Wait()
	close(results)

	var (
		success int
		quotaErr int
	)
	for err := range results {
		if err == nil {
			success++
			continue
		}
		if errors.Is(err, domainfiles.ErrStagedQuotaExceeded) {
			quotaErr++
		}
	}
	if success != 1 || quotaErr != 1 {
		t.Fatalf("expected one success and one staged quota failure, got success=%d quotaErr=%d", success, quotaErr)
	}
}

type commitUnknownFinalizeRepo struct {
	*infraPostgres.FilesRepository
	injected bool
}

func (r *commitUnknownFinalizeRepo) FinalizeUploadSession(ctx context.Context, input domainfiles.FinalizeUploadInput) (domainfiles.FinalizeUploadResult, error) {
	result, err := r.FilesRepository.FinalizeUploadSession(ctx, input)
	if err != nil {
		return domainfiles.FinalizeUploadResult{}, err
	}
	if !r.injected {
		r.injected = true
		return domainfiles.FinalizeUploadResult{}, &domainfiles.CommitUnknownError{
			Operation: "finalize_upload_session_commit",
			Cause:     errors.New("simulated ambiguous commit"),
		}
	}
	return result, nil
}

func TestIntegrationCommitUnknownRecovery(t *testing.T) {
	pool := setupIntegrationDB(t)
	baseRepo := infraPostgres.NewFilesRepository(pool)
	repo := &commitUnknownFinalizeRepo{FilesRepository: baseRepo}
	userID := createUser(t, pool, "commit-unknown@example.com")
	ctx := context.Background()

	storageRoot := t.TempDir()
	service := appfiles.NewService(repo, localfs.New(storageRoot), appfiles.RealClock{}, appfiles.Config{
		UploadSessionTTL:        time.Hour,
		MaxChunkBytes:           1024 * 1024,
		MaxFileBytes:            1024 * 1024,
		MaxActiveUploadSessions: 10,
		MaxUserStagedBytes:      1024 * 1024,
	}, slog.Default())

	session, err := service.CreateUploadSession(ctx, appfiles.CreateUploadSessionInput{
		OwnerUserID:       userID,
		TargetType:        domainfiles.UploadTargetTypeNewFile,
		FileName:          "hello.txt",
		ExpectedSizeBytes: 5,
		ChunkSizeBytes:    5,
		IdempotencyKey:    nil,
	})
	if err != nil {
		t.Fatalf("create upload session via service: %v", err)
	}

	payload := []byte("hello")
	hash := sha256.Sum256(payload)
	hashHex := hex.EncodeToString(hash[:])
	if _, err := service.UploadChunk(ctx, appfiles.UploadChunkInput{
		OwnerUserID:     userID,
		UploadSessionID: session.ID,
		ChunkIndex:      0,
		ContentHash:     hashHex,
		Body:            bytes.NewReader(payload),
	}); err != nil {
		t.Fatalf("upload chunk via service: %v", err)
	}

	result, err := service.FinalizeUploadSession(ctx, userID, session.ID)
	if err != nil {
		t.Fatalf("finalize should recover from commit-unknown: %v", err)
	}
	if result.Session.Status != domainfiles.UploadSessionStatusCompleted {
		t.Fatalf("expected completed upload session after commit-unknown recovery, got %s", result.Session.Status)
	}
}
