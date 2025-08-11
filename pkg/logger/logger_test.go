package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestLevel(t *testing.T) {
	// Test level ordering
	if LevelDebug >= LevelInfo {
		t.Error("LevelDebug should be less than LevelInfo")
	}
	if LevelInfo >= LevelWarn {
		t.Error("LevelInfo should be less than LevelWarn")
	}
	if LevelWarn >= LevelError {
		t.Error("LevelWarn should be less than LevelError")
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.Output == nil {
		t.Error("expected non-nil Output")
	}

	if opts.Level != LevelInfo {
		t.Errorf("expected default level to be LevelInfo, got %v", opts.Level)
	}

	if opts.Prefix != "" {
		t.Errorf("expected empty default prefix, got %s", opts.Prefix)
	}

	if !opts.ShowTimestamp {
		t.Error("expected ShowTimestamp to be true by default")
	}
}

func TestNew(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{
		Output:        &buf,
		Level:         LevelDebug,
		Prefix:        "TEST",
		ShowTimestamp: false,
	}

	logger := New(opts)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	// Test that it implements the Logger interface
	var _ Logger = logger
}

func TestDefault(t *testing.T) {
	logger := Default()
	if logger == nil {
		t.Fatal("expected non-nil default logger")
	}

	// Test that it implements the Logger interface
	var _ Logger = logger
}

func TestDefaultLogger_Debug(t *testing.T) {
	tests := []struct {
		name      string
		level     Level
		message   string
		args      []interface{}
		shouldLog bool
	}{
		{
			name:      "debug level logs debug",
			level:     LevelDebug,
			message:   "debug message",
			shouldLog: true,
		},
		{
			name:      "info level does not log debug",
			level:     LevelInfo,
			message:   "debug message",
			shouldLog: false,
		},
		{
			name:      "debug with formatting",
			level:     LevelDebug,
			message:   "debug %s %d",
			args:      []interface{}{"test", 42},
			shouldLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			opts := Options{
				Output:        &buf,
				Level:         tt.level,
				ShowTimestamp: false,
			}

			logger := New(opts).(*defaultLogger)
			logger.Debug(tt.message, tt.args...)

			output := buf.String()
			if tt.shouldLog {
				if output == "" {
					t.Error("expected log output but got none")
				}
				if !strings.Contains(output, "[DEBUG]") {
					t.Error("expected [DEBUG] prefix in output")
				}
				if !strings.Contains(output, tt.message) && len(tt.args) == 0 {
					t.Errorf("expected message %q in output %q", tt.message, output)
				}
			} else {
				if output != "" {
					t.Errorf("expected no output but got %q", output)
				}
			}
		})
	}
}

func TestDefaultLogger_Info(t *testing.T) {
	tests := []struct {
		name      string
		level     Level
		message   string
		shouldLog bool
	}{
		{
			name:      "debug level logs info",
			level:     LevelDebug,
			message:   "info message",
			shouldLog: true,
		},
		{
			name:      "info level logs info",
			level:     LevelInfo,
			message:   "info message",
			shouldLog: true,
		},
		{
			name:      "warn level does not log info",
			level:     LevelWarn,
			message:   "info message",
			shouldLog: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			opts := Options{
				Output:        &buf,
				Level:         tt.level,
				ShowTimestamp: false,
			}

			logger := New(opts).(*defaultLogger)
			logger.Info(tt.message)

			output := buf.String()
			if tt.shouldLog {
				if output == "" {
					t.Error("expected log output but got none")
				}
				if !strings.Contains(output, "[INFO]") {
					t.Error("expected [INFO] prefix in output")
				}
			} else {
				if output != "" {
					t.Errorf("expected no output but got %q", output)
				}
			}
		})
	}
}

func TestDefaultLogger_Warn(t *testing.T) {
	tests := []struct {
		name      string
		level     Level
		message   string
		shouldLog bool
	}{
		{
			name:      "debug level logs warn",
			level:     LevelDebug,
			message:   "warn message",
			shouldLog: true,
		},
		{
			name:      "warn level logs warn",
			level:     LevelWarn,
			message:   "warn message",
			shouldLog: true,
		},
		{
			name:      "error level does not log warn",
			level:     LevelError,
			message:   "warn message",
			shouldLog: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			opts := Options{
				Output:        &buf,
				Level:         tt.level,
				ShowTimestamp: false,
			}

			logger := New(opts).(*defaultLogger)
			logger.Warn(tt.message)

			output := buf.String()
			if tt.shouldLog {
				if output == "" {
					t.Error("expected log output but got none")
				}
				if !strings.Contains(output, "[WARN]") {
					t.Error("expected [WARN] prefix in output")
				}
			} else {
				if output != "" {
					t.Errorf("expected no output but got %q", output)
				}
			}
		})
	}
}

