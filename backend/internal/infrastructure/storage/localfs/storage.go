package localfs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"cloud/backend/internal/infrastructure/storage"
)

type Storage struct {
	root string
}

func New(root string) *Storage {
	return &Storage{root: root}
}

func (s *Storage) Put(ctx context.Context, key string, reader io.Reader, sizeBytes int64, opts storage.PutOptions) (storage.ObjectMeta, error) {
	_ = ctx
	_ = opts

	targetPath := s.pathForKey(key)
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return storage.ObjectMeta{}, fmt.Errorf("create storage directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(targetDir, ".tmp-*")
	if err != nil {
		return storage.ObjectMeta{}, fmt.Errorf("create temp storage file: %w", err)
	}

	written, copyErr := io.Copy(tmpFile, reader)
	closeErr := tmpFile.Close()
	if copyErr != nil {
		_ = os.Remove(tmpFile.Name())
		return storage.ObjectMeta{}, fmt.Errorf("write object body: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpFile.Name())
		return storage.ObjectMeta{}, fmt.Errorf("close temp storage file: %w", closeErr)
	}
	if sizeBytes >= 0 && written != sizeBytes {
		_ = os.Remove(tmpFile.Name())
		return storage.ObjectMeta{}, fmt.Errorf("written size mismatch: got %d want %d", written, sizeBytes)
	}

	if err := os.Rename(tmpFile.Name(), targetPath); err != nil {
		_ = os.Remove(tmpFile.Name())
		return storage.ObjectMeta{}, fmt.Errorf("finalize object write: %w", err)
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return storage.ObjectMeta{}, fmt.Errorf("stat stored object: %w", err)
	}

	return storage.ObjectMeta{
		Key:         key,
		SizeBytes:   info.Size(),
		ContentType: "",
		UpdatedAt:   info.ModTime().UTC(),
	}, nil
}

func (s *Storage) Open(ctx context.Context, key string) (io.ReadCloser, storage.ObjectMeta, error) {
	_ = ctx
	path := s.pathForKey(key)
	f, err := os.Open(path)
	if err != nil {
		return nil, storage.ObjectMeta{}, fmt.Errorf("open object: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, storage.ObjectMeta{}, fmt.Errorf("stat opened object: %w", err)
	}

	return f, storage.ObjectMeta{
		Key:       key,
		SizeBytes: info.Size(),
		UpdatedAt: info.ModTime().UTC(),
	}, nil
}

func (s *Storage) Delete(ctx context.Context, key string) error {
	_ = ctx
	path := s.pathForKey(key)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

func (s *Storage) Stat(ctx context.Context, key string) (storage.ObjectMeta, error) {
	_ = ctx
	path := s.pathForKey(key)
	info, err := os.Stat(path)
	if err != nil {
		return storage.ObjectMeta{}, fmt.Errorf("stat object: %w", err)
	}
	return storage.ObjectMeta{
		Key:       key,
		SizeBytes: info.Size(),
		UpdatedAt: info.ModTime().UTC(),
	}, nil
}

func (s *Storage) pathForKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	digest := hex.EncodeToString(sum[:])
	return filepath.Join(s.root, digest[:2], digest[2:4], digest)
}
