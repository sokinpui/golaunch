package manager

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/sokinpui/golaunch/internal/launchd"
)

type Manager struct {
	client *launchd.Client
}

func NewManager() (*Manager, error) {
	client, err := launchd.NewClient()
	if err != nil {
		return nil, err
	}
	return &Manager{client: client}, nil
}

func (m *Manager) Add(name string, cmdArgs []string, cwd string, customEnv map[string]string) error {
	if name == "" {
		return errors.New("service name cannot be empty")
	}
	if strings.ContainsAny(name, "/\\ :*?\"<>|") {
		return errors.New("service name contains invalid characters")
	}
	if len(cmdArgs) == 0 {
		return errors.New("command arguments cannot be empty")
	}

	activePlist, err := launchd.PlistPath(name)
	if err != nil {
		return err
	}
	disabledPlist, err := launchd.DisabledPlistPath(name)
	if err != nil {
		return err
	}

	if fileExists(activePlist) || fileExists(disabledPlist) {
		return fmt.Errorf("service '%s' already exists; remove it first with 'gol rm %s'", name, name)
	}

	if cwd == "" {
		currentDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		cwd = currentDir
	}

	resolvedCwd, err := filepath.Abs(cwd)
	if err != nil {
		return fmt.Errorf("invalid working directory: %w", err)
	}
	if !dirExists(resolvedCwd) {
		return fmt.Errorf("working directory does not exist: %s", resolvedCwd)
	}

	binaryPath := cmdArgs[0]
	if resolvedBin, err := exec.LookPath(binaryPath); err == nil {
		binaryPath = resolvedBin
	}
	programArgs := append([]string{binaryPath}, cmdArgs[1:]...)

	envMap := make(map[string]string)
	inheritedEnvs := []string{"PATH", "HOME", "USER", "SHELL", "SSH_AUTH_SOCK"}
	for _, key := range inheritedEnvs {
		if val := os.Getenv(key); val != "" {
			envMap[key] = val
		}
	}
	for k, v := range customEnv {
		envMap[k] = v
	}

	logDir, err := launchd.ServiceLogDir(name)
	if err != nil {
		return fmt.Errorf("failed to prepare log directory: %w", err)
	}

	stdoutPath := filepath.Join(logDir, "stdout.log")
	stderrPath := filepath.Join(logDir, "stderr.log")

	cfg := launchd.PlistConfig{
		Label:                launchd.FormatLabel(name),
		ProgramArguments:     programArgs,
		WorkingDirectory:     resolvedCwd,
		RunAtLoad:            true,
		KeepAlive:            true,
		StandardOutPath:      stdoutPath,
		StandardErrorPath:    stderrPath,
		EnvironmentVariables: envMap,
	}

	if err := launchd.WritePlistFile(activePlist, cfg); err != nil {
		return fmt.Errorf("failed to write plist file: %w", err)
	}

	if err := m.client.Bootstrap(activePlist); err != nil {
		_ = os.Remove(activePlist)
		return err
	}
	_ = m.client.Kickstart(launchd.FormatLabel(name))

	return nil
}

func (m *Manager) List() ([]launchd.ServiceInfo, error) {
	agentsDir, err := launchd.AgentsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read launch agents directory: %w", err)
	}

	liveJobs, err := m.client.ListJobs()
	if err != nil {
		return nil, err
	}

	var services []launchd.ServiceInfo
	for _, entry := range entries {
		filename := entry.Name()
		if !strings.HasPrefix(filename, launchd.DomainPrefix) {
			continue
		}

		isDisabled := strings.HasSuffix(filename, launchd.DisabledExt)
		cleanName := strings.TrimSuffix(filename, launchd.DisabledExt)
		if !strings.HasSuffix(cleanName, ".plist") {
			continue
		}

		label := strings.TrimSuffix(cleanName, ".plist")
		name := launchd.NameFromLabel(label)
		fullPath := filepath.Join(agentsDir, filename)

		cfg, err := launchd.ReadPlistFile(fullPath)
		if err != nil {
			continue
		}

		jobState, isLoaded := liveJobs[label]
		status := launchd.StatusStopped
		pid := 0
		exitCode := 0

		if isLoaded {
			pid = jobState.PID
			exitCode = jobState.ExitCode
			if pid > 0 {
				status = launchd.StatusRunning
			} else if exitCode != 0 {
				status = launchd.StatusError
			}
		}

		services = append(services, launchd.ServiceInfo{
			Name:         name,
			Label:        label,
			Status:       status,
			PID:          pid,
			ExitCode:     exitCode,
			Command:      strings.Join(cfg.ProgramArguments, " "),
			WorkingDir:   cfg.WorkingDirectory,
			PlistPath:    fullPath,
			IsDisabled:   isDisabled,
			StdoutPath:   cfg.StandardOutPath,
			StderrPath:   cfg.StandardErrorPath,
		})
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	for i := range services {
		services[i].ID = i
	}

	return services, nil
}

