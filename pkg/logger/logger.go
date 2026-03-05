// Package logger provides structured JSON logging with level filtering.
// Each log entry contains timestamp, level, component, and message (req 9.1–9.3).
package logger

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// Level is the severity of a log entry.
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

var levelNames = map[Level]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

func (l Level) String() string {
	if s, ok := levelNames[l]; ok {
		return s
	}
	return "UNKNOWN"
}

// ParseLevel converts a string like "DEBUG" to the corresponding Level.
// Returns INFO and an error if the string is unrecognised.
func ParseLevel(s string) (Level, error) {
	for lvl, name := range levelNames {
		if name == s {
			return lvl, nil
		}
	}
	return INFO, &UnknownLevelError{s}
}

// UnknownLevelError is returned when ParseLevel receives an unrecognised string.
type UnknownLevelError struct{ Value string }

func (e *UnknownLevelError) Error() string {
	return "logger: unknown level: " + e.Value
}

// entry is the JSON-serialisable log record.
type entry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Component string `json:"component"`
	Message   string `json:"message"`
}

// Logger writes structured JSON log entries to an io.Writer.
type Logger struct {
	mu        sync.Mutex
	out       io.Writer
	minLevel  Level
	component string
}

// New creates a Logger that writes to out, filtering entries below minLevel.
func New(component string, minLevel Level, out io.Writer) *Logger {
	return &Logger{out: out, minLevel: minLevel, component: component}
}

// Default returns a Logger writing to stderr at INFO level with component "mup-controller".
func Default() *Logger {
	return New("mup-controller", INFO, os.Stderr)
}

// SetLevel changes the minimum log level at runtime.
func (l *Logger) SetLevel(lvl Level) {
	l.mu.Lock()
	l.minLevel = lvl
	l.mu.Unlock()
}

// Debug logs at DEBUG level.
func (l *Logger) Debug(msg string) { l.log(DEBUG, msg) }

// Info logs at INFO level.
func (l *Logger) Info(msg string) { l.log(INFO, msg) }

// Warn logs at WARN level.
func (l *Logger) Warn(msg string) { l.log(WARN, msg) }

// Error logs at ERROR level.
func (l *Logger) Error(msg string) { l.log(ERROR, msg) }

func (l *Logger) log(lvl Level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if lvl < l.minLevel {
		return
	}
	e := entry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     lvl.String(),
		Component: l.component,
		Message:   msg,
	}
	data, _ := json.Marshal(e)
	data = append(data, '\n')
	_, _ = l.out.Write(data)
}
