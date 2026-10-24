//go:build !unix

package config

// lockFile is a no-op on platforms without flock; the in-process mutex
// still protects a single baky process
func lockFile(path string) (func(), error) {
	return func() {}, nil
}
