package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/sokinpui/golaunch/internal/launchd"
)

var (
	AccentColor   = lipgloss.Color("#7D56F4")
	GreenColor    = lipgloss.Color("#04B575")
	GrayColor     = lipgloss.Color("#6E6E6E")
	RedColor      = lipgloss.Color("#EF4444")
	BorderColor   = lipgloss.Color("#383838")
	DimText       = lipgloss.Color("#8A8A8A")

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(AccentColor)

	SuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(GreenColor)

	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(RedColor)

	StatusRunningStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(GreenColor).
				Padding(0, 1)

	StatusStoppedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(GrayColor).
				Padding(0, 1)

	StatusErrorStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(RedColor).
				Padding(0, 1)
)

func RenderStatusBadge(status launchd.ProcessStatus) string {
	switch status {
	case launchd.StatusRunning:
		return StatusRunningStyle.Render("RUNNING")
	case launchd.StatusStopped:
		return StatusStoppedStyle.Render("STOPPED")
	case launchd.StatusError:
		return StatusErrorStyle.Render("ERROR")
	default:
		return StatusStoppedStyle.Render(string(status))
	}
}

func PrintSuccess(msg string) {
	fmt.Printf("%s %s\n", SuccessStyle.Render("✓"), msg)
}

func PrintError(err error) {
	fmt.Printf("%s %s\n", ErrorStyle.Render("✗"), err.Error())
}

func PrintInfo(msg string) {
	fmt.Printf("%s %s\n", TitleStyle.Render("•"), msg)
}
