package client

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProgressModel represents the Bubble Tea model for the progress bar
type ProgressModel struct {
	progress     progress.Model
	totalBytes   int64
	currentBytes int64
	description  string
	startTime    time.Time
	quit         bool
}

// tickMsg is sent periodically to update the progress display
type tickMsg time.Time

// incrementMsg is sent to increment the progress
type incrementMsg int

// NewProgressModel creates a new progress bar model
func NewProgressModel(totalBytes int64, description string) *ProgressModel {
	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	return &ProgressModel{
		progress:     prog,
		totalBytes:   totalBytes,
		currentBytes: 0,
		description:  description,
		startTime:    time.Now(),
	}
}

// Init initializes the progress bar
func (m *ProgressModel) Init() tea.Cmd {
	return tickCmd()
}

// Update handles messages
func (m *ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quit = true
			return m, tea.Quit
		}

	case tickMsg:
		if m.quit {
			return m, tea.Quit
		}
		if m.currentBytes >= m.totalBytes {
			return m, nil
		}
		return m, tickCmd()

	case incrementMsg:
		m.currentBytes += int64(msg)
		if m.currentBytes >= m.totalBytes {
			m.currentBytes = m.totalBytes
			return m, tea.Quit
		}
		return m, nil

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd
	}

	return m, nil
}

// View renders the progress bar
func (m *ProgressModel) View() string {
	if m.quit {
		return ""
	}

	// Calculate percentage
	percent := float64(m.currentBytes) / float64(m.totalBytes)
	if percent > 1.0 {
		percent = 1.0
	}

	// Calculate speed and ETA
	elapsed := time.Since(m.startTime)
	var speedStr, etaStr string
	if elapsed > 0 && m.currentBytes > 0 {
		bytesPerSec := float64(m.currentBytes) / elapsed.Seconds()
		speedStr = fmt.Sprintf("%s/s", formatBytesHuman(int64(bytesPerSec)))

		if bytesPerSec > 0 {
			remaining := float64(m.totalBytes-m.currentBytes) / bytesPerSec
			if remaining > 0 {
				etaStr = fmt.Sprintf(" • ETA: %s", formatDuration(time.Duration(remaining)*time.Second))
			}
		}
	}

	// Styles
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	statsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	// Build the view
	var b strings.Builder

	// Description
	b.WriteString(descStyle.Render(m.description))
	b.WriteString("\n")

	// Progress bar
	b.WriteString(m.progress.ViewAs(percent))
	b.WriteString("\n")

	// Stats line: bytes transferred, percentage, speed, ETA
	stats := fmt.Sprintf("%s / %s • %.1f%%",
		formatBytesHuman(m.currentBytes),
		formatBytesHuman(m.totalBytes),
		percent*100,
	)
	if speedStr != "" {
		stats += fmt.Sprintf(" • %s", speedStr)
	}
	if etaStr != "" {
		stats += etaStr
	}

	b.WriteString(statsStyle.Render(stats))

	return b.String()
}

// tickCmd returns a command that sends a tick message every 100ms
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// formatBytesHuman formats bytes into human-readable string
func formatBytesHuman(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatDuration formats duration into human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

// ProgressWriter wraps the progress bar for file transfer
type ProgressWriter struct {
	program *tea.Program
	model   *ProgressModel
}

// NewProgressWriter creates a new progress writer
func NewProgressWriter(totalBytes int64, description string) *ProgressWriter {
	model := NewProgressModel(totalBytes, description)
	program := tea.NewProgram(model, tea.WithOutput(os.Stderr))

	// Start the program in a goroutine
	go program.Run()

	return &ProgressWriter{
		program: program,
		model:   model,
	}
}

// Add increments the progress bar by n bytes
func (pw *ProgressWriter) Add(n int) {
	pw.program.Send(incrementMsg(n))
}

// Finish completes the progress bar
func (pw *ProgressWriter) Finish() {
	pw.model.currentBytes = pw.model.totalBytes
	pw.program.Send(incrementMsg(0))
	time.Sleep(100 * time.Millisecond) // Give time for final render
	pw.program.Quit()
	pw.program.Wait()
	fmt.Fprintln(os.Stderr) // Add newline after progress bar
}
