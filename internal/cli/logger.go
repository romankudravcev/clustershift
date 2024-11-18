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
	parent     *Logger // Add parent reference to Logger
}

type LoggerConfig struct {
	LogDir     string
	TimeFormat string
}

var DefaultConfig = LoggerConfig{
	LogDir:     "logs",
	TimeFormat: "2006-01-02 15:04:05",
}

var (
	OGLOgger *Logger
)

func SetOGLogger(l *Logger) {
	OGLOgger = l
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
	fmt.Print("\n")

	logger := &Logger{
		timeFormat: DefaultConfig.TimeFormat,
		parent:     parent,
	}

	if parent == nil {
		// Only create new file for root logger
		if err := os.MkdirAll(DefaultConfig.LogDir, 0755); err != nil {
			logger.handleError("Failed to create log directory", err)
			return logger
		}

		timestamp := time.Now().Format("2006-01-02_15-04-05")
		logPath := filepath.Join(DefaultConfig.LogDir, fmt.Sprintf("%s.log", timestamp))
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			logger.handleError("Failed to create log file", err)
			return logger
		}
		logger.file = file
	} else {
		// Child loggers use parent's file
		logger.file = parent.file
	}

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

func (l *Logger) Warning(message string, err error) error {
	if err == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if err.Error() == "" {
		message = fmt.Sprintf("%s", message)
	} else {
		message = fmt.Sprintf("%s: %v", message, err)
	}
	if l.status != nil && !l.closed {
		l.status.Fail(message)
		l.writeToFile(fmt.Sprintf("Failed: %s", message))
		l.close() // Automatically close after failure
	}

	return fmt.Errorf(message)
}
func (l *Logger) Fail(message string, err error) error {
	if err == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if err.Error() == "" {
		message = fmt.Sprintf("%s", message)
	} else {
		message = fmt.Sprintf("%s: %v", message, err)
	}
	if l.status != nil && !l.closed {
		l.status.Fail(message)
		l.writeToFile(fmt.Sprintf("Failed: %s", message))
		l.close() // Automatically close after failure
	}

	for {
		if l.parent == nil {
			break
		}
		l = l.parent
		l.Fail(l.status.message, fmt.Errorf(""))
		l.writeToFile(fmt.Sprintf("Failed: %s", message))
		l.close() // Automatically close after failure
	}

	return fmt.Errorf(message)
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

func LogToFile(message string) {
	if OGLOgger.file == nil {
		OGLOgger.handleError("Log file error", fmt.Errorf("attempted to write to nil file"))
		return
	}

	timestamp := time.Now().Format(OGLOgger.timeFormat)
	indent := ""
	if OGLOgger.status != nil && OGLOgger.status.parent != nil {
		indent = strings.Repeat("  ", OGLOgger.status.indentLevel)
	}

	logMessage := fmt.Sprintf("[%s]%s %s\n", timestamp, indent, message)

	if _, err := OGLOgger.file.WriteString(logMessage); err != nil {
		OGLOgger.handleError("Write error", err)
	}
}

func (l *Logger) close() {
	if !l.closed {
		l.closed = true
		// Only close the file if this is the root logger
		if l.parent == nil && l.file != nil {
			if err := l.file.Close(); err != nil {
				l.handleError("Close error", err)
			}
			l.file = nil
		}
	}
}

func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.close()
}
