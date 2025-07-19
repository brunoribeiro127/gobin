package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
)

const skipNumStackFrames = 3

type handler struct {
	slog.Handler

	ModName string
}

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	_, file, line, ok := runtime.Caller(skipNumStackFrames)
	if ok {
		rel := filepath.Base(file)
		if h.ModName != "" {
			if idx := strings.Index(file, h.ModName); idx != -1 {
				rel = file[idx+len(h.ModName)+1:]
			}
		}
		r.AddAttrs(slog.String("caller", fmt.Sprintf("%s:%d", rel, line)))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handler{
		Handler: h.Handler.WithAttrs(attrs),
		ModName: h.ModName,
	}
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{
		Handler: h.Handler.WithGroup(name),
		ModName: h.ModName,
	}
}

func NewLogger() *slog.Logger {
	modName := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		modSplit := strings.Split(info.Main.Path, "/")
		modName = modSplit[len(modSplit)-1]
	}

	return slog.New(&handler{
		ModName: modName,
		Handler: slog.NewTextHandler(
			os.Stdout,
			&slog.HandlerOptions{
				Level: slog.LevelDebug,
			},
		)},
	)
}
