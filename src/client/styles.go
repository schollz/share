package client

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Color palette
	primaryColor   = lipgloss.Color("205") // Pink
	successColor   = lipgloss.Color("42")  // Green
	infoColor      = lipgloss.Color("117") // Light blue
	warningColor   = lipgloss.Color("214") // Orange
	errorColor     = lipgloss.Color("196") // Red
	subtleColor    = lipgloss.Color("241") // Gray

	// Styles
	TitleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginBottom(1)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	InfoStyle = lipgloss.NewStyle().
			Foreground(infoColor)

	WarningStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	SubtleStyle = lipgloss.NewStyle().
			Foreground(subtleColor)

	CodeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(0, 1)

	URLStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")).
			Underline(true)
)

// PrintTitle prints a styled title
func PrintTitle(text string) {
	fmt.Println(TitleStyle.Render(text))
}

// PrintSuccess prints a styled success message
func PrintSuccess(text string) {
	fmt.Println(SuccessStyle.Render("✓ " + text))
}

// PrintInfo prints a styled info message
func PrintInfo(text string) {
	fmt.Println(InfoStyle.Render(text))
}

// PrintWarning prints a styled warning message
func PrintWarning(text string) {
	fmt.Println(WarningStyle.Render("⚠ " + text))
}

// PrintError prints a styled error message
func PrintError(text string) {
	fmt.Println(ErrorStyle.Render("✗ " + text))
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
