//go:build !windows

package safety

import (
	"fmt"
	"os"
	"syscall"
)

const oNoFollow = syscall.O_NOFOLLOW

func lockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

func unlockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}

func platformDirCheck(dir string, info os.FileInfo) error {
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("state directory %q has group/world permissions %04o", dir, info.Mode().Perm())
	}
	if uid, ok := fileOwner(info); ok && uid != os.Getuid() {
		return fmt.Errorf("state directory %q is owned by uid %d, not %d", dir, uid, os.Getuid())
	}
	return nil
}

func fileOwner(info os.FileInfo) (int, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return int(stat.Uid), true
}
