package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/sokinpui/golaunch/internal/launchd"
)

func RenderServicesTable(services []launchd.ServiceInfo) string {
	if len(services) == 0 {
		return lipgloss.NewStyle().
			Foreground(DimText).
			Italic(true).
			Render("No managed processes found. Register one using 'gol add <name> <cmd...>'")
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(AccentColor).
		Padding(0, 1)

	cellStyle := lipgloss.NewStyle().
		Padding(0, 1)

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(BorderColor)).
		Headers("ID", "NAME", "STATUS", "PID", "PORT", "EXIT", "COMMAND", "WORKING DIR")

	runningCount := 0
	stoppedCount := 0

	for _, svc := range services {
		pidStr := "-"
		if svc.PID > 0 {
			pidStr = strconv.Itoa(svc.PID)
			runningCount++
		} else {
			stoppedCount++
		}

		portStr := "-"
		if len(svc.Ports) > 0 {
			var portParts []string
			for _, port := range svc.Ports {
				portParts = append(portParts, strconv.Itoa(port))
			}
			portStr = strings.Join(portParts, ", ")
		}

		exitStr := "-"
		if svc.Status != launchd.StatusRunning {
			exitStr = strconv.Itoa(svc.ExitCode)
		}

		cmdDisplay := svc.Command
		if len(cmdDisplay) > 40 {
			cmdDisplay = cmdDisplay[:37] + "..."
		}

		cwdDisplay := svc.WorkingDir
		if len(cwdDisplay) > 35 {
			cwdDisplay = "..." + cwdDisplay[len(cwdDisplay)-32:]
		}

		t.Row(
			strconv.Itoa(svc.ID),
			svc.Name,
			RenderStatusBadge(svc.Status),
			pidStr,
			portStr,
			exitStr,
			cmdDisplay,
			cwdDisplay,
		)
	}

	t.StyleFunc(func(row, col int) lipgloss.Style {
		if row < 0 {
			return headerStyle
		}
		return cellStyle
	})

	summary := lipgloss.NewStyle().
		Foreground(DimText).
		Render(fmt.Sprintf("\n%d services (%d running, %d stopped)", len(services), runningCount, stoppedCount))

	return t.Render() + summary
}
