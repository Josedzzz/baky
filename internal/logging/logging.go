// Package logging writes and reads the daemon log file
package logging

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Log levels, as written in the log file
const (
	Info  = "INFO"
	Warn  = "WARN"
	Error = "ERROR"
)

// Func is the signature the backup engines use to report what they do
type Func func(level, message string)

// Logger appends timestamped lines to the log file and echoes them to
// another writer (standard output, which systemd sends to the journal)
type Logger struct {
	mu   sync.Mutex
	file *os.File
	echo io.Writer
}

// New opens the log file at path for appending. echo may be nil.
func New(path string, echo io.Writer) (*Logger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &Logger{file: file, echo: echo}, nil
}

// Log writes one line: "2006-01-02 15:04:05 [LEVEL] message"
func (l *Logger) Log(level, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	line := fmt.Sprintf("%s [%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), level, message)
	l.file.WriteString(line)
	if l.echo != nil {
		io.WriteString(l.echo, line)
	}
}

// Logf formats and writes one line
func (l *Logger) Logf(level, format string, args ...any) {
	l.Log(level, fmt.Sprintf(format, args...))
}

// Close closes the log file
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

// Tail returns at most the last n lines of the file at path, oldest first
func Tail(path string, n int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Keep only the last n lines while reading, so a large log costs no memory
	lines := make([]string, 0, n)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if len(lines) == n {
			copy(lines, lines[1:])
			lines = lines[:n-1]
		}
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
