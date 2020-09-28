package lumberjack

import (
	"os"
	"path/filepath"
	"syscall"
)

// Chown is a var so we can mock it out during tests.
var Chown = os.Chown

func chown(name string, info os.FileInfo) error {
	file, err := os.OpenFile(filepath.Clean(name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	file.Close() // nolint
	stat := info.Sys().(*syscall.Stat_t)
	return Chown(name, int(stat.Uid), int(stat.Gid))
}
