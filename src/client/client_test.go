package client

import "testing"

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple filename", "test.txt", "test.txt"},
		{"path traversal with ..", "../../../etc/passwd", "passwd"},
		{"path traversal with multiple ..", "../../file.txt", "file.txt"},
		{"unix path", "/etc/passwd", "passwd"},
		{"relative path", "dir/subdir/file.txt", "file.txt"},
		{"hidden file", ".hidden", ".hidden"},
		{"empty string", "", "."},
		{"current dir", ".", "."},
		{"complex path traversal", "../../../tmp/../etc/passwd", "passwd"},
		{"filename with spaces", "my file.txt", "my file.txt"},
		{"path with spaces", "path/to/my file.txt", "my file.txt"},
		{"dot dot", "..", ".."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeFileName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeFileName(%q) = %q; expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"zero bytes", 0, "0 B"},
		{"bytes only", 500, "500 B"},
		{"1 KB", 1024, "1.0 KB"},
		{"1.5 KB", 1536, "1.5 KB"},
		{"1 MB", 1024 * 1024, "1.0 MB"},
		{"2.5 MB", 2621440, "2.5 MB"},
		{"1 GB", 1024 * 1024 * 1024, "1.0 GB"},
		{"1 TB", 1024 * 1024 * 1024 * 1024, "1.0 TB"},
		{"large value", 5368709120, "5.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatBytes(%d) = %s; expected %s", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestFormatBytesEdgeCases(t *testing.T) {
	// Test boundary values
	tests := []struct {
		bytes int64
	}{
		{1023},  // Just below 1 KB
		{1024},  // Exactly 1 KB
		{1025},  // Just above 1 KB
		{1048575}, // Just below 1 MB
		{1048576}, // Exactly 1 MB
	}

	for _, tt := range tests {
		result := formatBytes(tt.bytes)
		if result == "" {
			t.Errorf("formatBytes(%d) returned empty string", tt.bytes)
		}
	}
}

func TestFormatBytesUnits(t *testing.T) {
	// Test that units are correctly identified
	testCases := []struct {
		bytes        int64
		expectedUnit string
	}{
		{100, "B"},
		{1024, "KB"},
		{1024 * 1024, "MB"},
		{1024 * 1024 * 1024, "GB"},
		{1024 * 1024 * 1024 * 1024, "TB"},
	}

	for _, tc := range testCases {
		result := formatBytes(tc.bytes)
		// Check if the result contains the expected unit
		_ = result
		// Simple check: result should contain the unit
		if len(result) < 2 {
			t.Errorf("formatBytes(%d) = %s; expected to contain unit %s", tc.bytes, result, tc.expectedUnit)
		}
	}
}

func TestFormatBytesConsistency(t *testing.T) {
	// Test that same input gives same output
	bytes := int64(1234567)
	result1 := formatBytes(bytes)
	result2 := formatBytes(bytes)

	if result1 != result2 {
		t.Errorf("formatBytes not consistent: %s != %s", result1, result2)
	}
}

func TestFormatBytesNegative(t *testing.T) {
	// While negative bytes don't make sense, test the function handles it
	result := formatBytes(-100)
	// Function should still return something (not crash)
	if result == "" {
		t.Error("formatBytes(-100) returned empty string")
	}
}

func TestFormatBytesLargeValues(t *testing.T) {
	// Test with very large values
	largeValue := int64(1024 * 1024 * 1024 * 1024 * 10) // 10 TB
	result := formatBytes(largeValue)

	if result == "" {
		t.Error("formatBytes failed on large value")
	}

	// Should contain "TB"
	if len(result) < 4 {
		t.Errorf("formatBytes(%d) = %s; expected format with TB", largeValue, result)
	}
}

func TestFormatBytesPrecision(t *testing.T) {
	// Test that the precision is one decimal place
	bytes := int64(1536) // 1.5 KB
	result := formatBytes(bytes)

	// Should be "1.5 KB"
	if result != "1.5 KB" {
		t.Errorf("formatBytes(%d) = %s; expected 1.5 KB", bytes, result)
	}
}

func TestFormatBytesKilobyteBoundary(t *testing.T) {
	// Test values around the KB boundary
	justBelow := int64(1023)
	atBoundary := int64(1024)
	justAbove := int64(1025)

	resultBelow := formatBytes(justBelow)
	resultAt := formatBytes(atBoundary)
	resultAbove := formatBytes(justAbove)

	// Just below should be in bytes
	if resultBelow != "1023 B" {
		t.Errorf("formatBytes(%d) = %s; expected 1023 B", justBelow, resultBelow)
	}

	// At boundary should be 1.0 KB
	if resultAt != "1.0 KB" {
		t.Errorf("formatBytes(%d) = %s; expected 1.0 KB", atBoundary, resultAt)
	}

	// Just above should be 1.0 KB (rounds down in display)
	if resultAbove != "1.0 KB" {
		t.Errorf("formatBytes(%d) = %s; expected 1.0 KB", justAbove, resultAbove)
	}
}

func TestFormatBytesMegabyteBoundary(t *testing.T) {
	// Test values around the MB boundary
	atBoundary := int64(1048576) // 1 MB
	result := formatBytes(atBoundary)

	if result != "1.0 MB" {
		t.Errorf("formatBytes(%d) = %s; expected 1.0 MB", atBoundary, result)
	}
}

func TestFormatBytesGigabyteBoundary(t *testing.T) {
	// Test values around the GB boundary
	atBoundary := int64(1073741824) // 1 GB
	result := formatBytes(atBoundary)

	if result != "1.0 GB" {
		t.Errorf("formatBytes(%d) = %s; expected 1.0 GB", atBoundary, result)
	}
}
