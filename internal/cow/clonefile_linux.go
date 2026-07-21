//go:build linux

package cow

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// ficlone is the Linux ioctl for reflinking a file (see rift's reflink.rs).
const ficlone = 0x40049409

// Supported reports whether copy-on-write cloning may be available. Actual
// filesystem reflink support (Btrfs, XFS, bcachefs, ...) is verified by
// precheck before any copying happens.
func Supported() bool { return true }

// precheck mirrors rift's Linux safeguards: reflinks require source and
// destination on the same filesystem, and the filesystem must actually
// support FICLONE, verified with a probe file.
func precheck(src, dst string) error {
	var srcStat, dstStat syscall.Stat_t
	if err := syscall.Stat(src, &srcStat); err != nil {
		return err
	}
	if err := syscall.Stat(filepath.Dir(dst), &dstStat); err != nil {
		return err
	}
	if srcStat.Dev != dstStat.Dev {
		return &CowError{Src: src, Dst: dst, Err: errors.New("source and destination are on different filesystems")}
	}

	probe, err := os.CreateTemp(filepath.Dir(dst), ".jjw-reflink-probe-*")
	if err != nil {
		return err
	}
	probePath := probe.Name()
	defer os.Remove(probePath)
	if _, err := probe.WriteString("probe"); err != nil {
		probe.Close()
		return err
	}
	if err := probe.Close(); err != nil {
		return err
	}
	if err := reflinkFile(probePath, probePath+".clone"); err != nil {
		os.Remove(probePath + ".clone")
		return err
	}
	os.Remove(probePath + ".clone")
	return nil
}

func cloneFile(src, dst string) error {
	return reflinkFile(src, dst)
}

func reflinkFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, dstFile.Fd(), ficlone, srcFile.Fd())
	if errno != 0 {
		dstFile.Close()
		os.Remove(dst)
		return &CowError{Src: src, Dst: dst, Err: errno}
	}
	return dstFile.Close()
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

// clonePreservesMeta is false on Linux: FICLONE only shares extents, the
// destination inode is created fresh and needs mode/time fixups.
const clonePreservesMeta = false

func fileTimes(info os.FileInfo) (atime, mtime time.Time) {
	mtime = info.ModTime()
	atime = mtime
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		atime = time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
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
