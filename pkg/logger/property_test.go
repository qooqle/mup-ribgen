package logger_test

// Property 24: Error logs are emitted (req 9.1, 9.2)
// Property 25: Log level filtering is applied (req 9.3)

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/qooqle/mup-ribgen/pkg/logger"
)

// logEntry mirrors the JSON structure written by Logger.
type logEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Component string `json:"component"`
	Message   string `json:"message"`
}

func parseLastEntry(buf *bytes.Buffer) (*logEntry, bool) {
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 {
		return nil, false
	}
	var e logEntry
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &e); err != nil {
		return nil, false
	}
	return &e, true
}

// Property 24: For any message logged at ERROR level, the output must contain
// a valid JSON entry with non-empty timestamp, level="ERROR", non-empty
// component, and the original message (req 9.1, 9.2).
func TestProperty24_ErrorLogsAreEmitted(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("req9.1,9.2: ERROR entry has timestamp, level, component, message", prop.ForAll(
		func(component, message string) bool {
			if component == "" || message == "" {
				return true // skip degenerate
			}
			var buf bytes.Buffer
			l := logger.New(component, logger.ERROR, &buf)
			l.Error(message)

			e, ok := parseLastEntry(&buf)
			if !ok {
				return false
			}
			return e.Timestamp != "" &&
				e.Level == "ERROR" &&
				e.Component == component &&
				e.Message == message
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// All four levels produce entries with correct level labels when at DEBUG.
	properties.Property("req9.2: level label is correct for all levels", prop.ForAll(
		func(msg string) bool {
			if msg == "" {
				return true
			}
			type tc struct {
				emit  func(*logger.Logger)
				label string
			}
			cases := []tc{
				{func(l *logger.Logger) { l.Debug(msg) }, "DEBUG"},
				{func(l *logger.Logger) { l.Info(msg) }, "INFO"},
				{func(l *logger.Logger) { l.Warn(msg) }, "WARN"},
				{func(l *logger.Logger) { l.Error(msg) }, "ERROR"},
			}
			for _, c := range cases {
				var buf bytes.Buffer
				l := logger.New("comp", logger.DEBUG, &buf)
				c.emit(l)
				e, ok := parseLastEntry(&buf)
				if !ok || e.Level != c.label {
					return false
				}
			}
			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Property 25: Entries below the configured minimum level must not be written
// to the output (req 9.3).
func TestProperty25_LogLevelFilteringIsApplied(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// When minLevel=ERROR, DEBUG/INFO/WARN must be suppressed.
	properties.Property("req9.3: entries below minLevel are suppressed", prop.ForAll(
		func(msg string) bool {
			if msg == "" {
				return true
			}
			var buf bytes.Buffer
			l := logger.New("comp", logger.ERROR, &buf)
			l.Debug(msg)
			l.Info(msg)
			l.Warn(msg)
			// Nothing should have been written yet.
			if buf.Len() != 0 {
				return false
			}
			// ERROR must still appear.
			l.Error(msg)
			return buf.Len() > 0
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// SetLevel at runtime is respected.
	properties.Property("req9.3: SetLevel dynamically changes filtering", prop.ForAll(
		func(msg string) bool {
			if msg == "" {
				return true
			}
			var buf bytes.Buffer
			l := logger.New("comp", logger.ERROR, &buf)
			l.Info(msg) // suppressed
			if buf.Len() != 0 {
				return false
			}
			l.SetLevel(logger.INFO)
			l.Info(msg) // now visible
			return buf.Len() > 0
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
