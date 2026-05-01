// Package pkg provides shared, reusable utilities used across the entire application.
// This includes the structured logger, JWT helpers, password utilities, and common types.
package pkg

import (
	"log/slog"
	"os"
)

// Logger wraps two slog.Logger instances — one writing JSON to a file and one writing
// human-readable text to stdout — so every log call is captured in both places.
type Logger struct {
	FileLogger   *slog.Logger // JSON handler → logs/app.log
	StdoutLogger *slog.Logger // Text handler → os.Stdout
	file         *os.File     // Underlying file handle; closed via Close()
}

// NewLogger creates a Logger that writes to both logs/app.log and stdout.
// The logs/ directory and app.log file are created if they do not exist.
// Returns nil if initialisation fails (caller should treat nil as fatal).
func NewLogger() *Logger {
	l := &Logger{}

	// Ensure the logs directory exists
	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		if err := os.Mkdir("logs", os.ModePerm); err != nil {
			slog.Error("Failed to create logs directory", "error", err)
			return nil
		}
	}

	// Open (or create) the log file in append mode
	f, err := os.OpenFile("logs/app.jsonl", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		slog.Error("Failed to open log file", "error", err)
		return nil
	}

	l.file = f
	l.FileLogger = slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{
		// AddSource: true,
		Level: slog.LevelDebug, // File always captures everything
	}))
	l.StdoutLogger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		// AddSource: true,
		Level: slog.LevelInfo, // Console level is overridden at server startup
	}))

	return l
}

// Close flushes and closes the underlying log file.
// Always call this with defer after creating a Logger.
func (l *Logger) Close() {
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			slog.Error("Failed to close log file", "error", err)
		}
	}
}

// Info logs at INFO level to both file and stdout.
func (l *Logger) Info(msg string, args ...any) {
	l.FileLogger.Info(msg, args...)
	l.StdoutLogger.Info(msg, args...)
}

// Warn logs at WARN level to both file and stdout.
func (l *Logger) Warn(msg string, args ...any) {
	l.FileLogger.Warn(msg, args...)
	l.StdoutLogger.Warn(msg, args...)
}

// Error logs at ERROR level to both file and stdout.
func (l *Logger) Error(msg string, args ...any) {
	l.FileLogger.Error(msg, args...)
	l.StdoutLogger.Error(msg, args...)
}

// Debug logs at DEBUG level to both file and stdout.
func (l *Logger) Debug(msg string, args ...any) {
	l.FileLogger.Debug(msg, args...)
	l.StdoutLogger.Debug(msg, args...)
}
