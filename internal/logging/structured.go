package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/google/uuid"
)

type LogLevel string

const (
	DEBUG LogLevel = "DEBUG"
	INFO  LogLevel = "INFO"
	WARN  LogLevel = "WARN"
	ERROR LogLevel = "ERROR"
	FATAL LogLevel = "FATAL"
)

type StructuredLogger struct {
	level       LogLevel
	serviceName string
	output      *os.File
}

type LogEntry struct {
	Timestamp   string                 `json:"timestamp"`
	Level       LogLevel               `json:"level"`
	Service     string                 `json:"service"`
	Message     string                 `json:"message"`
	RequestID   string                 `json:"request_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	Method      string                 `json:"method,omitempty"`
	Path        string                 `json:"path,omitempty"`
	StatusCode  int                    `json:"status_code,omitempty"`
	Duration    int64                  `json:"duration_ms,omitempty"`
	Error       string                 `json:"error,omitempty"`
	StackTrace  string                 `json:"stack_trace,omitempty"`
	Fields      map[string]interface{} `json:"fields,omitempty"`
	File        string                 `json:"file,omitempty"`
	Line        int                    `json:"line,omitempty"`
}

var defaultStructuredLogger *StructuredLogger

func InitStructuredLogger(serviceName string, level LogLevel) *StructuredLogger {
	logger := &StructuredLogger{
		level:       level,
		serviceName: serviceName,
		output:      os.Stdout,
	}
	defaultStructuredLogger = logger
	return logger
}

func GetStructuredLogger() *StructuredLogger {
	if defaultStructuredLogger == nil {
		return InitStructuredLogger("gpuproxy", INFO)
	}
	return defaultStructuredLogger
}

func (l *StructuredLogger) shouldLog(level LogLevel) bool {
	levels := map[LogLevel]int{
		DEBUG: 0,
		INFO:  1,
		WARN:  2,
		ERROR: 3,
		FATAL: 4,
	}
	return levels[level] >= levels[l.level]
}

func (l *StructuredLogger) log(level LogLevel, message string, fields map[string]interface{}) {
	if !l.shouldLog(level) {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level,
		Service:   l.serviceName,
		Message:   message,
		Fields:    fields,
	}

	// Add file and line number for ERROR and FATAL
	if level == ERROR || level == FATAL {
		_, file, line, ok := runtime.Caller(2)
		if ok {
			entry.File = file
			entry.Line = line
		}
	}

	// Extract common fields
	if fields != nil {
		if reqID, ok := fields["request_id"].(string); ok {
			entry.RequestID = reqID
		}
		if userID, ok := fields["user_id"].(string); ok {
			entry.UserID = userID
		}
		if method, ok := fields["method"].(string); ok {
			entry.Method = method
		}
		if path, ok := fields["path"].(string); ok {
			entry.Path = path
		}
		if status, ok := fields["status_code"].(int); ok {
			entry.StatusCode = status
		}
		if err, ok := fields["error"].(error); ok {
			entry.Error = err.Error()
		} else if errStr, ok := fields["error"].(string); ok {
			entry.Error = errStr
		}
		if duration, ok := fields["duration"].(time.Duration); ok {
			entry.Duration = duration.Milliseconds()
		}
	}

	// Marshal to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		// Fallback to stderr if JSON marshaling fails
		fmt.Fprintf(os.Stderr, "Failed to marshal log entry: %v\n", err)
		return
	}

	fmt.Fprintln(l.output, string(data))

	if level == FATAL {
		os.Exit(1)
	}
}

func (l *StructuredLogger) Debug(message string, fields map[string]interface{}) {
	l.log(DEBUG, message, fields)
}

func (l *StructuredLogger) Info(message string, fields map[string]interface{}) {
	l.log(INFO, message, fields)
}

func (l *StructuredLogger) Warn(message string, fields map[string]interface{}) {
	l.log(WARN, message, fields)
}

func (l *StructuredLogger) Error(message string, fields map[string]interface{}) {
	l.log(ERROR, message, fields)
}

func (l *StructuredLogger) Fatal(message string, fields map[string]interface{}) {
	l.log(FATAL, message, fields)
}

// Convenience functions
func Debug(message string, fields map[string]interface{}) {
	GetStructuredLogger().Debug(message, fields)
}

func Info(message string, fields map[string]interface{}) {
	GetStructuredLogger().Info(message, fields)
}

func Warn(message string, fields map[string]interface{}) {
	GetStructuredLogger().Warn(message, fields)
}

func Error(message string, fields map[string]interface{}) {
	GetStructuredLogger().Error(message, fields)
}

func Fatal(message string, fields map[string]interface{}) {
	GetStructuredLogger().Fatal(message, fields)
}

// Request ID middleware helper
type contextKey string

const RequestIDKey contextKey = "request_id"

func NewRequestID() string {
	return uuid.New().String()
}
