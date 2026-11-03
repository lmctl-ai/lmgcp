//go:build windows

package safety

import "os"

import (
	"errors"
	"sync"

	"golang.org/x/sys/windows"
)

const oNoFollow = 0

var journalLocks sync.Map // map[*os.File]*windows.Overlapped

func lockFile(file *os.File) error {
	overlapped := &windows.Overlapped{}
	// Reserve a byte outside journal data so readers can inspect active logs.
	overlapped.Offset = 0x7fffffff
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
	if err := windows.LockFileEx(windows.Handle(file.Fd()), flags, 0, 1, 0, overlapped); err != nil {
		return err
	}
	if _, loaded := journalLocks.LoadOrStore(file, overlapped); loaded {
		_ = windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, overlapped)
		return errors.New("journal file is already locked by this process")
	}
	return nil
}

func unlockFile(file *os.File) error {
	value, ok := journalLocks.LoadAndDelete(file)
	if !ok {
		return errors.New("journal file is not locked")
	}
	overlapped := value.(*windows.Overlapped)
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, overlapped)
}

func platformDirCheck(string, os.FileInfo) error { return nil }
func fileOwner(os.FileInfo) (int, bool)          { return 0, false }
