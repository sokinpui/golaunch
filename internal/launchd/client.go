package launchd

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Client struct {
	domain string
}

func NewClient() (*Client, error) {
	domain, err := resolveDomain()
	if err != nil {
		return nil, err
	}
	return &Client{domain: domain}, nil
}

func resolveDomain() (string, error) {
	uid := os.Getuid()
	guiDomain := fmt.Sprintf("gui/%d", uid)
	cmd := exec.Command("launchctl", "print", guiDomain)
	if err := cmd.Run(); err == nil {
		return guiDomain, nil
	}

	userDomain := fmt.Sprintf("user/%d", uid)
	cmd = exec.Command("launchctl", "print", userDomain)
	if err := cmd.Run(); err == nil {
		return userDomain, nil
	}

	return "", fmt.Errorf("unable to resolve launchd domain for UID %d", uid)
}

func (c *Client) Domain() string {
	return c.domain
}

func (c *Client) Bootstrap(plistPath string) error {
	cmd := exec.Command("launchctl", "bootstrap", c.domain, plistPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bootstrap failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

func (c *Client) Bootout(label string) error {
	target := fmt.Sprintf("%s/%s", c.domain, label)
	cmd := exec.Command("launchctl", "bootout", target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(output)
		if strings.Contains(outStr, "No such process") || strings.Contains(outStr, "Could not find") {
			return nil
		}
		return fmt.Errorf("bootout failed: %s: %w", strings.TrimSpace(outStr), err)
	}
	return nil
}

func (c *Client) Kickstart(label string) error {
	target := fmt.Sprintf("%s/%s", c.domain, label)
	cmd := exec.Command("launchctl", "kickstart", "-k", target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kickstart failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

func (c *Client) ListJobs() (map[string]JobState, error) {
	cmd := exec.Command("launchctl", "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run launchctl list: %w", err)
	}

	jobs := make(map[string]JobState)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "PID\t") || strings.HasPrefix(line, "PID ") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		label := fields[2]
		if !strings.HasPrefix(label, DomainPrefix) {
			continue
		}

		pid := 0
		if fields[0] != "-" {
			parsedPID, err := strconv.Atoi(fields[0])
			if err == nil {
				pid = parsedPID
			}
		}

		exitCode, _ := strconv.Atoi(fields[1])
		jobs[label] = JobState{
			PID:      pid,
			ExitCode: exitCode,
			Loaded:   true,
		}
	}

	return jobs, scanner.Err()
}
