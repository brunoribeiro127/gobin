package internal

import (
	"log/slog"
	"os"
)

func NewLoggerWithLevel(level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(
		os.Stderr,
		&slog.HandlerOptions{
			AddSource: true,
			Level:     level,
		},
	))
}
