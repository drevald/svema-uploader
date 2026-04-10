//go:build !windows

package main

const MTPPrefix = "mtp:"

// MTPEntry represents a file or directory in an MTP device
type MTPEntry struct {
	Name  string
	IsDir bool
}

func IsMTPPath(path string) bool                                              { return false }
func MTPPathJoin(parent, child string) string                                 { return parent + "/" + child }
func MTPPathParent(mtpPath string) string                                     { return MTPPrefix }
func MTPPathBase(mtpPath string) string                                       { return mtpPath }
func GetMTPDeviceNames() []string                                             { return nil }
func parseMTPPath(mtpPath string) (deviceName string, components []string)    { return "", nil }
func ListMTPDirectory(mtpPath string) ([]MTPEntry, error)                     { return nil, nil }
func CopyMTPToTemp(mtpPath string, copySubdirs bool, progressFn func(name string)) (string, func(), error) {
	return "", func() {}, nil
}

func CountMTPImageFiles(mtpPath string, recursive bool) int { return 0 }
