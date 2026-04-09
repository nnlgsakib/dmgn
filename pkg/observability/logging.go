package observability

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// LogConfig holds logging configuration.
type LogConfig struct {
	Level      string
	LogDir     string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Stderr     bool
}

// LogOutput holds the logger and any closeable resources.
type LogOutput struct {
	Logger *slog.Logger
	closers []io.Closer
}

// Close releases any file handles held by the log output.
func (lo *LogOutput) Close() {
	for _, c := range lo.closers {
		c.Close()
	}
}

// InitLogging creates a structured JSON logger with optional dual output
// (stderr + rotating log file).
func InitLogging(cfg LogConfig) *LogOutput {
	level := LogLevelFromString(cfg.Level)
	opts := &slog.HandlerOptions{Level: level}

	var writers []io.Writer
	var closers []io.Closer

	if cfg.Stderr {
		writers = append(writers, os.Stderr)
	}

	if cfg.LogDir != "" {
		if err := os.MkdirAll(cfg.LogDir, 0755); err == nil {
			maxSize := cfg.MaxSizeMB
			if maxSize <= 0 {
				maxSize = 10
			}
			maxBackups := cfg.MaxBackups
			if maxBackups <= 0 {
				maxBackups = 5
			}
			maxAge := cfg.MaxAgeDays
			if maxAge <= 0 {
				maxAge = 30
			}

			lj := &lumberjack.Logger{
				Filename:   filepath.Join(cfg.LogDir, "dmgn.log"),
				MaxSize:    maxSize,
				MaxBackups: maxBackups,
				MaxAge:     maxAge,
				Compress:   true,
			}
			writers = append(writers, lj)
			closers = append(closers, lj)
		}
	}

	if len(writers) == 0 {
		writers = append(writers, os.Stderr)
	}

	var w io.Writer
	if len(writers) == 1 {
		w = writers[0]
	} else {
		w = io.MultiWriter(writers...)
	}

	handler := slog.NewJSONHandler(w, opts)
	return &LogOutput{
		Logger:  slog.New(handler),
		closers: closers,
	}
}

// LogLevelFromString parses a log level string to slog.Level.
func LogLevelFromString(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
