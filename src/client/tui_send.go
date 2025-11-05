package client

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// tickMsg is sent periodically to update the display
type tickMsg time.Time

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

type SendTUIModel struct {
	fileName        string
	fileSize        int64
	roomID          string
	receiveCmd      string
	webURL          string
	qrCode          string
	status          string
	connected       bool
	transferring    bool
	currentBytes    int64
	startTime       time.Time
	progress        progress.Model
	quitting        bool
	width           int
	height          int
	localRelay      bool
	senderName      string
	receiverName    string
	completed       bool
	err             error
}

type sendStatusMsg struct {
	status       string
	connected    bool
	localRelay   bool
	receiverName string
}

type sendProgressMsg int64
type sendCompleteMsg struct{}
type sendErrorMsg error

func NewSendTUIModel(fileName string, fileSize int64, roomID string) *SendTUIModel {
	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(50),
		progress.WithoutPercentage(),
	)

	return &SendTUIModel{
		fileName:     fileName,
		fileSize:     fileSize,
		roomID:       roomID,
		status:       "Waiting for receiver...",
		connected:    false,
		transferring: false,
		currentBytes: 0,
		startTime:    time.Now(),
		progress:     prog,
		width:        80,
		height:       24,
	}
}

func (m *SendTUIModel) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		tickCmd(),
	)
}

func (m *SendTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case sendStatusMsg:
		m.status = msg.status
		m.connected = msg.connected
		m.localRelay = msg.localRelay
		m.receiverName = msg.receiverName
		if msg.connected {
			m.transferring = true
			m.startTime = time.Now()
		}

	case sendProgressMsg:
		m.currentBytes = int64(msg)

	case sendCompleteMsg:
		m.completed = true
		m.currentBytes = m.fileSize
		return m, tea.Quit

	case sendErrorMsg:
		m.err = msg
		return m, tea.Quit

	case tickMsg:
		if m.quitting || m.completed {
			return m, nil
		}
		return m, tickCmd()

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd
	}

	return m, nil
}

func (m *SendTUIModel) View() string {
	if m.quitting && !m.completed {
		return ""
	}

	// Calculate dimensions
	qrWidth := 35
	contentWidth := m.width - qrWidth - 6

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Width(contentWidth)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Bold(true)

	codeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("99")).
		Background(lipgloss.Color("236")).
		Padding(0, 1)

	urlStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("99")).
		Underline(true)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Width(m.width - 4)

	// Build content
	var content strings.Builder

	// Title
	content.WriteString(titleStyle.Render(fmt.Sprintf("SENDING: %s (%s)", m.fileName, formatBytesHuman(m.fileSize))))
	content.WriteString("\n\n")

	// Status
	statusLabel := labelStyle.Render("Status: ")
	statusValue := m.status
	if m.localRelay {
		statusValue += " (direct connection)"
	}
	content.WriteString(statusLabel + valueStyle.Render(statusValue))
	content.WriteString("\n\n")

	// Room info
	if m.roomID != "" {
		content.WriteString(labelStyle.Render("Room: ") + valueStyle.Render(m.roomID))
		content.WriteString("\n\n")
	}

	// Receiver name
	if m.receiverName != "" {
		content.WriteString(labelStyle.Render("Receiver: ") + valueStyle.Render(m.receiverName))
		content.WriteString("\n\n")
	}

	// Receive instructions
	if m.receiveCmd != "" {
		content.WriteString(labelStyle.Render("Receive via CLI:"))
		content.WriteString("\n")
		content.WriteString(codeStyle.Render(m.receiveCmd))
		content.WriteString("\n\n")
	}

	// Web URL
	if m.webURL != "" {
		content.WriteString(labelStyle.Render("Or visit:"))
		content.WriteString("\n")
		content.WriteString(urlStyle.Render(m.webURL))
		content.WriteString("\n\n")
	}

	// Progress section
	if m.transferring {
		content.WriteString(labelStyle.Render("Progress:"))
		content.WriteString("\n")

		percent := float64(m.currentBytes) / float64(m.fileSize)
		if percent > 1.0 {
			percent = 1.0
		}

		content.WriteString(m.progress.ViewAs(percent))
		content.WriteString("\n")

		// Stats
		elapsed := time.Since(m.startTime)
		var speedStr, etaStr string
		if elapsed > 0 && m.currentBytes > 0 {
			bytesPerSec := float64(m.currentBytes) / elapsed.Seconds()
			speedStr = fmt.Sprintf("%s/s", formatBytesHuman(int64(bytesPerSec)))

			if bytesPerSec > 0 {
				remaining := float64(m.fileSize-m.currentBytes) / bytesPerSec
				if remaining > 0 {
					etaStr = formatDuration(time.Duration(remaining) * time.Second)
				}
			}
		}

		stats := fmt.Sprintf("%s / %s", formatBytesHuman(m.currentBytes), formatBytesHuman(m.fileSize))
		if speedStr != "" {
			stats += fmt.Sprintf(" • %s", speedStr)
		}
		if etaStr != "" {
			stats += fmt.Sprintf(" • ETA: %s", etaStr)
		}
		stats += fmt.Sprintf(" • %.1f%%", percent*100)

		content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(stats))
		content.WriteString("\n")
	}

	// Create left and right panels
	leftPanel := content.String()

	// QR code panel (right side)
	var qrPanel string
	if m.qrCode != "" {
		qrLines := strings.Split(m.qrCode, "\n")
		qrPanel = strings.Join(qrLines, "\n")
	} else {
		qrPanel = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render("QR code will appear\nwhen ready...")
	}

	// Combine panels side by side
	combined := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftPanel,
		strings.Repeat(" ", 3),
		qrPanel,
	)

	// Wrap in box
	result := boxStyle.Render(combined)

	// Add help text at bottom
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)
	help := helpStyle.Render("\nPress q or Ctrl+C to quit")

	return result + help
}

func (m *SendTUIModel) SetConnectionInfo(receiveCmd, webURL, qrCode string) {
	m.receiveCmd = receiveCmd
	m.webURL = webURL
	m.qrCode = qrCode
}

func (m *SendTUIModel) UpdateStatus(status string, connected bool, localRelay bool, receiverName string) {
	m.status = status
	m.connected = connected
	m.localRelay = localRelay
	m.receiverName = receiverName
	if connected {
		m.transferring = true
		m.startTime = time.Now()
	}
}

func (m *SendTUIModel) UpdateProgress(bytes int64) {
	m.currentBytes = bytes
}

func (m *SendTUIModel) Complete() {
	m.completed = true
	m.currentBytes = m.fileSize
}

func (m *SendTUIModel) Error(err error) {
	m.err = err
}
