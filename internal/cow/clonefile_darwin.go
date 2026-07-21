//go:build darwin

package cow

import (
	"errors"
	"os"
	"syscall"
	"time"
	"unsafe"
)

// sysClonefileat is the Darwin syscall number for clonefileat(2); the libc
// clonefile(2) wrapper is not reachable via a raw syscall.
const sysClonefileat = 462

// atFdcwd is a variable so the negative value converts to uintptr at run time.
var atFdcwd = -2

// Supported reports whether copy-on-write cloning is available. Actual APFS
// support is verified lazily by the first clonefile call.
func Supported() bool { return true }

// precheck is a no-op on macOS: clonefile(2) fails with a clear error on
// non-APFS volumes or cross-volume clones, and CloneDir surfaces it.
func precheck(src, dst string) error { return nil }

func cloneFile(src, dst string) error {
	srcPtr, err := syscall.BytePtrFromString(src)
	if err != nil {
		return err
	}
	dstPtr, err := syscall.BytePtrFromString(dst)
	if err != nil {
		return err
	}
	_, _, errno := syscall.Syscall6(
		sysClonefileat,
		uintptr(atFdcwd), uintptr(unsafe.Pointer(srcPtr)),
		uintptr(atFdcwd), uintptr(unsafe.Pointer(dstPtr)),
		0, 0,
	)
	if errno != 0 {
		return &CowError{Src: src, Dst: dst, Err: errno}
	}
	return nil
}

// fileIDOf returns the (device, inode) identity of a multiply-linked file,
// used to preserve hard links in the clone.
func fileIDOf(info os.FileInfo) (fileID, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink < 2 {
		return fileID{}, false
	}
	return fileID{dev: uint64(stat.Dev), ino: stat.Ino}, true
}

// clonePreservesMeta is true on macOS: APFS clonefile(2) clones permissions
// and timestamps along with the data, so no fixups are needed afterwards.
const clonePreservesMeta = true

func fileTimes(info os.FileInfo) (atime, mtime time.Time) {
	mtime = info.ModTime()
	atime = mtime
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		atime = time.Unix(stat.Atimespec.Sec, stat.Atimespec.Nsec)
	}
	return atime, mtime
}

// CowError indicates a clone operation failed, typically because the
// filesystem does not support copy-on-write clones.
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
