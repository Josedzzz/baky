package logging

import (
	"bytes"
	"path/filepath"
	"regexp"
	"testing"
)

func TestLoggerAndTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baky.log")

	var echo bytes.Buffer
	logger, err := New(path, &echo)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 5; i++ {
		logger.Logf(Info, "linea %d", i)
	}
	logger.Log(Error, "fallo")
	logger.Close()

	lines, err := Tail(path, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}

	format := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} \[(INFO|ERROR)\] `)
	for i, want := range []string{"linea 4", "linea 5", "fallo"} {
		if !format.MatchString(lines[i]) || lines[i][len(lines[i])-len(want):] != want {
			t.Errorf("line %d: expected a timestamped %q, got %q", i, want, lines[i])
		}
	}

	if all, _ := Tail(path, 100); len(all) != 6 {
		t.Errorf("expected all 6 lines, got %d", len(all))
	}
	if got := bytes.Count(echo.Bytes(), []byte("\n")); got != 6 {
		t.Errorf("expected 6 echoed lines, got %d", got)
	}

	if _, err := Tail(filepath.Join(t.TempDir(), "missing.log"), 10); err == nil {
		t.Errorf("expected an error for a missing log file")
	}
}