func TestDefaultLogger_Error(t *testing.T) {
	tests := []struct {
		name      string
		level     Level
		message   string
		shouldLog bool
	}{
		{
			name:      "debug level logs error",
			level:     LevelDebug,
			message:   "error message",
			shouldLog: true,
		},
		{
			name:      "error level logs error",
			level:     LevelError,
			message:   "error message",
			shouldLog: true,
		},
		// All levels should log errors since error is the highest level
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			opts := Options{
				Output:        &buf,
				Level:         tt.level,
				ShowTimestamp: false,
			}

			logger := New(opts).(*defaultLogger)
			logger.Error(tt.message)

			output := buf.String()
			if tt.shouldLog {
				if output == "" {
					t.Error("expected log output but got none")
				}
				if !strings.Contains(output, "[ERROR]") {
					t.Error("expected [ERROR] prefix in output")
				}
			}
		})
	}
}

func TestDefaultLogger_WithPrefix(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{
		Output:        &buf,
		Level:         LevelInfo,
		Prefix:        "BASE",
		ShowTimestamp: false,
	}

	logger := New(opts)
	prefixedLogger := logger.WithPrefix("SUB")

	prefixedLogger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "BASE SUB") {
		t.Errorf("expected prefixed output to contain 'BASE SUB', got %q", output)
	}
}

func TestDefaultLogger_WithPrefix_AutoSpace(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{
		Output:        &buf,
		Level:         LevelInfo,
		ShowTimestamp: false,
	}

	logger := New(opts)
	prefixedLogger := logger.WithPrefix("TEST")

	prefixedLogger.Info("message")

	output := buf.String()
	if !strings.Contains(output, "TEST [INFO]") {
		t.Errorf("expected output to contain 'TEST [INFO]', got %q", output)
	}
}

func TestDefaultLogger_SetLevel(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{
		Output:        &buf,
		Level:         LevelInfo,
		ShowTimestamp: false,
	}

	logger := New(opts).(*defaultLogger)

	// Initially should not log debug
	logger.Debug("debug message 1")
	if buf.String() != "" {
		t.Error("expected no output for debug at info level")
	}

	// Change level to debug
	logger.SetLevel(LevelDebug)
	logger.Debug("debug message 2")

	output := buf.String()
	if !strings.Contains(output, "debug message 2") {
		t.Error("expected debug message after setting level to debug")
	}
}

func TestDefaultLogger_PrefixFormatting(t *testing.T) {
	tests := []struct {
		name           string
		prefix         string
		expectedPrefix string
	}{
		{
			name:           "empty prefix",
			prefix:         "",
			expectedPrefix: "",
		},
		{
			name:           "prefix without space",
			prefix:         "TEST",
			expectedPrefix: "TEST ",
		},
		{
			name:           "prefix with space",
			prefix:         "TEST ",
			expectedPrefix: "TEST ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			opts := Options{
				Output:        &buf,
				Level:         LevelInfo,
				Prefix:        tt.prefix,
				ShowTimestamp: false,
			}

			logger := New(opts).(*defaultLogger)
			logger.Info("test")

			output := buf.String()
			if tt.expectedPrefix == "" {
				if strings.HasPrefix(output, " ") {
					t.Error("expected no leading space for empty prefix")
				}
			} else {
				if !strings.HasPrefix(output, tt.expectedPrefix) {
					t.Errorf("expected output to start with %q, got %q", tt.expectedPrefix, output)
				}
			}
		})
	}
}

func TestNoopLogger(t *testing.T) {
	logger := Noop()
	if logger == nil {
		t.Fatal("expected non-nil noop logger")
	}

	// Test that it implements the Logger interface
	var _ Logger = logger

	// Test that all methods can be called without panicking
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")
	logger.SetLevel(LevelDebug)

	prefixed := logger.WithPrefix("test")
	if prefixed != logger {
		t.Error("WithPrefix should return the same noop logger instance")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatBytes(tt.input)
			if result != tt.expected {
				t.Errorf("FormatBytes(%d) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{500, "500ns"},
		{1500, "1.50µs"},
		{1500000, "1.50ms"},
		{1500000000, "1.50s"},
		{2500000000, "2.50s"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatDuration(tt.input)
			if result != tt.expected {
				t.Errorf("FormatDuration(%d) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoggerWithTimestamp(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{
		Output:        &buf,
		Level:         LevelInfo,
		ShowTimestamp: true,
	}

	logger := New(opts)
	logger.Info("test with timestamp")

	output := buf.String()
	// Timestamp format will vary, but we can check it's not empty and has typical format
	if len(output) < 20 {
		t.Error("expected longer output with timestamp")
	}

	// Should still contain the log level and message
	if !strings.Contains(output, "[INFO]") {
		t.Error("expected [INFO] in timestamped output")
	}
	if !strings.Contains(output, "test with timestamp") {
		t.Error("expected message in timestamped output")
	}
}

func TestLoggerInterfaceCompliance(t *testing.T) {
	// Test that both implementations satisfy the Logger interface
	var logger1 Logger = Default()
	var logger2 Logger = Noop()

	if logger1 == nil {
		t.Error("default logger should not be nil")
	}

	if logger2 == nil {
		t.Error("noop logger should not be nil")
	}
}