func (m *Manager) ResolveTarget(target string) (string, error) {
	if target == "" {
		return "", errors.New("service target cannot be empty")
	}

	activePlist, err := launchd.PlistPath(target)
	if err == nil {
		disabledPlist, _ := launchd.DisabledPlistPath(target)
		if fileExists(activePlist) || fileExists(disabledPlist) {
			return target, nil
		}
	}

	id, err := strconv.Atoi(target)
	if err != nil {
		return "", fmt.Errorf("service '%s' not found", target)
	}

	services, err := m.List()
	if err != nil {
		return "", err
	}

	if id >= 0 && id < len(services) {
		return services[id].Name, nil
	}

	if len(services) == 0 {
		return "", errors.New("no managed services found")
	}

	return "", fmt.Errorf("service ID %d not found (available IDs: 0-%d)", id, len(services)-1)
}

func (m *Manager) Start(target string) error {
	name, err := m.ResolveTarget(target)
	if err != nil {
		return err
	}

	activePlist, err := launchd.PlistPath(name)
	if err != nil {
		return err
	}
	disabledPlist, err := launchd.DisabledPlistPath(name)
	if err != nil {
		return err
	}

	if fileExists(disabledPlist) {
		if err := os.Rename(disabledPlist, activePlist); err != nil {
			return fmt.Errorf("failed to enable plist: %w", err)
		}
	}

	if !fileExists(activePlist) {
		return fmt.Errorf("service '%s' not found", name)
	}

	label := launchd.FormatLabel(name)
	jobs, err := m.client.ListJobs()
	if err == nil {
		if job, loaded := jobs[label]; loaded && job.PID > 0 {
			return fmt.Errorf("service '%s' is already running (PID: %d)", name, job.PID)
		}
	}

	_ = m.client.Bootout(label)
	return m.client.Bootstrap(activePlist)
}

func (m *Manager) Stop(target string) error {
	name, err := m.ResolveTarget(target)
	if err != nil {
		return err
	}

	activePlist, err := launchd.PlistPath(name)
	if err != nil {
		return err
	}
	disabledPlist, err := launchd.DisabledPlistPath(name)
	if err != nil {
		return err
	}

	label := launchd.FormatLabel(name)
	if err := m.client.Bootout(label); err != nil {
		return err
	}

	if fileExists(activePlist) {
		if err := os.Rename(activePlist, disabledPlist); err != nil {
			return fmt.Errorf("failed to mark plist as disabled: %w", err)
		}
	}

	return nil
}

func (m *Manager) Restart(target string) error {
	name, err := m.ResolveTarget(target)
	if err != nil {
		return err
	}

	activePlist, err := launchd.PlistPath(name)
	if err != nil {
		return err
	}
	disabledPlist, err := launchd.DisabledPlistPath(name)
	if err != nil {
		return err
	}

	if fileExists(disabledPlist) {
		return m.Start(name)
	}

	if !fileExists(activePlist) {
		return fmt.Errorf("service '%s' not found", name)
	}

	label := launchd.FormatLabel(name)
	return m.client.Kickstart(label)
}

func (m *Manager) Remove(target string) error {
	name, err := m.ResolveTarget(target)
	if err != nil {
		return err
	}

	activePlist, err := launchd.PlistPath(name)
	if err != nil {
		return err
	}
	disabledPlist, err := launchd.DisabledPlistPath(name)
	if err != nil {
		return err
	}

	if !fileExists(activePlist) && !fileExists(disabledPlist) {
		return fmt.Errorf("service '%s' not found", name)
	}

	label := launchd.FormatLabel(name)
	_ = m.client.Bootout(label)

	_ = os.Remove(activePlist)
	_ = os.Remove(disabledPlist)
	return nil
}

func (m *Manager) Logs(target string, lines int, follow bool) error {
	name, err := m.ResolveTarget(target)
	if err != nil {
		return err
	}

	logDir, err := launchd.ServiceLogDir(name)
	if err != nil {
		return err
	}

	stdoutPath := filepath.Join(logDir, "stdout.log")
	stderrPath := filepath.Join(logDir, "stderr.log")

	if !fileExists(stdoutPath) && !fileExists(stderrPath) {
		return fmt.Errorf("no logs found for service '%s'", name)
	}

	targetLog := stdoutPath
	if !fileExists(targetLog) {
		targetLog = stderrPath
	}

	if !follow {
		return printLastLines(targetLog, lines)
	}

	return followFile(targetLog)
}

func printLastLines(filePath string, lineCount int) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > lineCount {
			lines = lines[1:]
		}
	}

	for _, line := range lines {
		fmt.Println(line)
	}

	return scanner.Err()
}

func followFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return err
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	reader := bufio.NewReader(file)
	for {
		select {
		case <-sigChan:
			return nil
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if errors.Is(err, io.EOF) {
					time.Sleep(250 * time.Millisecond)
					continue
				}
				return err
			}
			fmt.Print(line)
		}
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
