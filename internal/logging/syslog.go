package logging

import (
	"fmt"
	"io"
	"log"
	"log/syslog"
	"os"
	"sync"
)

// SyslogLogger handles logging to syslog, files, or stdout
type SyslogLogger struct {
	writer     *syslog.Writer
	fileWriter *os.File
	enabled    bool
	logToFile  bool
	facility   syslog.Priority
	tag        string
	mu         sync.RWMutex
}

// SyslogConfig holds syslog configuration
type SyslogConfig struct {
	Enabled  bool
	Network  string // "tcp", "udp", "unix", or "" (leave empty for local syslog)
	Address  string // "localhost:514", "/dev/log", or "" (auto-detect)
	Tag      string
	Facility string // "LOG_LOCAL0" through "LOG_LOCAL7"
	FilePath string // Path to log file (overrides syslog if set)
}

var (
	globalSyslog *SyslogLogger
	once         sync.Once
)

// NewSyslogLogger creates a new syslog logger
func NewSyslogLogger(cfg SyslogConfig) (*SyslogLogger, error) {
	if !cfg.Enabled {
		return &SyslogLogger{enabled: false}, nil
	}

	logger := &SyslogLogger{
		enabled:  true,
		tag:      cfg.Tag,
		facility: parseFacility(cfg.Facility),
	}

	// Check for AISERVE_LOG_FILE environment variable
	if cfg.FilePath == "" {
		cfg.FilePath = os.Getenv("AISERVE_LOG_FILE")
	}

	// If FilePath is set, log to file instead of syslog
	if cfg.FilePath != "" {
		file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		logger.fileWriter = file
		logger.logToFile = true
		log.Printf("Logging to file: %s", cfg.FilePath)
		return logger, nil
	}

	// Otherwise, use syslog
	priority := logger.facility | syslog.LOG_INFO
	var writer *syslog.Writer
	var err error

	// Auto-detect /dev/log if address is empty
	if cfg.Address == "" {
		if _, err := os.Stat("/dev/log"); err == nil {
			cfg.Network = "unix"
			cfg.Address = "/dev/log"
		}
	}

	if cfg.Network == "" && cfg.Address == "" {
		// Local syslog (system default)
		writer, err = syslog.New(priority, cfg.Tag)
		if err != nil {
			log.Printf("Warning: Failed to connect to system syslog: %v", err)
			log.Println("Falling back to stdout logging")
			return &SyslogLogger{enabled: false}, nil
		}
	} else {
		// Remote or specific syslog
		writer, err = syslog.Dial(cfg.Network, cfg.Address, priority, cfg.Tag)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to syslog at %s://%s: %w", cfg.Network, cfg.Address, err)
		}
	}

	logger.writer = writer
	log.Printf("Logging to syslog: %s://%s (facility: %s)", cfg.Network, cfg.Address, cfg.Facility)
	return logger, nil
}

// Initialize initializes the global syslog logger
func Initialize(cfg SyslogConfig) error {
	var err error
	once.Do(func() {
		globalSyslog, err = NewSyslogLogger(cfg)
		if err == nil && globalSyslog.enabled {
			// Also set log output to syslog
			log.SetOutput(globalSyslog)
			log.SetFlags(0) // Syslog adds its own timestamp
		}
	})
	return err
}

// GetLogger returns the global syslog logger
func GetLogger() *SyslogLogger {
	return globalSyslog
}

