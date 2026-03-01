//go:build windows

package main

import (
	"syscall"
	"time"
)

func getFileCreationDate(path string) (*time.Time, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(h)

	var data syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &data); err != nil {
		return nil, err
	}

	t := time.Unix(0, data.CreationTime.Nanoseconds())
	return &t, nil
}
