//go:build darwin

package main

import (
	"os"
	"syscall"
	"time"
)

func getFileCreationDate(path string) (*time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	stat := info.Sys().(*syscall.Stat_t)
	t := time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
	return &t, nil
}
