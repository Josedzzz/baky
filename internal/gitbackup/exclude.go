package gitbackup

import (
	"bytes"
	"path"
	"path/filepath"
	"strings"
)

// DefaultExcludes are left out of every watched folder: the usual names of
// private keys and secrets. The config repository is pushed to a remote,
// so a secret that reaches it is no longer a secret.
var DefaultExcludes = []string{
	"*.key", "*.pem", "*.p12", "*.pfx", "*.jks", "*.keystore", "*.kdbx",
	"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519",
	"shadow", "gshadow", "*.secret", "*.secrets",
	"pki/", "private/", "secrets/",
}

// secretMarkers are found inside files that hold a private key, whatever
// their name: PEM keys, OpenSSH keys, and OpenVPN static keys, which are
// often inlined in a .conf
var secretMarkers = [][]byte{
	[]byte("PRIVATE KEY-----"),
	[]byte("-----BEGIN OpenVPN Static key"),
}

type excludePattern struct {
	glob     string
	dirOnly  bool // ends with "/"
	anchored bool // has a "/" inside: matched against the relative path
}

// excluder decides what of a watched folder is left out of the repository
type excluder struct {
	patterns []excludePattern
}

// newExcluder combines DefaultExcludes with the extra patterns of a watch,
// which use the syntax of path.Match:
//
//   - "*.key" matches a file or folder with that name at any depth
//   - "pki/" with a trailing slash matches only folders
//   - "certs/server.crt" with a slash inside matches that path, relative
//     to the watched folder
//
// Invalid patterns are skipped: they are rejected when they are saved.
func newExcluder(extra []string) *excluder {
	e := &excluder{}
	for _, p := range append(append([]string{}, DefaultExcludes...), extra...) {
		glob := strings.TrimSuffix(p, "/")
		if _, err := path.Match(glob, ""); err != nil || glob == "" {
			continue
		}
		e.patterns = append(e.patterns, excludePattern{
			glob:     glob,
			dirOnly:  strings.HasSuffix(p, "/"),
			anchored: strings.Contains(glob, "/"),
		})
	}
	return e
}

// excluded reports whether rel, relative to the watched folder, or any
// folder above it matches a pattern. A nil excluder excludes nothing.
func (e *excluder) excluded(rel string, isDir bool) bool {
	if e == nil {
		return false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i, name := range parts {
		dir := i < len(parts)-1 || isDir
		prefix := strings.Join(parts[:i+1], "/")
		for _, p := range e.patterns {
			if p.dirOnly && !dir {
				continue
			}
			subject := name
			if p.anchored {
				subject = prefix
			}
			if ok, _ := path.Match(p.glob, subject); ok {
				return true
			}
		}
	}
	return false
}

// holdsSecret reports whether content looks like it contains a private
// key. A nil excluder checks nothing.
func (e *excluder) holdsSecret(content []byte) bool {
	if e == nil {
		return false
	}
	for _, marker := range secretMarkers {
		if bytes.Contains(content, marker) {
			return true
		}
	}
	return false
}
