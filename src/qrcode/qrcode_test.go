package qrcode

import (
	"bytes"
	"strings"
	"testing"

	"rsc.io/qr"
)

func TestPrintHalfBlockMatchesQRCodePixels(t *testing.T) {
	t.Helper()

	text := "https://example.com"
	leftPadding := 3

	var buf bytes.Buffer
	if err := PrintHalfBlock(&buf, text, leftPadding); err != nil {
		t.Fatalf("PrintHalfBlock returned error: %v", err)
	}

	rawOutput := buf.String()
	if !strings.HasSuffix(rawOutput, "\n") {
		t.Fatalf("PrintHalfBlock output should end with a newline, got %q", rawOutput)
	}

	output := strings.TrimSuffix(rawOutput, "\n")
	lines := strings.Split(output, "\n")

	code, err := qr.Encode(text, qr.L)
	if err != nil {
		t.Fatalf("failed to encode reference QR code: %v", err)
	}

	expectedLines := (code.Size + 1) / 2
	if len(lines) != expectedLines {
		t.Fatalf("expected %d lines, got %d", expectedLines, len(lines))
	}

	padding := strings.Repeat(" ", leftPadding)

	for row, line := range lines {
		if !strings.HasPrefix(line, padding) {
			t.Fatalf("line %d is missing left padding %q: %q", row, padding, line)
		}

		contentRunes := []rune(line[leftPadding:])
		if len(contentRunes) != code.Size {
			t.Fatalf("line %d expected %d runes, got %d", row, code.Size, len(contentRunes))
		}

		y := row * 2
		for x, r := range contentRunes {
			top := code.Black(x, y)
			bottom := false
			if y+1 < code.Size {
				bottom = code.Black(x, y+1)
			}

			var expected rune
			switch {
			case top && bottom:
				expected = '█'
			case top && !bottom:
				expected = '▀'
			case !top && bottom:
				expected = '▄'
			default:
				expected = ' '
			}

			if r != expected {
				t.Fatalf("line %d, column %d: expected %q, got %q", row, x, expected, r)
			}
		}
	}
}
