// Package logger provides structured logging configuration for clibot.
package logger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	globalLogger *logrus.Logger
)

// ANSI 256-color palette for an artistic CLI experience
// Using \033[38;5;Nm for foreground colors
const (
	colorReset = "\033[0m"
	colorBold  = "\033[1m"

	// Level & Metadata
	colorTime  = "\033[38;5;242m" // Gray
	colorInfo  = "\033[38;5;75m"  // Sky Blue
	colorWarn  = "\033[38;5;214m" // Orange
	colorError = "\033[38;5;196m" // Bright Red
	colorDebug = "\033[38;5;239m" // Deep Gray

	// Semantic Keywords
	colorSuccess = "\033[38;5;48m"  // Spring Green
	colorNeutral = "\033[38;5;250m" // Silver

	// Field Keys (Artistic Palette)
	clrSession  = "\033[38;5;120m" // Lime Green
	clrPlatform = "\033[38;5;39m"  // Deep Sky Blue
	clrUser     = "\033[38;5;170m" // Hot Pink/Plum
	clrCmd      = "\033[38;5;220m" // Gold/Yellow
	clrAction   = "\033[38;5;147m" // Light Purple
	clrMsg      = "\033[38;5;44m"  // Turquoise
	clrErrorKey = "\033[38;5;160m" // Crimson
	clrDefault  = "\033[38;5;37m"  // Teal
)

// OpenClawFormatter produces colorful, high-signal CLI output.
type OpenClawFormatter struct{}

// Format implements the logrus.Formatter interface
func (f *OpenClawFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	// 1. Timestamp (Low-key Gray)
	fmt.Fprintf(b, "%s[%s]%s ", colorTime, entry.Time.Format("15:04:05"), colorReset)

	// 2. Level Icon & Label
	var levelColor, levelText, icon string
	switch entry.Level {
	case logrus.InfoLevel:
		levelColor, levelText, icon = colorInfo, "INFO", "ℹ️ "
	case logrus.WarnLevel:
		levelColor, levelText, icon = colorWarn, "WARN", "⚠️ "
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		levelColor, levelText, icon = colorError, "ERRO", "❌"
	case logrus.DebugLevel, logrus.TraceLevel:
		levelColor, levelText, icon = colorDebug, "DEBU", "🔍"
	default:
		levelColor, levelText, icon = colorNeutral, "LOG ", "📝"
	}
	fmt.Fprintf(b, "%s%s %s%s%s ", levelColor, icon, colorBold, levelText, colorReset)

	// 3. Message with Semantic Highlighting
	msg := entry.Message
	lowerMsg := strings.ToLower(msg)
	if strings.Contains(lowerMsg, "start") || strings.Contains(lowerMsg, "success") || strings.Contains(lowerMsg, "initialized") {
		msg = colorSuccess + msg + colorReset
	} else if strings.Contains(lowerMsg, "stop") || strings.Contains(lowerMsg, "close") || strings.Contains(lowerMsg, "disconnect") {
		msg = colorWarn + msg + colorReset
	} else if strings.Contains(lowerMsg, "fail") || strings.Contains(lowerMsg, "error") {
		msg = colorError + msg + colorReset
	}
	fmt.Fprintf(b, "%-35s ", msg)

	// 4. Fields with Artistic Key Color Mapping
	if len(entry.Data) > 0 {
		keys := make([]string, 0, len(entry.Data))
		for k := range entry.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := entry.Data[k]
			valStr := fmt.Sprintf("%v", v)

			var kClr string
			switch k {
			case "session":
				kClr = clrSession
			case "platform":
				kClr = clrPlatform
			case "user", "user_id", "username":
				kClr = clrUser
			case "command", "cmd":
				kClr = clrCmd
			case "action", "event", "state":
				kClr = clrAction
			case "content", "msg", "input":
				kClr = clrMsg
			case "error", "panic":
				kClr = clrErrorKey
			default:
				kClr = clrDefault
			}
			// key:value formatting
			fmt.Fprintf(b, " %s%s%s:%s%s%s", kClr, k, colorReset, colorBold, valStr, colorReset)
		}
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}

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

	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	globalLogger.SetLevel(level)

	if config.File != "" {
		logDir := filepath.Dir(config.File)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}
	}

	var writers []io.Writer
	if config.File != "" {
		fileWriter := &lumberjack.Logger{
			Filename:   config.File,
			MaxSize:    config.MaxSize,
			MaxBackups: config.MaxBackups,
			MaxAge:     config.MaxAge,
			Compress:   config.Compress,
		}
		writers = append(writers, fileWriter)
	}

	if config.EnableStdout {
		writers = append(writers, os.Stdout)
	}

	if len(writers) > 0 {
		globalLogger.SetOutput(io.MultiWriter(writers...))
	}

	globalLogger.SetFormatter(&OpenClawFormatter{})
	return nil
}

// GetLogger returns the global logger instance
func GetLogger() *logrus.Logger {
	if globalLogger == nil {
		globalLogger = logrus.New()
		globalLogger.SetLevel(logrus.InfoLevel)
		globalLogger.SetFormatter(&OpenClawFormatter{})
	}
	return globalLogger
}

// Convenience functions
func Debug(args ...interface{}) { GetLogger().Debug(args...) }
func Info(args ...interface{}) { GetLogger().Info(args...) }
func Warn(args ...interface{}) { GetLogger().Warn(args...) }
func Error(args ...interface{}) { GetLogger().Error(args...) }
func Fatal(args ...interface{}) { GetLogger().Fatal(args...) }

func Debugf(format string, args ...interface{}) { GetLogger().Debugf(format, args...) }
func Infof(format string, args ...interface{}) { GetLogger().Infof(format, args...) }
func Warnf(format string, args ...interface{}) { GetLogger().Warnf(format, args...) }
func Errorf(format string, args ...interface{}) { GetLogger().Errorf(format, args...) }
func Fatalf(format string, args ...interface{}) { GetLogger().Fatalf(format, args...) }

func WithFields(fields logrus.Fields) *logrus.Entry { return GetLogger().WithFields(fields) }
func WithField(key string, value interface{}) *logrus.Entry { return GetLogger().WithField(key, value) }
