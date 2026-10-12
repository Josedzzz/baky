//go:build unix

package gitbackup

import (
	"os"
	"strconv"
	"syscall"
)

// owner returns the numeric user and group that own a file
func owner(path string) (uid, gid string, ok bool) {
	info, err := os.Stat(path)
	if err != nil {
		return "", "", false
	}
	stat, isStat := info.Sys().(*syscall.Stat_t)
	if !isStat {
		return "", "", false
	}
	return strconv.FormatUint(uint64(stat.Uid), 10), strconv.FormatUint(uint64(stat.Gid), 10), true
}
