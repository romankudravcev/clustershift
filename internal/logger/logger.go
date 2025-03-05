package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// LogLevel represents the severity of log messages
type LogLevel int

// Predefined log levels in increasing order of severity
const (
	DEBUG LogLevel = iota
	INFO
	WARNING
	ERROR
)

// Logger provides a thread-safe logging mechanism with file and console output
type Logger struct {
	mu           sync.Mutex
	currentLevel LogLevel
	logFile      *os.File
}

// Predefined styles for different log levels
var (
	PrimaryStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true) // Blue for primary/info logs
	WarningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true) // Yellow for warnings
	ErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true) // Red for errors
)

// Singleton logger instance
var (
	defaultLogger *Logger
	initOnce      sync.Once
	globalMutex   sync.Mutex
)

// DefaultLogLevel sets the default logging level if not specified
var DefaultLogLevel = INFO

// Init creates a new Logger instance with the specified log level
//
// It performs the following actions:
// 1. Creates a 'logs' directory if it doesn't exist
// 2. Generates a log file with a timestamp in the filename
// 3. Configures the logger with the specified log level
//
// Parameters:
//   - level: Minimum log level to record
//
// Returns:
//   - *Logger: Configured logger instance
//   - error: Any error encountered during initialization
func Init(level LogLevel) (*Logger, error) {
	// Ensure logs directory exists
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Create timestamped log file
	filename := filepath.Join(logsDir, fmt.Sprintf("app_%s.log", time.Now().Format("2006-01-02_15-04-05")))
	logFile, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %v", err)
	}

	return &Logger{
		currentLevel: level,
		logFile:      logFile,
	}, nil
}

// Close safely closes the log file
//
// Returns an error if file closure fails
func (l *Logger) Close() error {
	return l.logFile.Close()
}

// SetLevel dynamically changes the current log level
//
// # This method is thread-safe and allows runtime log level adjustment
//
// Parameters:
//   - level: New log level to set
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.currentLevel = level
}

// log is an internal method to handle logging across console and file
//
// It manages:
// 1. Log level filtering
// 2. Thread-safe logging
// 3. Formatting log entries
// 4. Writing to console and file
//
// Parameters:
//   - level: Log level of the message
//   - style: Lipgloss style for formatting
//   - message: Log message content
func (l *Logger) log(level LogLevel, style lipgloss.Style, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Generate log entry with timestamp and level
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Plain text for file logging (always write)
	fileLogEntry := fmt.Sprintf("%s %s %s\n",
		timestamp,
		getLevelPrefix(level),
		message,
	)

	// Write to log file if available (unconditional)
	if l.logFile != nil {
		l.logFile.WriteString(fileLogEntry)
		l.logFile.Sync() // Ensure immediate write
	}

	// Console output with styling and level filtering
	if level >= l.currentLevel {
		consoleLogEntry := fmt.Sprintf("%s %s %s\n",
			timestamp,
			PrimaryStyle.Render(getLevelPrefix(level)),
			style.Render(message),
		)
		fmt.Print(consoleLogEntry)
	}
}

// getLevelPrefix converts LogLevel to a string representation
//
// Returns a standardized prefix for each log level
func getLevelPrefix(level LogLevel) string {
	switch level {
	case DEBUG:
		return "[DEBUG]"
	case INFO:
		return "[INFO]"
	case WARNING:
		return "[WARNING]"
	case ERROR:
		return "[ERROR]"
	default:
		return "[UNKNOWN]"
	}
}

// LogDebug logs a debug-level message
func (l *Logger) LogDebug(message string) {
	l.log(DEBUG, PrimaryStyle, message)
}

// LogInfo logs an info-level message
func (l *Logger) LogInfo(message string) {
	l.log(INFO, PrimaryStyle, message)
}

// LogWarning logs a warning-level message
func (l *Logger) LogWarning(message string) {
	l.log(WARNING, WarningStyle, message)
}

// LogError logs an error-level message
func (l *Logger) LogError(message string) {
	l.log(ERROR, ErrorStyle, message)
}

// EnsureLogger initializes the global logger if not already initialized
//
// # This function is safe to call multiple times and ensures only one initialization occurs
//
// Parameters:
//   - level (optional): Log level to set. Uses DefaultLogLevel if not provided
//
// Returns:
//   - error: Any error encountered during logger initialization
func EnsureLogger(level ...LogLevel) error {
	var err error
	initOnce.Do(func() {
		// Determine log level
		logLevel := DefaultLogLevel
		if len(level) > 0 {
			logLevel = level[0]
		}

		// Initialize the default logger
		defaultLogger, err = Init(logLevel)
	})
	return err
}

// GetLogger returns the global logger instance
//
// # If no logger has been initialized, it initializes one with the default log level
//
// Returns:
//   - *Logger: The global logger instance
//   - error: Any error encountered during initialization
func GetLogger() (*Logger, error) {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	if defaultLogger == nil {
		if err := EnsureLogger(); err != nil {
			return nil, fmt.Errorf("failed to initialize default logger: %v", err)
		}
	}
	return defaultLogger, nil
}

// Global logging convenience methods

// Debug logs a debug-level message using the global logger
func Debug(message string) {
	logger, err := GetLogger()
	if err != nil {
		fmt.Printf("Failed to get logger: %v\n", err)
		return
	}
	logger.LogDebug(message)
}

// Info logs an info-level message using the global logger
func Info(message string) {
	logger, err := GetLogger()
	if err != nil {
		fmt.Printf("Failed to get logger: %v\n", err)
		return
	}
	logger.LogInfo(message)
}

// Warning logs a warning-level message using the global logger
func Warning(message string) {
	logger, err := GetLogger()
	if err != nil {
		fmt.Printf("Failed to get logger: %v\n", err)
		return
	}
	logger.LogWarning(message)
}

// Error logs an error-level message using the global logger
func Error(message string) {
	logger, err := GetLogger()
	if err != nil {
		fmt.Printf("Failed to get logger: %v\n", err)
		return
	}
	logger.LogError(message)
}

// Close releases resources associated with the global logger
func Close() error {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	if defaultLogger != nil {
		return defaultLogger.Close()
	}
	return nil
}

// SetLevel changes the log level of the global logger
func SetLevel(level LogLevel) error {
	logger, err := GetLogger()
	if err != nil {
		return err
	}
	logger.SetLevel(level)
	return nil
}
