package logger

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	globalLogger *logrus.Logger
)

// Config represents the configuration for the logger
type Config struct {
	Level        string
	File         string
	MaxSize      int
	MaxBackups   int
	MaxAge       int
	Compress     bool
	EnableStdout bool
}

// InitLogger initializes the global logger with the given configuration
func InitLogger(config Config) error {
	globalLogger = logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		// Default to info level if parsing fails
		level = logrus.InfoLevel
	}
	globalLogger.SetLevel(level)

	// Create log directory if it doesn't exist
	if config.File != "" {
		logDir := filepath.Dir(config.File)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}
	}

	// Configure output
	var writers []io.Writer

	// File output with rotation
	if config.File != "" {
		fileWriter := &lumberjack.Logger{
			Filename:   config.File,
			MaxSize:    config.MaxSize,    // megabytes
			MaxBackups: config.MaxBackups, // number of backups
			MaxAge:     config.MaxAge,     // days
			Compress:   config.Compress,   // compress old logs
		}
		writers = append(writers, fileWriter)
	}

	// Stdout output
	if config.EnableStdout {
		writers = append(writers, os.Stdout)
	}

	// Set multi-writer if needed
	if len(writers) > 0 {
		multiWriter := io.MultiWriter(writers...)
		globalLogger.SetOutput(multiWriter)
	}

	// Set formatter based on level
	if level == logrus.DebugLevel {
		// Use text formatter with colors for debug mode
		globalLogger.SetFormatter(&logrus.TextFormatter{
			ForceColors:     true,
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
			DisableColors:   false,
		})
	} else {
		// Use JSON formatter for production
		globalLogger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05Z",
		})
	}

	return nil
}

// GetLogger returns the global logger instance
func GetLogger() *logrus.Logger {
	if globalLogger == nil {
		// Initialize with default config if not initialized
		globalLogger = logrus.New()
		globalLogger.SetLevel(logrus.InfoLevel)
		globalLogger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}
	return globalLogger
}

// Convenience functions for logging

// Debug logs a message at debug level
func Debug(args ...interface{}) {
	GetLogger().Debug(args...)
}

// Info logs a message at info level
func Info(args ...interface{}) {
	GetLogger().Info(args...)
}

// Warn logs a message at warning level
func Warn(args ...interface{}) {
	GetLogger().Warn(args...)
}

// Error logs a message at error level
func Error(args ...interface{}) {
	GetLogger().Error(args...)
}

// Fatal logs a message at fatal level and exits
func Fatal(args ...interface{}) {
	GetLogger().Fatal(args...)
}

// Debugf logs a formatted message at debug level
func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

// Infof logs a formatted message at info level
func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

// Warnf logs a formatted message at warning level
func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}

// Errorf logs a formatted message at error level
func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

// Fatalf logs a formatted message at fatal level and exits
func Fatalf(format string, args ...interface{}) {
	GetLogger().Fatalf(format, args...)
}

// WithFields returns a logger entry with structured fields
func WithFields(fields logrus.Fields) *logrus.Entry {
	return GetLogger().WithFields(fields)
}

// WithField returns a logger entry with a single field
func WithField(key string, value interface{}) *logrus.Entry {
	return GetLogger().WithField(key, value)
}
