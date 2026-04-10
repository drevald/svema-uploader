//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

const MTPPrefix = "mtp:"

// MTPEntry represents a file or directory in an MTP device
type MTPEntry struct {
	Name  string
	IsDir bool
}

// IsMTPPath returns true if the path is an MTP virtual path
func IsMTPPath(path string) bool {
	return strings.HasPrefix(path, MTPPrefix)
}

// MTPPathJoin appends a child name to an MTP path
func MTPPathJoin(parent, child string) string {
	if parent == MTPPrefix {
		return MTPPrefix + child
	}
	return parent + "\\" + child
}

// MTPPathParent returns the parent MTP path
func MTPPathParent(mtpPath string) string {
	inner := strings.TrimPrefix(mtpPath, MTPPrefix)
	idx := strings.LastIndex(inner, "\\")
	if idx < 0 {
		return MTPPrefix
	}
	return MTPPrefix + inner[:idx]
}

// MTPPathBase returns the last component of an MTP path
func MTPPathBase(mtpPath string) string {
	inner := strings.TrimPrefix(mtpPath, MTPPrefix)
	idx := strings.LastIndex(inner, "\\")
	if idx < 0 {
		return inner
	}
	return inner[idx+1:]
}

// parseMTPPath returns the device name and path components
// "mtp:DeviceName\sub\dir" -> "DeviceName", ["sub", "dir"]
func parseMTPPath(mtpPath string) (deviceName string, components []string) {
	inner := strings.TrimPrefix(mtpPath, MTPPrefix)
	if inner == "" {
		return "", nil
	}
	parts := strings.Split(inner, "\\")
	if len(parts) == 0 {
		return "", nil
	}
	deviceName = parts[0]
	for _, p := range parts[1:] {
		if p != "" {
			components = append(components, p)
		}
	}
	return deviceName, components
}

// isMTPDevicePath returns true if pathStr looks like a portable device shell path
// (not a regular drive letter or filesystem path).
// Drive/filesystem paths start with a letter then ':', e.g. "C:\", "C:\Users\...".
// MTP device paths start with "::{GUID}\..." — note position 0 is ':', not a letter.
func isMTPDevicePath(pathStr string) bool {
	if len(pathStr) == 0 {
		return false
	}
	first := pathStr[0]
	if (first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z') {
		// Starts with a letter — regular drive or filesystem path
		return false
	}
	return true
}

// withShell initializes COM and creates a Shell.Application dispatch, calls fn, then cleans up.
func withShell(fn func(shellDisp *ole.IDispatch) error) error {
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		oleErr, ok := err.(*ole.OleError)
		if !ok || (oleErr.Code() != 0 && oleErr.Code() != 1) {
			return fmt.Errorf("CoInitializeEx: %w", err)
		}
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("Shell.Application")
	if err != nil {
		return fmt.Errorf("create Shell.Application: %w", err)
	}
	defer unknown.Release()

	shellDisp, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("QueryInterface IDispatch: %w", err)
	}
	defer shellDisp.Release()

	return fn(shellDisp)
}

// getThisPCNamespace returns the "This PC" folder (CSIDL_DRIVES=17)
func getThisPCNamespace(shellDisp *ole.IDispatch) (*ole.IDispatch, error) {
	v, err := oleutil.CallMethod(shellDisp, "NameSpace", 17)
	if err != nil {
		return nil, fmt.Errorf("NameSpace(17): %w", err)
	}
	disp := v.ToIDispatch()
	if disp == nil {
		return nil, fmt.Errorf("NameSpace(17) returned nil")
	}
	return disp, nil
}

// folderItems returns the Items collection and count of a Folder dispatch
func folderItems(folder *ole.IDispatch) (*ole.IDispatch, int, error) {
	v, err := oleutil.CallMethod(folder, "Items")
	if err != nil {
		return nil, 0, fmt.Errorf("Items(): %w", err)
	}
	items := v.ToIDispatch()
	if items == nil {
		return nil, 0, fmt.Errorf("Items() returned nil")
	}
	cv, err := oleutil.GetProperty(items, "Count")
	if err != nil {
		items.Release()
		return nil, 0, fmt.Errorf("Count: %w", err)
	}
	return items, int(cv.Val), nil
}

