package logger

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitLogger(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config with file",
			config: Config{
				Level:        "info",
				File:         filepath.Join(os.TempDir(), "clibot-test.log"),
				MaxSize:      1,
				MaxBackups:   1,
				MaxAge:       1,
				Compress:     false,
				EnableStdout: false,
			},
			wantErr: false,
		},
		{
			name: "valid config with stdout only",
			config: Config{
				Level:        "debug",
				EnableStdout: true,
			},
			wantErr: false,
		},
		{
			name: "valid config with both file and stdout",
			config: Config{
				Level:        "warn",
				File:         filepath.Join(os.TempDir(), "clibot-test.log"),
				EnableStdout: true,
			},
			wantErr: false,
		},
		{
			name: "invalid log level defaults to info",
			config: Config{
				Level:        "invalid",
				EnableStdout: true,
			},
			wantErr: false,
		},
		{
			name: "empty config",
			config: Config{
				Level:        "info",
				EnableStdout: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := InitLogger(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify logger was initialized
				logger := GetLogger()
				assert.NotNil(t, logger)
			}

			// Clean up test log file
			if tt.config.File != "" {
				os.Remove(tt.config.File)
			}
		})
	}
}

func TestInitLogger_CreatesLogDirectory(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "clibot-test-logs")
	logFile := filepath.Join(tmpDir, "test.log")

	config := Config{
		Level:        "info",
		File:         logFile,
		EnableStdout: false,
	}

	err := InitLogger(config)
	require.NoError(t, err)

	// Verify directory was created
	info, err := os.Stat(tmpDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Clean up
	os.RemoveAll(tmpDir)
}

func TestGetLogger(t *testing.T) {
	// Test getting uninitialized logger
	logger := GetLogger()
	assert.NotNil(t, logger)
	assert.Equal(t, logrus.InfoLevel, logger.GetLevel())
}

func TestGetLogger_ReturnsSameInstance(t *testing.T) {
	logger1 := GetLogger()
	logger2 := GetLogger()
	assert.Same(t, logger1, logger2)
}

func TestLogFunctions(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Initialize logger
	config := Config{
		Level:        "info",
		EnableStdout: true,
	}
	err := InitLogger(config)
	require.NoError(t, err)

	// Test log functions
	Debug("debug message")
	Info("info message")
	Warn("warn message")
	Error("error message")

	// Close writer and restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	w.Close()

	output := buf.String()
	assert.Contains(t, output, "info message")
	assert.Contains(t, output, "warn message")
	assert.Contains(t, output, "error message")
	// Debug message should not appear with info level
	assert.NotContains(t, output, "debug message")
}

func TestLogFormattedFunctions(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Initialize logger
	config := Config{
		Level:        "info",
		EnableStdout: true,
	}
	err := InitLogger(config)
	require.NoError(t, err)

	// Test formatted functions
	Debugf("debug %s", "message")
	Infof("info %s", "message")
	Warnf("warn %s", "message")
	Errorf("error %s", "message")

	// Close writer and restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	w.Close()

	output := buf.String()
	assert.Contains(t, output, "info message")
	assert.Contains(t, output, "warn message")
	assert.Contains(t, output, "error message")
}

func TestWithFields(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Initialize logger
	config := Config{
		Level:        "info",
		EnableStdout: true,
	}
	err := InitLogger(config)
	require.NoError(t, err)

	// Test WithFields
	WithFields(logrus.Fields{
		"user": "alice",
		"action": "login",
	}).Info("User action")

	// Test WithField
	WithField("key", "value").Info("Single field")

	// Close writer and restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	w.Close()

	output := buf.String()
	assert.Contains(t, output, "alice")
	assert.Contains(t, output, "login")
	assert.Contains(t, output, "value")
}

func TestLogLevelSetting(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected logrus.Level
	}{
		{"debug level", "debug", logrus.DebugLevel},
		{"info level", "info", logrus.InfoLevel},
		{"warn level", "warn", logrus.WarnLevel},
		{"error level", "error", logrus.ErrorLevel},
		{"invalid level defaults to info", "invalid", logrus.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Level:        tt.level,
				EnableStdout: false,
			}
			err := InitLogger(config)
			require.NoError(t, err)

			logger := GetLogger()
			assert.Equal(t, tt.expected, logger.GetLevel())
		})
	}
}

func TestFormatterSetting(t *testing.T) {
	// Test debug mode uses text formatter
	config := Config{
		Level:        "debug",
		EnableStdout: false,
	}
	err := InitLogger(config)
	require.NoError(t, err)

	logger := GetLogger()
	formatter := logger.Formatter
	assert.IsType(t, &logrus.TextFormatter{}, formatter)

	// Test production mode uses JSON formatter
	config = Config{
		Level:        "info",
		EnableStdout: false,
	}
	err = InitLogger(config)
	require.NoError(t, err)

	logger = GetLogger()
	formatter = logger.Formatter
	assert.IsType(t, &logrus.JSONFormatter{}, formatter)
}

func TestInitLogger_WithCompression(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "clibot-compress-test.log")

	config := Config{
		Level:      "info",
		File:       tmpFile,
		MaxSize:    1,
		MaxBackups: 1,
		MaxAge:     1,
		Compress:   true,
	}

	err := InitLogger(config)
	assert.NoError(t, err)

	// Clean up
	os.Remove(tmpFile)
}

func TestFatal_LogFunction(t *testing.T) {
	// Note: Fatal() calls os.Exit(1), so we can't test it directly
	// We can only verify that the function exists and has the right signature
	logger := GetLogger()
	assert.NotNil(t, logger)

	// Just verify the function can be referenced
	_ = logger.Fatal
}

func TestFatalf_LogFunction(t *testing.T) {
	// Note: Fatalf() calls os.Exit(1), so we can't test it directly
	// We can only verify that the function exists and has the right signature
	logger := GetLogger()
	assert.NotNil(t, logger)

	// Just verify the function can be referenced
	_ = logger.Fatalf
}

func TestInitLogger_MultipleWriters(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	tmpFile := filepath.Join(os.TempDir(), "clibot-multi-writer.log")

	config := Config{
		Level:        "info",
		File:         tmpFile,
		EnableStdout: true,
	}

	err := InitLogger(config)
	assert.NoError(t, err)

	// Test that logging works
	Info("test message")

	// Close writer and restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Clean up
	os.Remove(tmpFile)
}
