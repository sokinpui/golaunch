package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sokinpui/golaunch/internal/launchd"
)

func RenderServiceInfo(svc launchd.ServiceInfo) string {
	nameBadge := lipgloss.NewStyle().
		Bold(true).
		Foreground(AccentColor).
		Render(svc.Name)

	statusBadge := RenderStatusBadge(svc.Status)

	var pidInfo string
	if svc.PID > 0 {
		pidInfo = lipgloss.NewStyle().Foreground(DimText).Render(fmt.Sprintf("(PID: %d)", svc.PID))
	} else if svc.ExitCode != 0 {
		pidInfo = lipgloss.NewStyle().Foreground(RedColor).Render(fmt.Sprintf("(Exit code: %d)", svc.ExitCode))
	}

	header := fmt.Sprintf("%s  %s", nameBadge, statusBadge)
	if pidInfo != "" {
		header = fmt.Sprintf("%s %s", header, pidInfo)
	}

	metaKey := lipgloss.NewStyle().Foreground(DimText).Width(13)
	metaVal := lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0"))

	var metaLines []string
	if svc.WorkingDir != "" {
		metaLines = append(metaLines, fmt.Sprintf("%s %s", metaKey.Render("Working Dir:"), metaVal.Render(svc.WorkingDir)))
	}
	if len(svc.Ports) > 0 {
		var portParts []string
		for _, port := range svc.Ports {
			portParts = append(portParts, strconv.Itoa(port))
		}
		label := "Port:"
		if len(portParts) > 1 {
			label = "Ports:"
		}
		metaLines = append(metaLines, fmt.Sprintf("%s %s", metaKey.Render(label), metaVal.Render(strings.Join(portParts, ", "))))
	}
	if svc.PlistPath != "" {
		metaLines = append(metaLines, fmt.Sprintf("%s %s", metaKey.Render("Plist:"), metaVal.Render(svc.PlistPath)))
	}
	if svc.StdoutPath != "" {
		metaLines = append(metaLines, fmt.Sprintf("%s %s", metaKey.Render("Stdout:"), metaVal.Render(svc.StdoutPath)))
	}
	if svc.StderrPath != "" {
		metaLines = append(metaLines, fmt.Sprintf("%s %s", metaKey.Render("Stderr:"), metaVal.Render(svc.StderrPath)))
	}

	cmdLabel := lipgloss.NewStyle().
		Bold(true).
		Foreground(AccentColor).
		Render("Add Command:")

	cmdContent := svc.Comment
	if cmdContent == "" {
		cmdContent = svc.Command
	}

	cmdBox := highlightCommand(cmdContent)

	var sections []string
	sections = append(sections, header)
	if len(metaLines) > 0 {
		sections = append(sections, strings.Join(metaLines, "\n"))
	}
	sections = append(sections, fmt.Sprintf("%s\n%s", cmdLabel, cmdBox))

	return strings.Join(sections, "\n\n")
}

func highlightCommand(cmd string) string {
	if cmd == "" {
		return ""
	}

	lines := strings.Split(cmd, "\n")
	highlightedLines := make([]string, 0, len(lines))

	flagStyle := lipgloss.NewStyle().Foreground(CyanColor)
	stringStyle := lipgloss.NewStyle().Foreground(YellowColor)
	slashStyle := lipgloss.NewStyle().Foreground(DimText)
	dimEqual := lipgloss.NewStyle().Foreground(DimText).Render("=")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			highlightedLines = append(highlightedLines, line)
			continue
		}

		indentLen := len(line) - len(strings.TrimLeft(line, " "))
		indent := line[:indentLen]

		hasContinuation := strings.HasSuffix(trimmed, " \\")
		if hasContinuation {
			trimmed = strings.TrimSuffix(trimmed, " \\")
		}

		var styledContent string
		if strings.HasPrefix(trimmed, "gol add ") {
			parts := strings.SplitN(trimmed, " ", 3)
			golWord := lipgloss.NewStyle().Bold(true).Foreground(AccentColor).Render("gol")
			addWord := lipgloss.NewStyle().Bold(true).Foreground(GreenColor).Render("add")
			rest := ""
			if len(parts) >= 3 {
				subParts := strings.SplitN(parts[2], " ", 2)
				svcName := lipgloss.NewStyle().Bold(true).Foreground(WhiteColor).Render(subParts[0])
				if len(subParts) > 1 {
					rest = " " + stringStyle.Render(subParts[1])
				}
				rest = svcName + rest
			}
			styledContent = fmt.Sprintf("%s %s %s", golWord, addWord, rest)
		} else if strings.HasPrefix(trimmed, "--cwd ") {
			cwdVal := strings.TrimPrefix(trimmed, "--cwd ")
			styledContent = fmt.Sprintf("%s %s", flagStyle.Render("--cwd"), stringStyle.Render(cwdVal))
		} else if strings.HasPrefix(trimmed, "--env ") {
			envVal := strings.TrimPrefix(trimmed, "--env ")
			cleanEnv := strings.Trim(envVal, `"'`)
			envParts := strings.SplitN(cleanEnv, "=", 2)
			if len(envParts) == 2 {
				k := lipgloss.NewStyle().Foreground(WhiteColor).Render(envParts[0])
				v := stringStyle.Render(envParts[1])
				quote := ""
				if strings.HasPrefix(envVal, `"`) && strings.HasSuffix(envVal, `"`) {
					quote = `"`
				} else if strings.HasPrefix(envVal, `'`) && strings.HasSuffix(envVal, `'`) {
					quote = `'`
				}
				if quote != "" {
					styledContent = fmt.Sprintf("%s %s%s%s%s%s", flagStyle.Render("--env"), quote, k, dimEqual, v, quote)
				} else {
					styledContent = fmt.Sprintf("%s %s%s%s", flagStyle.Render("--env"), k, dimEqual, v)
				}
			} else {
				styledContent = fmt.Sprintf("%s %s", flagStyle.Render("--env"), stringStyle.Render(envVal))
			}
		} else {
			styledContent = stringStyle.Render(trimmed)
		}

		if hasContinuation {
			styledContent = fmt.Sprintf("%s %s", styledContent, slashStyle.Render("\\"))
		}

		highlightedLines = append(highlightedLines, indent+styledContent)
	}

	return strings.Join(highlightedLines, "\n")
}
