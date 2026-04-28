//go:build windows

package handlers

import "fmt"

func readStorageStats(_ string) (AdminStorageStats, error) {
	return AdminStorageStats{}, fmt.Errorf("storage stats are not supported on this platform")
}