// findItemByName returns a FolderItem dispatch matching name (case-insensitive)
func findItemByName(folder *ole.IDispatch, name string) (*ole.IDispatch, error) {
	items, count, err := folderItems(folder)
	if err != nil {
		return nil, err
	}
	defer items.Release()

	for i := 0; i < count; i++ {
		iv, err := oleutil.CallMethod(items, "Item", i)
		if err != nil {
			continue
		}
		item := iv.ToIDispatch()
		if item == nil {
			continue
		}
		nv, err := oleutil.GetProperty(item, "Name")
		if err != nil {
			item.Release()
			continue
		}
		if strings.EqualFold(nv.ToString(), name) {
			return item, nil
		}
		item.Release()
	}
	return nil, fmt.Errorf("item %q not found", name)
}

// openFolderFromItem opens a FolderItem as a Folder, trying multiple approaches.
func openFolderFromItem(shellDisp *ole.IDispatch, item *ole.IDispatch) (*ole.IDispatch, error) {
	// Approach 1: GetFolder()
	v, err := oleutil.CallMethod(item, "GetFolder")
	if err == nil && v.VT != ole.VT_EMPTY && v.VT != ole.VT_NULL {
		if folder := v.ToIDispatch(); folder != nil {
			return folder, nil
		}
	}

	// Approach 2: Shell.NameSpace(item) — pass the item itself as namespace
	v2, err := oleutil.CallMethod(shellDisp, "NameSpace", item)
	if err == nil && v2.VT != ole.VT_EMPTY && v2.VT != ole.VT_NULL {
		if folder := v2.ToIDispatch(); folder != nil {
			return folder, nil
		}
	}

	// Approach 3: Shell.NameSpace(item.Path)
	pathV, pathErr := oleutil.GetProperty(item, "Path")
	if pathErr == nil {
		pathStr := pathV.ToString()
		if pathStr != "" {
			v3, err := oleutil.CallMethod(shellDisp, "NameSpace", pathStr)
			if err == nil && v3.VT != ole.VT_EMPTY && v3.VT != ole.VT_NULL {
				if folder := v3.ToIDispatch(); folder != nil {
					return folder, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("cannot open folder (GetFolder/NameSpace all returned nil)")
}

// navigateToMTPFolder returns the Folder dispatch for the given MTP path.
// Caller must Release() the returned dispatch.
func navigateToMTPFolder(shellDisp *ole.IDispatch, mtpPath string) (*ole.IDispatch, error) {
	deviceName, components := parseMTPPath(mtpPath)

	thisPCFolder, err := getThisPCNamespace(shellDisp)
	if err != nil {
		return nil, err
	}

	if deviceName == "" {
		return thisPCFolder, nil
	}

	deviceItem, err := findItemByName(thisPCFolder, deviceName)
	thisPCFolder.Release()
	if err != nil {
		return nil, fmt.Errorf("MTP device %q not found under This PC", deviceName)
	}

	currentFolder, err := openFolderFromItem(shellDisp, deviceItem)
	deviceItem.Release()
	if err != nil {
		return nil, fmt.Errorf("cannot open device %q: %w", deviceName, err)
	}

	for _, component := range components {
		item, err := findItemByName(currentFolder, component)
		currentFolder.Release()
		if err != nil {
			return nil, fmt.Errorf("cannot find %q in MTP path: %w", component, err)
		}
		nextFolder, err := openFolderFromItem(shellDisp, item)
		item.Release()
		if err != nil {
			return nil, fmt.Errorf("cannot open %q: %w", component, err)
		}
		currentFolder = nextFolder
	}

	return currentFolder, nil
}

// GetMTPDeviceNames returns names of portable device items visible under "This PC"
// (excludes regular drives and filesystem paths like Downloads, Pictures, etc.)
func GetMTPDeviceNames() []string {
	var names []string
	withShell(func(shellDisp *ole.IDispatch) error {
		thisPCFolder, err := getThisPCNamespace(shellDisp)
		if err != nil {
			return err
		}
		defer thisPCFolder.Release()

		items, count, err := folderItems(thisPCFolder)
		if err != nil {
			return err
		}
		defer items.Release()

		for i := 0; i < count; i++ {
			iv, err := oleutil.CallMethod(items, "Item", i)
			if err != nil {
				continue
			}
			item := iv.ToIDispatch()
			if item == nil {
				continue
			}

			pathVar, _ := oleutil.GetProperty(item, "Path")
			pathStr := pathVar.ToString()

			// Only include items whose shell path is NOT a regular filesystem path.
			// Regular drives: "C:\", "D:\". User folders: "C:\Users\...\Downloads".
			// MTP devices: "::{GUID}\\\?\usb#..." or other non-letter-colon paths.
			if !isMTPDevicePath(pathStr) {
				item.Release()
				continue
			}

			nameVar, err := oleutil.GetProperty(item, "Name")
			if err == nil && nameVar.ToString() != "" {
				names = append(names, nameVar.ToString())
			}
			item.Release()
		}
		return nil
	})
	return names
}

// ListMTPDirectory lists entries in an MTP path.
func ListMTPDirectory(mtpPath string) ([]MTPEntry, error) {
	var entries []MTPEntry

	err := withShell(func(shellDisp *ole.IDispatch) error {
		if mtpPath == MTPPrefix {
			// List MTP devices only
			thisPCFolder, err := getThisPCNamespace(shellDisp)
			if err != nil {
				return err
			}
			defer thisPCFolder.Release()

			items, count, err := folderItems(thisPCFolder)
			if err != nil {
				return err
			}
			defer items.Release()

			for i := 0; i < count; i++ {
				iv, err := oleutil.CallMethod(items, "Item", i)
				if err != nil {
					continue
				}
				item := iv.ToIDispatch()
				if item == nil {
					continue
				}

				pathVar, _ := oleutil.GetProperty(item, "Path")
				if !isMTPDevicePath(pathVar.ToString()) {
					item.Release()
					continue
				}

				nameVar, _ := oleutil.GetProperty(item, "Name")
				if nameVar.ToString() != "" {
					entries = append(entries, MTPEntry{Name: nameVar.ToString(), IsDir: true})
				}
				item.Release()
			}
			return nil
		}

		folder, err := navigateToMTPFolder(shellDisp, mtpPath)
		if err != nil {
			return err
		}
		defer folder.Release()

		items, count, err := folderItems(folder)
		if err != nil {
			return err
		}
		defer items.Release()

		for i := 0; i < count; i++ {
			iv, err := oleutil.CallMethod(items, "Item", i)
			if err != nil {
				continue
			}
			item := iv.ToIDispatch()
			if item == nil {
				continue
			}

			nameVar, _ := oleutil.GetProperty(item, "Name")
			isFolderVar, _ := oleutil.GetProperty(item, "IsFolder")

			name := nameVar.ToString()
			isDir := isFolderVar.Val != 0
			if name != "" {
				entries = append(entries, MTPEntry{Name: name, IsDir: isDir})
			}
			item.Release()
		}
		return nil
	})

	return entries, err
}

// CountMTPImageFiles counts image files in an MTP path without copying them.
func CountMTPImageFiles(mtpPath string, recursive bool) int {
	count := 0
	withShell(func(shellDisp *ole.IDispatch) error {
		countMTPImages(shellDisp, mtpPath, recursive, &count)
		return nil
	})
	return count
}

func countMTPImages(shellDisp *ole.IDispatch, mtpPath string, recursive bool, count *int) {
	folder, err := navigateToMTPFolder(shellDisp, mtpPath)
	if err != nil {
		return
	}
	defer folder.Release()

	items, total, err := folderItems(folder)
	if err != nil {
		return
	}
	defer items.Release()

	for i := 0; i < total; i++ {
		iv, err := oleutil.CallMethod(items, "Item", i)
		if err != nil {
			continue
		}
		item := iv.ToIDispatch()
		if item == nil {
			continue
		}
		nameVar, _ := oleutil.GetProperty(item, "Name")
		isFolderVar, _ := oleutil.GetProperty(item, "IsFolder")
		name := nameVar.ToString()
		isDir := isFolderVar.Val != 0
		item.Release()

		if isDir {
			if recursive && name != "" {
				countMTPImages(shellDisp, MTPPathJoin(mtpPath, name), recursive, count)
			}
		} else {
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".tiff" || ext == ".tif" {
				*count++
			}
		}
	}
}

// CopyMTPToTemp copies image files from the MTP path to a new temp directory.
// Returns the temp dir path and a cleanup function.
func CopyMTPToTemp(mtpPath string, copySubdirs bool, progressFn func(name string)) (string, func(), error) {
	tempDir, err := os.MkdirTemp("", "svema-mtp-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}
	cleanup := func() { os.RemoveAll(tempDir) }

	err = copyMTPFolderToDir(mtpPath, tempDir, copySubdirs, progressFn)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	return tempDir, cleanup, nil
}

// copyMTPFolderToDir copies image files from an MTP folder to a local directory.
func copyMTPFolderToDir(mtpPath, destDir string, recursive bool, progressFn func(name string)) error {
	return withShell(func(shellDisp *ole.IDispatch) error {
		destNsV, err := oleutil.CallMethod(shellDisp, "NameSpace", destDir)
		if err != nil {
			return fmt.Errorf("NameSpace(%s): %w", destDir, err)
		}
		destFolder := destNsV.ToIDispatch()
		if destFolder == nil {
			return fmt.Errorf("dest namespace is nil")
		}
		defer destFolder.Release()

		srcFolder, err := navigateToMTPFolder(shellDisp, mtpPath)
		if err != nil {
			return err
		}
		defer srcFolder.Release()

		items, count, err := folderItems(srcFolder)
		if err != nil {
			return err
		}
		defer items.Release()

		for i := 0; i < count; i++ {
			iv, err := oleutil.CallMethod(items, "Item", i)
			if err != nil {
				continue
			}
			item := iv.ToIDispatch()
			if item == nil {
				continue
			}

			nameVar, _ := oleutil.GetProperty(item, "Name")
			isFolderVar, _ := oleutil.GetProperty(item, "IsFolder")
			name := nameVar.ToString()
			isDir := isFolderVar.Val != 0

			if isDir {
				if recursive && name != "" {
					subDest := filepath.Join(destDir, name)
					if mkErr := os.MkdirAll(subDest, 0755); mkErr == nil {
						subMTP := MTPPathJoin(mtpPath, name)
						copyMTPFolderToDir(subMTP, subDest, recursive, progressFn)
					}
				}
				item.Release()
				continue
			}

			ext := strings.ToLower(filepath.Ext(name))
			isImage := ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".tiff" || ext == ".tif"
			if !isImage {
				item.Release()
				continue
			}

			if progressFn != nil {
				progressFn(name)
			}

			// CopyHere: 4=no progress dialog, 16=yes to all, 1024=no error UI
			oleutil.CallMethod(destFolder, "CopyHere", item, 4|16|1024)
			item.Release()

			// Wait for file to appear (CopyHere is asynchronous)
			destPath := filepath.Join(destDir, name)
			for tries := 0; tries < 150; tries++ {
				if fi, statErr := os.Stat(destPath); statErr == nil && fi.Size() > 0 {
					break
				}
				time.Sleep(200 * time.Millisecond)
			}
		}
		return nil
	})
}
