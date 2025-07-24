package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
)

const skipNumStackFrames = 3

type handler struct {
	slog.Handler
}

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	_, file, line, ok := runtime.Caller(skipNumStackFrames)
	if ok {
		r.AddAttrs(slog.String("caller", fmt.Sprintf("%s:%d", file, line)))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{Handler: h.Handler.WithGroup(name)}
}

func NewLogger() *slog.Logger {
	return slog.New(&handler{
		Handler: slog.NewTextHandler(
			os.Stdout,
			&slog.HandlerOptions{
				Level: slog.LevelDebug,
			},
		)},
	)
}
