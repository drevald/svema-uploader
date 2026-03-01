//go:build !windows

package main

import (
	"os"
	"time"
)

// getFileCreationDate falls back to modification time on non-Windows platforms
// since creation time is not universally accessible.
func getFileCreationDate(path string) (*time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	t := info.ModTime()
	return &t, nil
}
