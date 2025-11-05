package client

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Styles with no custom colors - use terminal defaults
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			MarginBottom(1)

	SuccessStyle = lipgloss.NewStyle().
			Bold(true)

	InfoStyle = lipgloss.NewStyle()

	WarningStyle = lipgloss.NewStyle().
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Bold(true)

	SubtleStyle = lipgloss.NewStyle()

	CodeStyle = lipgloss.NewStyle().
			Padding(0, 1)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)

	URLStyle = lipgloss.NewStyle().
			Underline(true)
)

// PrintTitle prints a styled title
func PrintTitle(text string) {
	fmt.Println(TitleStyle.Render(text))
}

// PrintSuccess prints a styled success message
func PrintSuccess(text string) {
	fmt.Println(SuccessStyle.Render(text))
}

// PrintInfo prints a styled info message
func PrintInfo(text string) {
	fmt.Println(InfoStyle.Render(text))
}

// PrintWarning prints a styled warning message
func PrintWarning(text string) {
	fmt.Println(WarningStyle.Render(text))
}

// PrintError prints a styled error message
func PrintError(text string) {
	fmt.Println(ErrorStyle.Render(text))
}

// PrintSubtle prints a styled subtle message
func PrintSubtle(text string) {
	fmt.Println(SubtleStyle.Render(text))
}

// PrintCode prints a styled code snippet
func PrintCode(text string) {
	fmt.Println(CodeStyle.Render(text))
}

// PrintBox prints text in a styled box
func PrintBox(text string) {
	fmt.Println(BoxStyle.Render(text))
}
