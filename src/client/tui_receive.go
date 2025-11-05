package client

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ReceiveTUIModel struct {
	fileName      string
	fileSize      int64
	roomID        string
	myCode        string
	status        string
	connected     bool
	transferring  bool
	currentBytes  int64
	startTime     time.Time
	progress      progress.Model
	quitting      bool
	width         int
	height        int
	localRelay    bool
	senderName    string
	completed     bool
	savedPath     string
	err           error
	isFolder      bool
	folderName    string
}

type receiveStatusMsg struct {
	status     string
	connected  bool
	localRelay bool
	senderName string
}

type receiveProgressMsg int64

type receiveFileInfoMsg struct {
	fileName   string
	fileSize   int64
	isFolder   bool
	folderName string
}

type receiveCompleteMsg struct {
	savedPath string
}

type receiveErrorMsg error

func NewReceiveTUIModel(roomID string) *ReceiveTUIModel {
	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(60),
		progress.WithoutPercentage(),
	)

	return &ReceiveTUIModel{
		roomID:       roomID,
		status:       "Connecting...",
		connected:    false,
		transferring: false,
		currentBytes: 0,
		startTime:    time.Now(),
		progress:     prog,
		width:        80,
		height:       24,
	}
}

func (m *ReceiveTUIModel) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		tickCmd(),
	)
}

func (m *ReceiveTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case receiveStatusMsg:
		m.status = msg.status
		m.connected = msg.connected
		m.localRelay = msg.localRelay
		m.senderName = msg.senderName

	case receiveFileInfoMsg:
		m.fileName = msg.fileName
		m.fileSize = msg.fileSize
		m.isFolder = msg.isFolder
		m.folderName = msg.folderName
		m.transferring = true
		m.startTime = time.Now()

	case receiveProgressMsg:
		m.currentBytes = int64(msg)

	case receiveCompleteMsg:
		m.completed = true
		m.savedPath = msg.savedPath
		m.currentBytes = m.fileSize
		return m, tea.Quit

	case receiveErrorMsg:
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

func (m *ReceiveTUIModel) View() string {
	if m.quitting && !m.completed {
		return ""
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Width(m.width - 8)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Bold(true)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Width(m.width - 4)

	// Build content
	var content strings.Builder

	// Title
	title := "RECEIVING"
	if m.fileName != "" {
		if m.isFolder && m.folderName != "" {
			title = fmt.Sprintf("RECEIVING FOLDER: %s (%s)", m.folderName, formatBytesHuman(m.fileSize))
		} else {
			title = fmt.Sprintf("RECEIVING: %s (%s)", m.fileName, formatBytesHuman(m.fileSize))
		}
	}
	content.WriteString(titleStyle.Render(title))
	content.WriteString("\n\n")

	// My code
	if m.myCode != "" {
		content.WriteString(labelStyle.Render("Your code: ") + valueStyle.Render(m.myCode))
		content.WriteString("\n\n")
	}

	// Room info
	if m.roomID != "" {
		content.WriteString(labelStyle.Render("Room: ") + valueStyle.Render(m.roomID))
		content.WriteString("\n\n")
	}

	// Sender name
	if m.senderName != "" {
		content.WriteString(labelStyle.Render("Sender: ") + valueStyle.Render(m.senderName))
		content.WriteString("\n\n")
	}

	// Status
	statusLabel := labelStyle.Render("Status: ")
	statusValue := m.status
	if m.localRelay {
		statusValue += " (direct connection)"
	}
	content.WriteString(statusLabel + valueStyle.Render(statusValue))
	content.WriteString("\n\n")

	// Progress section
	if m.transferring && m.fileSize > 0 {
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

	// Wrap in box
	result := boxStyle.Render(content.String())

	// Add help text at bottom
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)
	help := helpStyle.Render("\nPress q or Ctrl+C to quit")

	return result + help
}

func (m *ReceiveTUIModel) SetMyCode(code string) {
	m.myCode = code
}

func (m *ReceiveTUIModel) SetFileInfo(fileName string, fileSize int64, isFolder bool, folderName string) {
	m.fileName = fileName
	m.fileSize = fileSize
	m.isFolder = isFolder
	m.folderName = folderName
	m.transferring = true
	m.startTime = time.Now()
}

func (m *ReceiveTUIModel) UpdateStatus(status string, connected bool, localRelay bool, senderName string) {
	m.status = status
	m.connected = connected
	m.localRelay = localRelay
	m.senderName = senderName
}

func (m *ReceiveTUIModel) UpdateProgress(bytes int64) {
	m.currentBytes = bytes
}

func (m *ReceiveTUIModel) Complete(savedPath string) {
	m.completed = true
	m.savedPath = savedPath
	m.currentBytes = m.fileSize
}

func (m *ReceiveTUIModel) Error(err error) {
	m.err = err
}
