package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Logger struct {
	status     *Status
	file       *os.File
	mu         sync.Mutex
	timeFormat string
	closed     bool
}

type LoggerConfig struct {
	LogDir     string
	TimeFormat string
}

var DefaultConfig = LoggerConfig{
	LogDir:     "logs",
	TimeFormat: "2006-01-02 15:04:05",
}

func (l *Logger) handleError(message string, err error) {
	errorMsg := fmt.Sprintf("%s: %v", message, err)
	if l.status != nil {
		l.status.Fail(errorMsg)
	} else {
		fmt.Fprintf(os.Stderr, "Logger error: %s\n", errorMsg)
	}
}

func NewLogger(message string, parent *Logger) *Logger {
	config := &DefaultConfig

	logger := &Logger{
		timeFormat: config.TimeFormat,
	}

	if err := os.MkdirAll(config.LogDir, 0755); err != nil {
		logger.handleError("Failed to create log directory", err)
		return logger
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logPath := filepath.Join(config.LogDir, fmt.Sprintf("%s.log", timestamp))
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		logger.handleError("Failed to create log file", err)
		return logger
	}
	logger.file = file

	var parentStatus *Status
	if parent != nil {
		parentStatus = parent.status
	}
	logger.status = Start(message, parentStatus)

	logger.writeToFile(fmt.Sprintf("Started: %s", message))
	return logger
}

func (l *Logger) Log(message string) *Logger {
	child := NewLogger(message, l)
	return child
}

func (l *Logger) Success(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.status != nil && !l.closed {
		l.status.Success(message)
		l.writeToFile(fmt.Sprintf("Success: %s", message))
		l.close() // Automatically close after success
	}
}

func (l *Logger) Fail(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.status != nil && !l.closed {
		l.status.Fail(message)
		l.writeToFile(fmt.Sprintf("Failed: %s", message))
		l.close() // Automatically close after failure
	}
}

func (l *Logger) writeToFile(message string) {
	if l.file == nil {
		l.handleError("Log file error", fmt.Errorf("attempted to write to nil file"))
		return
	}

	timestamp := time.Now().Format(l.timeFormat)
	indent := ""
	if l.status != nil && l.status.parent != nil {
		indent = strings.Repeat("  ", l.status.indentLevel)
	}

	logMessage := fmt.Sprintf("[%s]%s %s\n", timestamp, indent, message)

	if _, err := l.file.WriteString(logMessage); err != nil {
		l.handleError("Write error", err)
	}
}

// private close method
func (l *Logger) close() {
	if !l.closed && l.file != nil {
		if err := l.file.Close(); err != nil {
			l.handleError("Close error", err)
		}
		l.file = nil
		l.closed = true
	}
}

// Close remains public for deferred calls that don't end in Success/Fail
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.close()
}
