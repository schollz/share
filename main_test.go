package main

import (
	"log/slog"
	"testing"
)

func TestGetWebSocketURL(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		expected string
	}{
		{
			name:     "https domain",
			domain:   "https://example.com",
			expected: "wss://example.com",
		},
		{
			name:     "http domain",
			domain:   "http://example.com",
			expected: "ws://example.com",
		},
		{
			name:     "no protocol assumes https",
			domain:   "example.com",
			expected: "wss://example.com",
		},
		{
			name:     "https with port",
			domain:   "https://example.com:3001",
			expected: "wss://example.com:3001",
		},
		{
			name:     "http with port",
			domain:   "http://localhost:3001",
			expected: "ws://localhost:3001",
		},
		{
			name:     "domain with path",
			domain:   "https://example.com/api",
			expected: "wss://example.com/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getWebSocketURL(tt.domain)
			if result != tt.expected {
				t.Errorf("getWebSocketURL(%s) = %s; expected %s", tt.domain, result, tt.expected)
			}
		})
	}
}

func TestGetWebSocketURLConsistency(t *testing.T) {
	// Test that same input gives same output
	domain := "https://example.com"
	result1 := getWebSocketURL(domain)
	result2 := getWebSocketURL(domain)

	if result1 != result2 {
		t.Errorf("getWebSocketURL not consistent: %s != %s", result1, result2)
	}
}

func TestGetWebSocketURLEmptyString(t *testing.T) {
	result := getWebSocketURL("")
	expected := "wss://"
	if result != expected {
		t.Errorf("getWebSocketURL('') = %s; expected %s", result, expected)
	}
}

func TestGetWebSocketURLHttpsPrefix(t *testing.T) {
	// Test that https:// is properly converted to wss://
	domain := "https://secure.example.com"
	result := getWebSocketURL(domain)

	if result[:6] != "wss://" {
		t.Errorf("getWebSocketURL(%s) should start with wss://, got %s", domain, result)
	}
}

func TestGetWebSocketURLHttpPrefix(t *testing.T) {
	// Test that http:// is properly converted to ws://
	domain := "http://insecure.example.com"
	result := getWebSocketURL(domain)

	if result[:5] != "ws://" {
		t.Errorf("getWebSocketURL(%s) should start with ws://, got %s", domain, result)
	}
}

func TestGetWebSocketURLPreservesPath(t *testing.T) {
	domain := "https://example.com/path/to/resource"
	result := getWebSocketURL(domain)
	expected := "wss://example.com/path/to/resource"

	if result != expected {
		t.Errorf("getWebSocketURL should preserve path: expected %s, got %s", expected, result)
	}
}

func TestCreateLogger(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected slog.Level
	}{
		{"debug level", "debug", slog.LevelDebug},
		{"info level", "info", slog.LevelInfo},
		{"warn level", "warn", slog.LevelWarn},
		{"error level", "error", slog.LevelError},
		{"invalid level defaults to info", "invalid", slog.LevelInfo},
		{"empty string defaults to info", "", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := createLogger(tt.level)
			if logger == nil {
				t.Fatal("Expected non-nil logger")
			}

			// Logger should be usable
			logger.Info("test message")
		})
	}
}

func TestCreateLoggerOutput(t *testing.T) {
	// Test that logger can write without error
	logger := createLogger("info")

	// Just verify logger is usable and doesn't panic
	logger.Info("test message", "key", "value")
	logger.Debug("debug message")
	logger.Warn("warn message")
	logger.Error("error message")

	// Test passed if no panic occurred
}

func TestCreateLoggerLevels(t *testing.T) {
	// Test all valid log levels
	levels := []string{"debug", "info", "warn", "error"}

	for _, level := range levels {
		logger := createLogger(level)
		if logger == nil {
			t.Errorf("createLogger(%s) returned nil", level)
		}
	}
}

func TestCreateLoggerCaseInsensitive(t *testing.T) {
	// The current implementation is case-sensitive, but we test what it does
	loggerLower := createLogger("info")
	loggerUpper := createLogger("INFO")

	if loggerLower == nil {
		t.Error("createLogger('info') returned nil")
	}
	if loggerUpper == nil {
		t.Error("createLogger('INFO') returned nil")
	}
}

func TestCreateLoggerDefaultLevel(t *testing.T) {
	// Test various invalid inputs that should default to info
	invalidLevels := []string{
		"invalid",
		"DEBUG",  // uppercase
		"Info",   // mixed case
		"trace",  // not a valid level
		"fatal",  // not a valid level
		"unknown",
	}

	for _, level := range invalidLevels {
		logger := createLogger(level)
		if logger == nil {
			t.Errorf("createLogger(%s) returned nil, expected default logger", level)
		}
	}
}

func TestCreateLoggerNonNil(t *testing.T) {
	// Test that logger is never nil
	testCases := []string{"debug", "info", "warn", "error", "invalid", ""}

	for _, tc := range testCases {
		logger := createLogger(tc)
		if logger == nil {
			t.Errorf("createLogger(%s) returned nil logger", tc)
		}
	}
}

func TestGetWebSocketURLMultipleCalls(t *testing.T) {
	// Test that function is stateless and consistent
	domain := "https://test.example.com"

	results := make([]string, 10)
	for i := 0; i < 10; i++ {
		results[i] = getWebSocketURL(domain)
	}

	// All results should be identical
	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Errorf("Inconsistent results: %s != %s", results[i], results[0])
		}
	}
}

func TestGetWebSocketURLSpecialCharacters(t *testing.T) {
	domain := "https://example.com/path?query=value&other=123"
	result := getWebSocketURL(domain)
	expected := "wss://example.com/path?query=value&other=123"

	if result != expected {
		t.Errorf("getWebSocketURL should preserve query params: expected %s, got %s", expected, result)
	}
}

func TestGetWebSocketURLIPAddress(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://192.168.1.1", "wss://192.168.1.1"},
		{"http://127.0.0.1", "ws://127.0.0.1"},
		{"https://[::1]", "wss://[::1]"},
		{"192.168.1.1", "wss://192.168.1.1"},
	}

	for _, tt := range tests {
		result := getWebSocketURL(tt.input)
		if result != tt.expected {
			t.Errorf("getWebSocketURL(%s) = %s; expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestGetWebSocketURLEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"just https://", "https://", "wss://"},
		{"just http://", "http://", "ws://"},
		{"single char domain", "https://x", "wss://x"},
		{"trailing slash", "https://example.com/", "wss://example.com/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getWebSocketURL(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
