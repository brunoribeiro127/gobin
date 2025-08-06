package internal

import (
	"log/slog"
	"os"
)

// NewLoggerWithLevel creates a new logger with the specified log level. It uses
// the slog package and adds the source file and line number to the log messages.
func NewLoggerWithLevel(level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(
		os.Stderr,
		&slog.HandlerOptions{
			AddSource: true,
			Level:     level,
		},
	))
}
