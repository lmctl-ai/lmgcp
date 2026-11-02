//go:build windows

package safety

import "os"

const oNoFollow = 0

func lockFile(*os.File) error           { return nil }
func unlockFile(*os.File) error         { return nil }
func fileOwner(os.FileInfo) (int, bool) { return 0, false }
