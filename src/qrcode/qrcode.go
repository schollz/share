package qrcode

import (
	"io"
	"strings"

	"rsc.io/qr"
)

// PrintHalfBlock prints a QR code using half-block characters for compact display
// leftPadding specifies the number of spaces to add before each line
func PrintHalfBlock(w io.Writer, text string, leftPadding int) error {
	// Generate QR code with low error correction for smaller size
	code, err := qr.Encode(text, qr.L)
	if err != nil {
		return err
	}

	size := code.Size
	padding := strings.Repeat(" ", leftPadding)

	// Print QR code using half-blocks (each char represents 2 vertical pixels)
	for y := 0; y < size; y += 2 {
		// Write left padding
		w.Write([]byte(padding))

		for x := 0; x < size; x++ {
			topBlack := code.Black(x, y)
			bottomBlack := false
			if y+1 < size {
				bottomBlack = code.Black(x, y+1)
			}

			// Choose character based on which pixels are black
			switch {
			case topBlack && bottomBlack:
				w.Write([]byte("█")) // Both black
			case topBlack && !bottomBlack:
				w.Write([]byte("▀")) // Top black
			case !topBlack && bottomBlack:
				w.Write([]byte("▄")) // Bottom black
			default:
				w.Write([]byte(" ")) // Both white
			}
		}
		w.Write([]byte("\n"))
	}

	return nil
}
