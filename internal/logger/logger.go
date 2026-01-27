package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Level represents log level
type Level string

const (
	LevelDebug   Level = "DEBUG"
	LevelInfo    Level = "INFO"
	LevelWarning Level = "WARNING"
	LevelError   Level = "ERROR"
	LevelSuccess Level = "SUCCESS"
)

// Entry represents a log entry
type Entry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     Level     `json:"level"`
	Message   string    `json:"message"`
}

// Logger handles logging to console and file
type Logger struct {
	file    *os.File
	verbose bool
	debug   bool
}

// NewLogger creates a new logger
func NewLogger(logFilePath string, verbose, debug bool) (*Logger, error) {
	var file *os.File
	var err error

	if logFilePath != "" {
		file, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
	}

	return &Logger{
		file:    file,
		verbose: verbose,
		debug:   debug,
	}, nil
}

// Close closes the log file
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// log writes a log entry
func (l *Logger) log(level Level, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)

	entry := Entry{
		Timestamp: time.Now().UTC(),
		Level:     level,
		Message:   message,
	}

	// Write to file as JSON
	if l.file != nil {
		data, _ := json.Marshal(entry)
		l.file.Write(data)
		l.file.Write([]byte("\n"))
	}

	// Write to console with formatting
	l.writeConsole(level, message)
}

// writeConsole writes formatted output to console
func (l *Logger) writeConsole(level Level, message string) {
	var prefix string
	var color string

	switch level {
	case LevelDebug:
		if !l.debug {
			return
		}
		prefix = "[DEBUG]"
		color = "\033[36m" // Cyan
	case LevelInfo:
		prefix = "[INFO]"
		color = "\033[34m" // Blue
	case LevelWarning:
		prefix = "[WARN]"
		color = "\033[33m" // Yellow
	case LevelError:
		prefix = "[ERROR]"
		color = "\033[31m" // Red
	case LevelSuccess:
		prefix = "[✓]"
		color = "\033[32m" // Green
	}

	reset := "\033[0m"
	fmt.Printf("%s%s%s %s\n", color, prefix, reset, message)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

// Warning logs a warning message
func (l *Logger) Warning(format string, args ...interface{}) {
	l.log(LevelWarning, format, args...)
}

// Warn is an alias for Warning
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LevelWarning, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

// Success logs a success message
func (l *Logger) Success(format string, args ...interface{}) {
	l.log(LevelSuccess, format, args...)
}
