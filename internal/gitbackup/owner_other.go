//go:build !unix

package gitbackup

// owner is not available on this platform: files keep the default owner
func owner(path string) (uid, gid string, ok bool) {
	return "", "", false
}
