package files

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	domainfiles "cloud/backend/internal/domain/files"
	"cloud/backend/internal/infrastructure/storage"

	"github.com/google/uuid"
)

type fakeRepository struct {
	nodes                  map[uuid.UUID]domainfiles.Node
	listByParent           map[string][]domainfiles.Node
	uploadSessions         map[uuid.UUID]domainfiles.UploadSession
	uploadChunks           map[uuid.UUID][]domainfiles.UploadChunk
	isDescendantResult     bool
	beginFinalizeErr       error
	finalizeResult         domainfiles.FinalizeUploadResult
	finalizeErr            error
	finalizeCommitsBeforeError bool
	lastBeginFinalizeInput *domainfiles.BeginFinalizeUploadInput
	lastFinalizeInput      *domainfiles.FinalizeUploadInput
	lastCreateUploadInput  *domainfiles.CreateUploadSessionInput
	lastUpdateNodeInput    *domainfiles.UpdateNodeInput
	lastSoftDeleteNodeID   *uuid.UUID
}

func (f *fakeRepository) CreateFolder(_ context.Context, input domainfiles.CreateFolderInput) (domainfiles.Node, error) {
	id := uuid.New()
	node := domainfiles.Node{
		ID:          id,
		OwnerUserID: input.OwnerUserID,
		ParentID:    input.ParentID,
		Type:        domainfiles.NodeTypeFolder,
		Name:        input.Name,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if f.nodes == nil {
		f.nodes = map[uuid.UUID]domainfiles.Node{}
	}
	f.nodes[id] = node
	return node, nil
}

func (f *fakeRepository) GetNodeByID(_ context.Context, _ uuid.UUID, nodeID uuid.UUID) (domainfiles.Node, error) {
	node, ok := f.nodes[nodeID]
	if !ok {
		return domainfiles.Node{}, domainfiles.ErrNodeNotFound
	}
	return node, nil
}

func (f *fakeRepository) GetActiveNodeByID(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID) (domainfiles.Node, error) {
	node, err := f.GetNodeByID(ctx, ownerUserID, nodeID)
	if err != nil {
		return domainfiles.Node{}, err
	}
	if node.DeletedAt != nil {
		return domainfiles.Node{}, domainfiles.ErrNodeNotFound
	}
	return node, nil
}

func (f *fakeRepository) ListActiveNodesByParent(_ context.Context, _ uuid.UUID, parentID *uuid.UUID) ([]domainfiles.Node, error) {
	key := "root"
	if parentID != nil {
		key = parentID.String()
	}
	out := f.listByParent[key]
	return append([]domainfiles.Node(nil), out...), nil
}

func (f *fakeRepository) IsNodeInSubtree(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ uuid.UUID) (bool, error) {
	return f.isDescendantResult, nil
}

func (f *fakeRepository) UpdateNodeNameAndParent(_ context.Context, input domainfiles.UpdateNodeInput) (domainfiles.Node, error) {
	f.lastUpdateNodeInput = &input
	node, ok := f.nodes[input.NodeID]
	if !ok {
		return domainfiles.Node{}, domainfiles.ErrNodeNotFound
	}
	node.Name = input.Name
	node.ParentID = input.ParentID
	node.UpdatedAt = time.Now().UTC()
	f.nodes[input.NodeID] = node
	return node, nil
}

func (f *fakeRepository) SoftDeleteNodeTree(_ context.Context, _ uuid.UUID, nodeID uuid.UUID, _ time.Time) error {
	f.lastSoftDeleteNodeID = &nodeID
	node, ok := f.nodes[nodeID]
	if !ok {
		return domainfiles.ErrNodeNotFound
	}
	now := time.Now().UTC()
	node.DeletedAt = &now
	f.nodes[nodeID] = node
	return nil
}

func (f *fakeRepository) CreateUploadSession(_ context.Context, input domainfiles.CreateUploadSessionInput) (domainfiles.UploadSession, error) {
	f.lastCreateUploadInput = &input
	session := domainfiles.UploadSession{
		ID:                 uuid.New(),
		OwnerUserID:        input.OwnerUserID,
		TargetType:         input.TargetType,
		TargetNodeID:       input.TargetNodeID,
		ParentID:           input.ParentID,
		FileName:           input.FileName,
		ExpectedSizeBytes:  input.ExpectedSizeBytes,
		ChunkSizeBytes:     input.ChunkSizeBytes,
		ExpectedChunks:     input.ExpectedChunks,
		IdempotencyKey:     input.IdempotencyKey,
		RequestFingerprint: input.RequestFingerprint,
		Status:             domainfiles.UploadSessionStatusPending,
		ExpiresAt:          input.ExpiresAt,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}
	if f.uploadSessions == nil {
		f.uploadSessions = map[uuid.UUID]domainfiles.UploadSession{}
	}
	f.uploadSessions[session.ID] = session
	return session, nil
}

func (f *fakeRepository) CountActiveUploadSessions(_ context.Context, ownerUserID uuid.UUID) (int, error) {
	count := 0
	for _, session := range f.uploadSessions {
		if session.OwnerUserID == ownerUserID && (session.Status == domainfiles.UploadSessionStatusPending || session.Status == domainfiles.UploadSessionStatusFinalizing) {
			count++
		}
	}
	return count, nil
}

func (f *fakeRepository) GetUserStagedBytes(_ context.Context, ownerUserID uuid.UUID) (int64, error) {
	var staged int64
	for _, session := range f.uploadSessions {
		if session.OwnerUserID == ownerUserID && (session.Status == domainfiles.UploadSessionStatusPending || session.Status == domainfiles.UploadSessionStatusFinalizing) {
			staged += session.UploadedBytes
		}
	}
	return staged, nil
}

func (f *fakeRepository) GetUploadSessionByID(_ context.Context, _ uuid.UUID, uploadSessionID uuid.UUID) (domainfiles.UploadSession, error) {
	session, ok := f.uploadSessions[uploadSessionID]
	if !ok {
		return domainfiles.UploadSession{}, domainfiles.ErrUploadSessionNotFound
	}
	return session, nil
}

func (f *fakeRepository) ListUploadChunks(_ context.Context, uploadSessionID uuid.UUID) ([]domainfiles.UploadChunk, error) {
	out := f.uploadChunks[uploadSessionID]
	return append([]domainfiles.UploadChunk(nil), out...), nil
}

func (f *fakeRepository) ListExpiredActiveUploadSessions(_ context.Context, now time.Time, limit int) ([]domainfiles.UploadSession, error) {
	if limit <= 0 {
		limit = 100
	}
	out := make([]domainfiles.UploadSession, 0)
	for _, session := range f.uploadSessions {
		if (session.Status == domainfiles.UploadSessionStatusPending || session.Status == domainfiles.UploadSessionStatusFinalizing) &&
			!now.Before(session.ExpiresAt) {
			out = append(out, session)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (f *fakeRepository) UpsertUploadChunk(_ context.Context, input domainfiles.UpsertUploadChunkInput) (domainfiles.UploadSession, error) {
	session, ok := f.uploadSessions[input.UploadSessionID]
	if !ok {
		return domainfiles.UploadSession{}, domainfiles.ErrUploadSessionNotFound
	}
	if f.uploadChunks == nil {
		f.uploadChunks = map[uuid.UUID][]domainfiles.UploadChunk{}
	}
	updated := false
	chunks := f.uploadChunks[input.UploadSessionID]
	for i := 0; i < len(chunks); i++ {
		if chunks[i].ChunkIndex == input.ChunkIndex {
			if chunks[i].SizeBytes != input.SizeBytes || chunks[i].ContentHash != input.ContentHash {
				return domainfiles.UploadSession{}, domainfiles.ErrChunkConflict
			}
			chunks[i] = domainfiles.UploadChunk{
				UploadSessionID: input.UploadSessionID,
				ChunkIndex:      input.ChunkIndex,
				SizeBytes:       input.SizeBytes,
				ContentHash:     input.ContentHash,
				StagingKey:      input.StagingKey,
				CreatedAt:       time.Now().UTC(),
			}
			updated = true
			break
		}
	}
	delta := int64(0)
	if !updated {
		delta = int64(input.SizeBytes)
		chunks = append(chunks, domainfiles.UploadChunk{
			UploadSessionID: input.UploadSessionID,
			ChunkIndex:      input.ChunkIndex,
			SizeBytes:       input.SizeBytes,
			ContentHash:     input.ContentHash,
			StagingKey:      input.StagingKey,
			CreatedAt:       time.Now().UTC(),
		})
	}
	f.uploadChunks[input.UploadSessionID] = chunks

	if input.MaxUserStagedBytes > 0 && delta > 0 {
		staged, _ := f.GetUserStagedBytes(context.Background(), input.OwnerUserID)
		if staged+delta > input.MaxUserStagedBytes {
			return domainfiles.UploadSession{}, domainfiles.ErrStagedQuotaExceeded
		}
	}

	session.UploadedBytes += delta
	session.UpdatedAt = time.Now().UTC()
	f.uploadSessions[input.UploadSessionID] = session
	return session, nil
}

func (f *fakeRepository) AbortUploadSession(_ context.Context, _ uuid.UUID, uploadSessionID uuid.UUID, _ time.Time) (domainfiles.UploadSession, error) {
	session, ok := f.uploadSessions[uploadSessionID]
	if !ok {
		return domainfiles.UploadSession{}, domainfiles.ErrUploadSessionNotFound
	}
	session.Status = domainfiles.UploadSessionStatusAborted
	now := time.Now().UTC()
	session.CompletedAt = &now
	f.uploadSessions[uploadSessionID] = session
	return session, nil
}

func (f *fakeRepository) BeginFinalizeUploadSession(_ context.Context, input domainfiles.BeginFinalizeUploadInput) (domainfiles.UploadSession, error) {
	f.lastBeginFinalizeInput = &input
	if f.beginFinalizeErr != nil {
		return domainfiles.UploadSession{}, f.beginFinalizeErr
	}
	session, ok := f.uploadSessions[input.UploadSessionID]
	if !ok {
		return domainfiles.UploadSession{}, domainfiles.ErrUploadSessionNotFound
	}
	switch session.Status {
	case domainfiles.UploadSessionStatusCompleted:
		return session, nil
	case domainfiles.UploadSessionStatusAborted:
		return domainfiles.UploadSession{}, domainfiles.ErrUploadAborted
	case domainfiles.UploadSessionStatusFinalizing:
		if session.ObjectKey != nil && *session.ObjectKey == input.ObjectKey {
			return session, nil
		}
		return domainfiles.UploadSession{}, domainfiles.ErrFinalizeInProgress
	case domainfiles.UploadSessionStatusPending:
		if !input.Now.Before(session.ExpiresAt) {
			return domainfiles.UploadSession{}, domainfiles.ErrUploadExpired
		}
	}
	session.Status = domainfiles.UploadSessionStatusFinalizing
	session.ObjectKey = &input.ObjectKey
	f.uploadSessions[input.UploadSessionID] = session
	return session, nil
}

func (f *fakeRepository) FinalizeUploadSession(_ context.Context, input domainfiles.FinalizeUploadInput) (domainfiles.FinalizeUploadResult, error) {
	f.lastFinalizeInput = &input
	if f.finalizeCommitsBeforeError {
		if session, ok := f.uploadSessions[input.UploadSessionID]; ok {
			session.Status = domainfiles.UploadSessionStatusCompleted
			session.ObjectKey = &input.ObjectKey
			f.uploadSessions[input.UploadSessionID] = session
		}
	}
	if f.finalizeErr != nil {
		return domainfiles.FinalizeUploadResult{}, f.finalizeErr
	}
	if session, ok := f.uploadSessions[input.UploadSessionID]; ok {
		session.Status = domainfiles.UploadSessionStatusCompleted
		session.ObjectKey = &input.ObjectKey
		f.uploadSessions[input.UploadSessionID] = session
	}
	return f.finalizeResult, nil
}

func (f *fakeRepository) GetCompletedFinalizeResult(_ context.Context, _ uuid.UUID, uploadSessionID uuid.UUID) (domainfiles.FinalizeUploadResult, error) {
	session, ok := f.uploadSessions[uploadSessionID]
	if !ok {
		return domainfiles.FinalizeUploadResult{}, domainfiles.ErrUploadSessionNotFound
	}
	if session.Status != domainfiles.UploadSessionStatusCompleted {
		return domainfiles.FinalizeUploadResult{}, domainfiles.ErrInvalidOperation
	}
	result := f.finalizeResult
	result.Session = session
	result.WasAlreadyFinal = true
	return result, nil
}

func (f *fakeRepository) MarkUploadSessionCleanupInProgress(_ context.Context, ownerUserID, uploadSessionID uuid.UUID, now time.Time) (domainfiles.UploadSession, error) {
	session, ok := f.uploadSessions[uploadSessionID]
	if !ok || session.OwnerUserID != ownerUserID {
		return domainfiles.UploadSession{}, domainfiles.ErrUploadSessionNotFound
	}
	session.Status = domainfiles.UploadSessionStatusAborted
	session.CleanupStatus = "in_progress"
	session.CleanupAttempts++
	session.CleanupUpdatedAt = &now
	f.uploadSessions[uploadSessionID] = session
	return session, nil
}

func (f *fakeRepository) MarkUploadSessionCleanupResult(_ context.Context, ownerUserID, uploadSessionID uuid.UUID, now time.Time, cleanupErr error) error {
	session, ok := f.uploadSessions[uploadSessionID]
	if !ok || session.OwnerUserID != ownerUserID {
		return domainfiles.ErrUploadSessionNotFound
	}
	if cleanupErr != nil {
		session.CleanupStatus = "failed"
		msg := cleanupErr.Error()
		session.CleanupLastError = &msg
	} else {
		session.CleanupStatus = "done"
		session.CleanupLastError = nil
	}
	session.CleanupUpdatedAt = &now
	f.uploadSessions[uploadSessionID] = session
	return nil
}

type fakeObjectStorage struct {
	objects map[string][]byte
}

func (f *fakeObjectStorage) Put(_ context.Context, key string, reader io.Reader, sizeBytes int64, _ storage.PutOptions) (storage.ObjectMeta, error) {
	if f.objects == nil {
		f.objects = map[string][]byte{}
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return storage.ObjectMeta{}, err
	}
	if sizeBytes >= 0 && int64(len(data)) != sizeBytes {
		return storage.ObjectMeta{}, errors.New("size mismatch")
	}
	f.objects[key] = append([]byte(nil), data...)
	return storage.ObjectMeta{
		Key:       key,
		SizeBytes: int64(len(data)),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func (f *fakeObjectStorage) Open(_ context.Context, key string) (io.ReadCloser, storage.ObjectMeta, error) {
	data, ok := f.objects[key]
	if !ok {
		return nil, storage.ObjectMeta{}, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(data)), storage.ObjectMeta{
		Key:       key,
		SizeBytes: int64(len(data)),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func (f *fakeObjectStorage) Delete(_ context.Context, key string) error {
	delete(f.objects, key)
	return nil
}

func (f *fakeObjectStorage) Stat(_ context.Context, key string) (storage.ObjectMeta, error) {
	data, ok := f.objects[key]
	if !ok {
		return storage.ObjectMeta{}, errors.New("not found")
	}
	return storage.ObjectMeta{
		Key:       key,
		SizeBytes: int64(len(data)),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

type fixedClock struct {
	now time.Time
}

func (f fixedClock) Now() time.Time {
	return f.now
}

func TestMoveNodeRejectsMoveIntoOwnSubtree(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	rootFolderID := uuid.New()
	childFolderID := uuid.New()

	repo := &fakeRepository{
		nodes: map[uuid.UUID]domainfiles.Node{
			rootFolderID: {
				ID:          rootFolderID,
				OwnerUserID: ownerID,
				Type:        domainfiles.NodeTypeFolder,
				Name:        "root-folder",
			},
			childFolderID: {
				ID:          childFolderID,
				OwnerUserID: ownerID,
				Type:        domainfiles.NodeTypeFolder,
				Name:        "child-folder",
				ParentID:    &rootFolderID,
			},
		},
		isDescendantResult: true,
	}

	service := NewService(repo, &fakeObjectStorage{}, fixedClock{now: time.Now().UTC()}, Config{}, slog.Default())
	if _, err := service.MoveNode(ctx, ownerID, rootFolderID, &childFolderID); !errors.Is(err, domainfiles.ErrInvalidOperation) {
		t.Fatalf("expected invalid operation for subtree move, got %v", err)
	}
}

func TestUploadChunkRejectsUnexpectedChunkSize(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := uuid.New()
	sessionID := uuid.New()

	repo := &fakeRepository{
		uploadSessions: map[uuid.UUID]domainfiles.UploadSession{
			sessionID: {
				ID:                sessionID,
				OwnerUserID:       ownerID,
				TargetType:        domainfiles.UploadTargetTypeNewFile,
				FileName:          "doc.txt",
				ExpectedSizeBytes: 10,
				ChunkSizeBytes:    6,
				ExpectedChunks:    2,
				Status:            domainfiles.UploadSessionStatusPending,
				ExpiresAt:         now.Add(30 * time.Minute),
			},
		},
	}

	service := NewService(repo, &fakeObjectStorage{}, fixedClock{now: now}, Config{MaxChunkBytes: 1024}, slog.Default())
	_, err := service.UploadChunk(ctx, UploadChunkInput{
		OwnerUserID:     ownerID,
		UploadSessionID: sessionID,
		ChunkIndex:      0,
		ContentHash:     strings.Repeat("a", 64),
		Body:            bytes.NewReader([]byte("12345")),
	})
	if !errors.Is(err, domainfiles.ErrInvalidOperation) {
		t.Fatalf("expected invalid operation on wrong chunk size, got %v", err)
	}
}

func TestFinalizeUploadSessionSuccess(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := uuid.New()
	sessionID := uuid.New()
	nodeID := uuid.New()
	versionID := uuid.New()
	chunk0Key := buildStagingObjectKey(ownerID, sessionID, 0, hashBytesHex([]byte("hello")))
	chunk1Key := buildStagingObjectKey(ownerID, sessionID, 1, hashBytesHex([]byte("world")))

	repo := &fakeRepository{
		uploadSessions: map[uuid.UUID]domainfiles.UploadSession{
			sessionID: {
				ID:                sessionID,
				OwnerUserID:       ownerID,
				TargetType:        domainfiles.UploadTargetTypeNewFile,
				FileName:          "hello.txt",
				ExpectedSizeBytes: 10,
				ChunkSizeBytes:    5,
				ExpectedChunks:    2,
				Status:            domainfiles.UploadSessionStatusPending,
				ExpiresAt:         now.Add(time.Hour),
			},
		},
		uploadChunks: map[uuid.UUID][]domainfiles.UploadChunk{
			sessionID: {
				{
					UploadSessionID: sessionID,
					ChunkIndex:      0,
					SizeBytes:       5,
					StagingKey:      chunk0Key,
				},
				{
					UploadSessionID: sessionID,
					ChunkIndex:      1,
					SizeBytes:       5,
					StagingKey:      chunk1Key,
				},
			},
		},
		finalizeResult: domainfiles.FinalizeUploadResult{
			Session: domainfiles.UploadSession{
				ID:     sessionID,
				Status: domainfiles.UploadSessionStatusCompleted,
			},
			Node: domainfiles.Node{
				ID:   nodeID,
				Type: domainfiles.NodeTypeFile,
				Name: "hello.txt",
			},
			Version: domainfiles.FileVersion{
				ID:        versionID,
				VersionNo: 1,
			},
		},
	}
	objStore := &fakeObjectStorage{
		objects: map[string][]byte{
			chunk0Key: []byte("hello"),
			chunk1Key: []byte("world"),
		},
	}

	service := NewService(repo, objStore, fixedClock{now: now}, Config{}, slog.Default())
	result, err := service.FinalizeUploadSession(ctx, ownerID, sessionID)
	if err != nil {
		t.Fatalf("expected finalize success, got %v", err)
	}
	if result.Node.ID != nodeID {
		t.Fatalf("unexpected node id: got %s want %s", result.Node.ID, nodeID)
	}
	if repo.lastFinalizeInput == nil {
		t.Fatalf("expected finalize repository call")
	}

	expectedPayload := []byte("helloworld")
	expectedDigest := sha256.Sum256(expectedPayload)
	expectedHash := hex.EncodeToString(expectedDigest[:])
	if repo.lastFinalizeInput.ContentHash != expectedHash {
		t.Fatalf("unexpected content hash: got %s want %s", repo.lastFinalizeInput.ContentHash, expectedHash)
	}
	if repo.lastFinalizeInput.SizeBytes != int64(len(expectedPayload)) {
		t.Fatalf("unexpected final size: got %d want %d", repo.lastFinalizeInput.SizeBytes, len(expectedPayload))
	}

	finalKey := repo.lastFinalizeInput.ObjectKey
	if _, ok := objStore.objects[finalKey]; !ok {
		t.Fatalf("expected final object to be stored")
	}
	if _, ok := objStore.objects[chunk0Key]; ok {
		t.Fatalf("expected chunk 0 to be cleaned after finalize")
	}
	if _, ok := objStore.objects[chunk1Key]; ok {
		t.Fatalf("expected chunk 1 to be cleaned after finalize")
	}
}

func TestFinalizeUploadSessionDeletesFinalObjectOnFinalizeFailure(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := uuid.New()
	sessionID := uuid.New()
	chunk0Key := buildStagingObjectKey(ownerID, sessionID, 0, hashBytesHex([]byte("hello")))

	repo := &fakeRepository{
		uploadSessions: map[uuid.UUID]domainfiles.UploadSession{
			sessionID: {
				ID:                sessionID,
				OwnerUserID:       ownerID,
				TargetType:        domainfiles.UploadTargetTypeNewFile,
				FileName:          "a.txt",
				ExpectedSizeBytes: 5,
				ChunkSizeBytes:    5,
				ExpectedChunks:    1,
				Status:            domainfiles.UploadSessionStatusPending,
				ExpiresAt:         now.Add(time.Hour),
			},
		},
		uploadChunks: map[uuid.UUID][]domainfiles.UploadChunk{
			sessionID: {
				{
					UploadSessionID: sessionID,
					ChunkIndex:      0,
					SizeBytes:       5,
					StagingKey:      chunk0Key,
				},
			},
		},
		finalizeErr: domainfiles.ErrQuotaExceeded,
	}
	objStore := &fakeObjectStorage{
		objects: map[string][]byte{
			chunk0Key: []byte("hello"),
		},
	}

	service := NewService(repo, objStore, fixedClock{now: now}, Config{}, slog.Default())
	_, err := service.FinalizeUploadSession(ctx, ownerID, sessionID)
	if !errors.Is(err, domainfiles.ErrQuotaExceeded) {
		t.Fatalf("expected quota exceeded error, got %v", err)
	}

	prefix := "objects/" + ownerID.String() + "/" + sessionID.String() + "/"
	for key := range objStore.objects {
		if strings.HasPrefix(key, prefix) {
			t.Fatalf("expected final object cleanup on finalize failure, found key %s", key)
		}
	}
}

func TestCreateUploadSessionComputesExpectedChunks(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := uuid.New()

	repo := &fakeRepository{}
	service := NewService(repo, &fakeObjectStorage{}, fixedClock{now: now}, Config{UploadSessionTTL: time.Hour}, slog.Default())

	session, err := service.CreateUploadSession(ctx, CreateUploadSessionInput{
		OwnerUserID:       ownerID,
		TargetType:        domainfiles.UploadTargetTypeNewFile,
		FileName:          "video.mp4",
		ExpectedSizeBytes: 10_000_000,
		ChunkSizeBytes:    4_000_000,
	})
	if err != nil {
		t.Fatalf("expected create upload session success, got %v", err)
	}
	if session.ExpectedChunks != 3 {
		t.Fatalf("unexpected expected chunk count: got %d want 3", session.ExpectedChunks)
	}
	if repo.lastCreateUploadInput == nil || repo.lastCreateUploadInput.RequestFingerprint == nil {
		t.Fatalf("expected request fingerprint to be set")
	}
}

func TestUploadChunkRequiresHash(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := uuid.New()
	sessionID := uuid.New()

	repo := &fakeRepository{
		uploadSessions: map[uuid.UUID]domainfiles.UploadSession{
			sessionID: {
				ID:                sessionID,
				OwnerUserID:       ownerID,
				TargetType:        domainfiles.UploadTargetTypeNewFile,
				FileName:          "doc.txt",
				ExpectedSizeBytes: 5,
				ChunkSizeBytes:    5,
				ExpectedChunks:    1,
				Status:            domainfiles.UploadSessionStatusPending,
				ExpiresAt:         now.Add(time.Hour),
			},
		},
	}
	service := NewService(repo, &fakeObjectStorage{}, fixedClock{now: now}, Config{MaxChunkBytes: 1024}, slog.Default())

	_, err := service.UploadChunk(ctx, UploadChunkInput{
		OwnerUserID:     ownerID,
		UploadSessionID: sessionID,
		ChunkIndex:      0,
		ContentHash:     "",
		Body:            bytes.NewReader([]byte("hello")),
	})
	if !errors.Is(err, domainfiles.ErrChunkHashRequired) {
		t.Fatalf("expected chunk hash required, got %v", err)
	}
}

func TestUploadChunkConflictDoesNotDeleteExistingStagedChunk(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := uuid.New()
	sessionID := uuid.New()

	repo := &fakeRepository{
		uploadSessions: map[uuid.UUID]domainfiles.UploadSession{
			sessionID: {
				ID:                sessionID,
				OwnerUserID:       ownerID,
				TargetType:        domainfiles.UploadTargetTypeNewFile,
				FileName:          "doc.txt",
				ExpectedSizeBytes: 5,
				ChunkSizeBytes:    5,
				ExpectedChunks:    1,
				Status:            domainfiles.UploadSessionStatusPending,
				ExpiresAt:         now.Add(time.Hour),
			},
		},
	}
	objStore := &fakeObjectStorage{objects: map[string][]byte{}}
	service := NewService(repo, objStore, fixedClock{now: now}, Config{MaxChunkBytes: 1024}, slog.Default())

	firstPayload := []byte("hello")
	firstHash := hashBytesHex(firstPayload)
	if _, err := service.UploadChunk(ctx, UploadChunkInput{
		OwnerUserID:     ownerID,
		UploadSessionID: sessionID,
		ChunkIndex:      0,
		ContentHash:     firstHash,
		Body:            bytes.NewReader(firstPayload),
	}); err != nil {
		t.Fatalf("first upload chunk should succeed: %v", err)
	}

	secondPayload := []byte("world")
	secondHash := hashBytesHex(secondPayload)
	_, err := service.UploadChunk(ctx, UploadChunkInput{
		OwnerUserID:     ownerID,
		UploadSessionID: sessionID,
		ChunkIndex:      0,
		ContentHash:     secondHash,
		Body:            bytes.NewReader(secondPayload),
	})
	if !errors.Is(err, domainfiles.ErrChunkConflict) {
		t.Fatalf("expected chunk conflict, got %v", err)
	}

	firstKey := buildStagingObjectKey(ownerID, sessionID, 0, firstHash)
	secondKey := buildStagingObjectKey(ownerID, sessionID, 0, secondHash)
	if _, ok := objStore.objects[firstKey]; !ok {
		t.Fatalf("expected original staged chunk to remain after conflict")
	}
	if _, ok := objStore.objects[secondKey]; ok {
		t.Fatalf("expected conflicting staged chunk to be cleaned")
	}
}

func TestFinalizeUploadSessionResolvesCommitUnknownWhenSessionCompletedWithMatchingObjectKey(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := uuid.New()
	sessionID := uuid.New()
	nodeID := uuid.New()
	versionID := uuid.New()
	chunk0Key := buildStagingObjectKey(ownerID, sessionID, 0, hashBytesHex([]byte("hello")))
	chunk1Key := buildStagingObjectKey(ownerID, sessionID, 1, hashBytesHex([]byte("world")))

	repo := &fakeRepository{
		uploadSessions: map[uuid.UUID]domainfiles.UploadSession{
			sessionID: {
				ID:                sessionID,
				OwnerUserID:       ownerID,
				TargetType:        domainfiles.UploadTargetTypeNewFile,
				FileName:          "hello.txt",
				ExpectedSizeBytes: 10,
				ChunkSizeBytes:    5,
				ExpectedChunks:    2,
				Status:            domainfiles.UploadSessionStatusPending,
				ExpiresAt:         now.Add(time.Hour),
			},
		},
		uploadChunks: map[uuid.UUID][]domainfiles.UploadChunk{
			sessionID: {
				{UploadSessionID: sessionID, ChunkIndex: 0, SizeBytes: 5, StagingKey: chunk0Key},
				{UploadSessionID: sessionID, ChunkIndex: 1, SizeBytes: 5, StagingKey: chunk1Key},
			},
		},
		finalizeCommitsBeforeError: true,
		finalizeErr: &domainfiles.CommitUnknownError{
			Operation: "finalize_upload_session_commit",
			Cause:     errors.New("ambiguous commit"),
		},
		finalizeResult: domainfiles.FinalizeUploadResult{
			Session: domainfiles.UploadSession{ID: sessionID, Status: domainfiles.UploadSessionStatusCompleted},
			Node:    domainfiles.Node{ID: nodeID, Type: domainfiles.NodeTypeFile, Name: "hello.txt"},
			Version: domainfiles.FileVersion{ID: versionID, VersionNo: 1},
		},
	}
	objStore := &fakeObjectStorage{
		objects: map[string][]byte{
			chunk0Key: []byte("hello"),
			chunk1Key: []byte("world"),
		},
	}

	service := NewService(repo, objStore, fixedClock{now: now}, Config{}, slog.Default())
	result, err := service.FinalizeUploadSession(ctx, ownerID, sessionID)
	if err != nil {
		t.Fatalf("expected commit unknown to resolve to success, got %v", err)
	}
	if result.WasAlreadyFinal != true {
		t.Fatalf("expected already finalized=true after commit-unknown recovery")
	}
}

func TestDownloadNodeReturnsFileObject(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	fileID := uuid.New()
	storageKey := "objects/test/file.txt"
	mimeType := "text/plain; charset=utf-8"

	repo := &fakeRepository{
		nodes: map[uuid.UUID]domainfiles.Node{
			fileID: {
				ID:          fileID,
				OwnerUserID: ownerID,
				Type:        domainfiles.NodeTypeFile,
				Name:        "notes.txt",
				StorageKey:  &storageKey,
				MIMEType:    &mimeType,
			},
		},
	}
	store := &fakeObjectStorage{
		objects: map[string][]byte{
			storageKey: []byte("hello-download"),
		},
	}

	service := NewService(repo, store, fixedClock{now: time.Now().UTC()}, Config{}, slog.Default())
	result, err := service.DownloadNode(ctx, ownerID, fileID)
	if err != nil {
		t.Fatalf("expected file download success, got %v", err)
	}
	defer result.Reader.Close()

	payload, err := io.ReadAll(result.Reader)
	if err != nil {
		t.Fatalf("read download payload: %v", err)
	}
	if string(payload) != "hello-download" {
		t.Fatalf("unexpected payload: %q", string(payload))
	}
	if result.FileName != "notes.txt" {
		t.Fatalf("unexpected file name: %q", result.FileName)
	}
	if result.MIMEType != mimeType {
		t.Fatalf("unexpected mime type: %q", result.MIMEType)
	}
	if result.SizeBytes != int64(len(payload)) {
		t.Fatalf("unexpected size bytes: %d", result.SizeBytes)
	}
}

func TestDownloadNodeReturnsFolderZip(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	rootID := uuid.New()
	nestedFolderID := uuid.New()
	fileRootID := uuid.New()
	fileNestedID := uuid.New()
	rootFileStorageKey := "objects/root-file.txt"
	nestedFileStorageKey := "objects/nested-file.txt"

	repo := &fakeRepository{
		nodes: map[uuid.UUID]domainfiles.Node{
			rootID: {
				ID:          rootID,
				OwnerUserID: ownerID,
				Type:        domainfiles.NodeTypeFolder,
				Name:        "Root",
			},
		},
		listByParent: map[string][]domainfiles.Node{
			rootID.String(): {
				{
					ID:          fileRootID,
					OwnerUserID: ownerID,
					ParentID:    &rootID,
					Type:        domainfiles.NodeTypeFile,
					Name:        "readme.txt",
					StorageKey:  &rootFileStorageKey,
				},
				{
					ID:          nestedFolderID,
					OwnerUserID: ownerID,
					ParentID:    &rootID,
					Type:        domainfiles.NodeTypeFolder,
					Name:        "Nested",
				},
			},
			nestedFolderID.String(): {
				{
					ID:          fileNestedID,
					OwnerUserID: ownerID,
					ParentID:    &nestedFolderID,
					Type:        domainfiles.NodeTypeFile,
					Name:        "info.txt",
					StorageKey:  &nestedFileStorageKey,
				},
			},
		},
	}
	store := &fakeObjectStorage{
		objects: map[string][]byte{
			rootFileStorageKey:   []byte("root-content"),
			nestedFileStorageKey: []byte("nested-content"),
		},
	}

	service := NewService(repo, store, fixedClock{now: time.Now().UTC()}, Config{}, slog.Default())
	result, err := service.DownloadNode(ctx, ownerID, rootID)
	if err != nil {
		t.Fatalf("expected folder download success, got %v", err)
	}
	defer result.Reader.Close()

	if result.MIMEType != "application/zip" {
		t.Fatalf("expected zip mime type, got %q", result.MIMEType)
	}
	if result.FileName != "Root.zip" {
		t.Fatalf("unexpected folder archive name: %q", result.FileName)
	}

	zipPayload, err := io.ReadAll(result.Reader)
	if err != nil {
		t.Fatalf("read zip payload: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(zipPayload), int64(len(zipPayload)))
	if err != nil {
		t.Fatalf("open zip payload: %v", err)
	}

	names := map[string]struct{}{}
	for _, file := range reader.File {
		names[file.Name] = struct{}{}
	}
	if _, ok := names["Root/readme.txt"]; !ok {
		t.Fatalf("expected Root/readme.txt in archive, names=%v", names)
	}
	if _, ok := names["Root/Nested/info.txt"]; !ok {
		t.Fatalf("expected Root/Nested/info.txt in archive, names=%v", names)
	}
}
