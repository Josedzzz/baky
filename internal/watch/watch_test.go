package watch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestIsWithin(t *testing.T) {
	cases := []struct {
		name, root string
		want       bool
	}{
		{"/etc/nginx", "/etc/nginx", true},
		{"/etc/nginx/conf.d/a.conf", "/etc/nginx", true},
		{"/etc/nginx/a.conf", "/etc/nginx/", true},
		{"/etc/nginx2/a.conf", "/etc/nginx", false},
		{"/etc/nginx.conf.bak", "/etc/nginx.conf", false},
	}
	for _, c := range cases {
		if got := IsWithin(c.name, c.root); got != c.want {
			t.Errorf("IsWithin(%q, %q) = %v, expected %v", c.name, c.root, got, c.want)
		}
	}
}

func TestAddRecursive(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "a", "b"), 0o755)
	file := filepath.Join(root, "single", "nginx.conf")
	os.MkdirAll(filepath.Dir(file), 0o755)
	os.WriteFile(file, []byte("x"), 0o644)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Skipf("fsnotify not available: %v", err)
	}
	defer w.Close()

	AddRecursive(w, filepath.Join(root, "a"))
	AddRecursive(w, file)

	watched := map[string]bool{}
	for _, p := range w.WatchList() {
		watched[p] = true
	}
	for _, want := range []string{filepath.Join(root, "a"), filepath.Join(root, "a", "b"), filepath.Dir(file)} {
		if !watched[want] {
			t.Errorf("expected %s to be watched, watching %v", want, w.WatchList())
		}
	}

	RemoveRecursive(w, filepath.Join(root, "a"))
	if len(w.WatchList()) != 1 {
		t.Errorf("expected only the file's directory to stay watched, watching %v", w.WatchList())
	}
}
