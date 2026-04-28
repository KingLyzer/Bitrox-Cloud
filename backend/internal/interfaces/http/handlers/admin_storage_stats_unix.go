//go:build !windows

package handlers

import (
	"fmt"
	"path/filepath"
	"syscall"
)

func readStorageStats(storageRoot string) (AdminStorageStats, error) {
	root := storageRoot
	if root == "" {
		root = "."
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return AdminStorageStats{}, fmt.Errorf("resolve storage root: %w", err)
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(absoluteRoot, &stat); err != nil {
		return AdminStorageStats{}, fmt.Errorf("statfs %s: %w", absoluteRoot, err)
	}

	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bavail) * int64(stat.Bsize)
	used := total - free
	if used < 0 {
		used = 0
	}

	return AdminStorageStats{
		TotalBytes: total,
		UsedBytes:  used,
		FreeBytes:  free,
	}, nil
}
