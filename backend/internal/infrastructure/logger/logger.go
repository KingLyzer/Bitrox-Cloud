package logger

import (
	"log/slog"
	"os"
	"strings"
)

func New(level, format string) *slog.Logger {
	lvl := parseLevel(level)

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level:     lvl,
		AddSource: true,
	}

	if strings.ToLower(format) == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
