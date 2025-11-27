package main

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// LogLevel represents the severity level of a log entry
type LogLevel string

const (
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp  string                 `json:"timestamp"`
	Level      string                 `json:"level"`
	RequestID  string                 `json:"request_id,omitempty"`
	Message    string                 `json:"message"`
	Duration   float64                `json:"duration_ms,omitempty"`
	StatusCode int                    `json:"status_code,omitempty"`
	Format     string                 `json:"format,omitempty"`
	Model      string                 `json:"model,omitempty"`
	Provider   string                 `json:"provider,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Streaming  bool                   `json:"streaming,omitempty"`
	Extra      map[string]interface{} `json:"extra,omitempty"`
}

// Logger provides structured JSON logging
type Logger struct {
	output io.Writer
	mu     sync.Mutex
}

// NewLogger creates a new Logger that writes to the specified output
func NewLogger(w io.Writer) *Logger {
	return &Logger{
		output: w,
	}
}

// Info logs an informational message
func (l *Logger) Info(entry LogEntry) {
	l.log(LogLevelInfo, entry)
}

// Warn logs a warning message
func (l *Logger) Warn(entry LogEntry) {
	l.log(LogLevelWarn, entry)
}

// Error logs an error message
func (l *Logger) Error(entry LogEntry) {
	l.log(LogLevelError, entry)
}

// log is the internal logging implementation
func (l *Logger) log(level LogLevel, entry LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Set timestamp and level
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	entry.Level = string(level)

	// Marshal to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		// Fallback: write error directly
		l.output.Write([]byte(`{"error":"failed to marshal log entry","level":"error"}` + "\n"))
		return
	}

	// Write to output
	l.output.Write(data)
	l.output.Write([]byte("\n"))
}

// InfoMsg is a convenience function for simple info messages
func (l *Logger) InfoMsg(message string) {
	l.Info(LogEntry{Message: message})
}

// WarnMsg is a convenience function for simple warning messages
func (l *Logger) WarnMsg(message string) {
	l.Warn(LogEntry{Message: message})
}

// ErrorMsg is a convenience function for simple error messages
func (l *Logger) ErrorMsg(message string) {
	l.Error(LogEntry{Message: message})
}
