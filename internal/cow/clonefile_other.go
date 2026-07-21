//go:build !darwin && !linux

package cow

import (
	"errors"
	"os"
	"time"
)

// Supported reports whether copy-on-write cloning is available.
func Supported() bool { return false }

func precheck(src, dst string) error {
	return &CowError{Src: src, Dst: dst, Err: errors.New("copy-on-write not supported on this platform")}
}

func cloneFile(src, dst string) error {
	return &CowError{Src: src, Dst: dst, Err: errors.New("copy-on-write not supported on this platform")}
}

// fileIDOf always reports false on unsupported platforms.
func fileIDOf(info os.FileInfo) (fileID, bool) { return fileID{}, false }

// clonePreservesMeta is false on unsupported platforms.
const clonePreservesMeta = false

func fileTimes(info os.FileInfo) (atime, mtime time.Time) {
	mtime = info.ModTime()
	return mtime, mtime
}

// CowError indicates a clone operation failed.
type CowError struct {
	Src string
	Dst string
	Err error
}

func (e *CowError) Error() string {
	return "cow: clone " + e.Src + " -> " + e.Dst + ": " + e.Err.Error()
}

func (e *CowError) Unwrap() error { return e.Err }

// IsCowError reports whether err is a copy-on-write capability failure.
func IsCowError(err error) bool {
	var cowErr *CowError
	return errors.As(err, &cowErr)
}
