//go:build windows

package fsutil

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

// FreeBytes returns available bytes on the volume containing path.
func FreeBytes(path string) (uint64, error) {
	path = filepath.Clean(path)
	// GetDiskFreeSpaceEx wants a directory; use drive root if needed.
	dir := path
	if fi, err := windows.GetFileAttributes(windows.StringToUTF16Ptr(path)); err == nil {
		if fi&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
			dir = filepath.Dir(path)
		}
	} else {
		dir = filepath.Dir(path)
	}

	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64
	err := windows.GetDiskFreeSpaceEx(
		windows.StringToUTF16Ptr(dir),
		&freeBytesAvailable,
		&totalNumberOfBytes,
		&totalNumberOfFreeBytes,
	)
	if err != nil {
		return 0, err
	}
	return freeBytesAvailable, nil
}