// Write implements io.Writer for compatibility with standard log package
func (s *SyslogLogger) Write(p []byte) (n int, err error) {
	if !s.enabled {
		return os.Stdout.Write(p)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Log to file if file writer is set
	if s.logToFile && s.fileWriter != nil {
		return s.fileWriter.Write(p)
	}

	// Otherwise log to syslog
	if s.writer == nil {
		return os.Stdout.Write(p)
	}

	// Write to syslog as INFO
	err = s.writer.Info(string(p))
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// writeToOutput writes to the appropriate output (file or syslog)
func (s *SyslogLogger) writeToOutput(level, msg string) error {
	if !s.enabled {
		log.Printf("[%s] %s", level, msg)
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Write to file if file writer is set
	if s.logToFile && s.fileWriter != nil {
		_, err := fmt.Fprintf(s.fileWriter, "[%s] %s\n", level, msg)
		return err
	}

	// Otherwise write to syslog
	if s.writer == nil {
		log.Printf("[%s] %s", level, msg)
		return nil
	}

	switch level {
	case "EMERG":
		return s.writer.Emerg(msg)
	case "ALERT":
		return s.writer.Alert(msg)
	case "CRIT":
		return s.writer.Crit(msg)
	case "ERROR":
		return s.writer.Err(msg)
	case "WARN":
		return s.writer.Warning(msg)
	case "NOTICE":
		return s.writer.Notice(msg)
	case "INFO":
		return s.writer.Info(msg)
	case "DEBUG":
		return s.writer.Debug(msg)
	default:
		return s.writer.Info(msg)
	}
}

// Emergency logs an emergency message
func (s *SyslogLogger) Emergency(msg string) error {
	return s.writeToOutput("EMERG", msg)
}

// Alert logs an alert message
func (s *SyslogLogger) Alert(msg string) error {
	return s.writeToOutput("ALERT", msg)
}

// Critical logs a critical message
func (s *SyslogLogger) Critical(msg string) error {
	return s.writeToOutput("CRIT", msg)
}

// Error logs an error message
func (s *SyslogLogger) Error(msg string) error {
	return s.writeToOutput("ERROR", msg)
}

// Warning logs a warning message
func (s *SyslogLogger) Warning(msg string) error {
	return s.writeToOutput("WARN", msg)
}

// Notice logs a notice message
func (s *SyslogLogger) Notice(msg string) error {
	return s.writeToOutput("NOTICE", msg)
}

// Info logs an info message
func (s *SyslogLogger) Info(msg string) error {
	return s.writeToOutput("INFO", msg)
}

// Debug logs a debug message
func (s *SyslogLogger) Debug(msg string) error {
	return s.writeToOutput("DEBUG", msg)
}

// Infof logs a formatted info message
func (s *SyslogLogger) Infof(format string, args ...interface{}) error {
	return s.Info(fmt.Sprintf(format, args...))
}

// Errorf logs a formatted error message
func (s *SyslogLogger) Errorf(format string, args ...interface{}) error {
	return s.Error(fmt.Sprintf(format, args...))
}

// Warningf logs a formatted warning message
func (s *SyslogLogger) Warningf(format string, args ...interface{}) error {
	return s.Warning(fmt.Sprintf(format, args...))
}

// Debugf logs a formatted debug message
func (s *SyslogLogger) Debugf(format string, args ...interface{}) error {
	return s.Debug(fmt.Sprintf(format, args...))
}

// Close closes the syslog connection or file
func (s *SyslogLogger) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled {
		return nil
	}

	var errors []error

	if s.fileWriter != nil {
		if err := s.fileWriter.Close(); err != nil {
			errors = append(errors, fmt.Errorf("file writer: %w", err))
		}
		s.fileWriter = nil
	}

	if s.writer != nil {
		if err := s.writer.Close(); err != nil {
			errors = append(errors, fmt.Errorf("syslog writer: %w", err))
		}
		s.writer = nil
	}

	if len(errors) > 0 {
		return fmt.Errorf("close errors: %v", errors)
	}
	return nil
}

// GetMultiWriter returns an io.Writer that writes to both syslog/file and stdout
func (s *SyslogLogger) GetMultiWriter() io.Writer {
	if !s.enabled {
		return os.Stdout
	}
	return io.MultiWriter(s, os.Stdout)
}

// parseFacility converts facility string to syslog.Priority
func parseFacility(facility string) syslog.Priority {
	switch facility {
	case "LOG_LOCAL0":
		return syslog.LOG_LOCAL0
	case "LOG_LOCAL1":
		return syslog.LOG_LOCAL1
	case "LOG_LOCAL2":
		return syslog.LOG_LOCAL2
	case "LOG_LOCAL3":
		return syslog.LOG_LOCAL3
	case "LOG_LOCAL4":
		return syslog.LOG_LOCAL4
	case "LOG_LOCAL5":
		return syslog.LOG_LOCAL5
	case "LOG_LOCAL6":
		return syslog.LOG_LOCAL6
	case "LOG_LOCAL7":
		return syslog.LOG_LOCAL7
	case "LOG_USER":
		return syslog.LOG_USER
	case "LOG_DAEMON":
		return syslog.LOG_DAEMON
	default:
		return syslog.LOG_LOCAL0
	}
}

// Structured logging helpers

// LogRequest logs an HTTP request
func LogRequest(method, path, remoteAddr string, statusCode int, duration int64) {
	msg := fmt.Sprintf("method=%s path=%s remote=%s status=%d duration=%dms",
		method, path, remoteAddr, statusCode, duration)

	if globalSyslog != nil && globalSyslog.enabled {
		globalSyslog.Info(msg)
	} else {
		log.Println(msg)
	}
}

// LogError logs an error with context
func LogError(component, message string, err error) {
	msg := fmt.Sprintf("component=%s message=%s error=%v",
		component, message, err)

	if globalSyslog != nil && globalSyslog.enabled {
		globalSyslog.Error(msg)
	} else {
		log.Println(msg)
	}
}

// LogInfo logs an informational message with context
func LogInfo(component, message string) {
	msg := fmt.Sprintf("component=%s message=%s",
		component, message)

	if globalSyslog != nil && globalSyslog.enabled {
		globalSyslog.Info(msg)
	} else {
		log.Println(msg)
	}
}

// LogWarning logs a warning with context
func LogWarning(component, message string) {
	msg := fmt.Sprintf("component=%s message=%s",
		component, message)

	if globalSyslog != nil && globalSyslog.enabled {
		globalSyslog.Warning(msg)
	} else {
		log.Println(msg)
	}
}

// LogDebug logs a debug message with context
func LogDebug(component, message string) {
	msg := fmt.Sprintf("component=%s message=%s",
		component, message)

	if globalSyslog != nil && globalSyslog.enabled {
		globalSyslog.Debug(msg)
	} else {
		log.Println(msg)
	}
}
