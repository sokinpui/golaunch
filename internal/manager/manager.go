package manager

import (
	"bufio"
	"context"
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
	"github.com/sokinpui/golaunch/internal/ui"
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
	if pathEnv := os.Getenv("PATH"); pathEnv != "" {
		envMap["PATH"] = pathEnv
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

type LogOptions struct {
	Lines   int
	Follow  bool
	ShowOut bool
	ShowErr bool
}

type streamMsg struct {
	stream string
	line   string
}

func (m *Manager) Logs(target string, opts LogOptions) error {
	name, err := m.ResolveTarget(target)
	if err != nil {
		return err
	}

	stdoutPath, stderrPath, err := m.resolveLogPaths(name)
	if err != nil {
		return err
	}

	hasStdout := fileExists(stdoutPath)
	hasStderr := fileExists(stderrPath)
	if !hasStdout && !hasStderr {
		return fmt.Errorf("no logs found for service '%s'", name)
	}

	includeOut := opts.ShowOut || (!opts.ShowOut && !opts.ShowErr)
	includeErr := opts.ShowErr || (!opts.ShowOut && !opts.ShowErr)

	targetOut := ""
	if includeOut && hasStdout {
		targetOut = stdoutPath
	}
	targetErr := ""
	if includeErr && hasStderr {
		targetErr = stderrPath
	}

	if targetOut == "" && targetErr == "" {
		if opts.ShowErr {
			return fmt.Errorf("no stderr log found for service '%s'", name)
		}
		if opts.ShowOut {
			return fmt.Errorf("no stdout log found for service '%s'", name)
		}
		return fmt.Errorf("no logs found for service '%s'", name)
	}

	var outLines []string
	var errLines []string
	if targetOut != "" && opts.Lines > 0 {
		outLines, _ = readLastLines(targetOut, opts.Lines)
	}
	if targetErr != "" && opts.Lines > 0 {
		errLines, _ = readLastLines(targetErr, opts.Lines)
	}

	if !opts.Follow {
		if len(outLines) == 0 && len(errLines) == 0 {
			fmt.Println("(log files exist but are currently empty)")
			return nil
		}
		if len(outLines) > 0 && len(errLines) == 0 {
			for _, line := range outLines {
				fmt.Println(line)
			}
			return nil
		}
		if len(outLines) == 0 && len(errLines) > 0 {
			for _, line := range errLines {
				fmt.Println(line)
			}
			return nil
		}
		fmt.Println(ui.RenderLogHeader("STDOUT"))
		for _, line := range outLines {
			fmt.Println(line)
		}
		fmt.Println()
		fmt.Println(ui.RenderLogHeader("STDERR"))
		for _, line := range errLines {
			fmt.Println(line)
		}
		return nil
	}

	both := targetOut != "" && targetErr != ""
	if both {
		for _, line := range outLines {
			fmt.Printf("%s %s\n", ui.RenderStreamBadge("STDOUT"), line)
		}
		for _, line := range errLines {
			fmt.Printf("%s %s\n", ui.RenderStreamBadge("STDERR"), line)
		}
	} else {
		for _, line := range outLines {
			fmt.Println(line)
		}
		for _, line := range errLines {
			fmt.Println(line)
		}
	}

	return followStreams(targetOut, targetErr)
}

func (m *Manager) resolveLogPaths(name string) (string, string, error) {
	logDir, err := launchd.ServiceLogDir(name)
	if err != nil {
		return "", "", err
	}
	stdoutPath := filepath.Join(logDir, "stdout.log")
	stderrPath := filepath.Join(logDir, "stderr.log")

	activePlist, _ := launchd.PlistPath(name)
	disabledPlist, _ := launchd.DisabledPlistPath(name)
	plistPath := activePlist
	if !fileExists(plistPath) && fileExists(disabledPlist) {
		plistPath = disabledPlist
	}

	if fileExists(plistPath) {
		if cfg, err := launchd.ReadPlistFile(plistPath); err == nil {
			if cfg.StandardOutPath != "" {
				stdoutPath = cfg.StandardOutPath
			}
			if cfg.StandardErrorPath != "" {
				stderrPath = cfg.StandardErrorPath
			}
		}
	}

	return stdoutPath, stderrPath, nil
}

func readLastLines(filePath string, maxLines int) ([]string, error) {
	if maxLines <= 0 {
		return nil, nil
	}
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() == 0 {
		return nil, nil
	}

	seeked := false
	const maxDirectReadSize = 1024 * 1024
	if stat.Size() > maxDirectReadSize {
		seekBytes := int64(maxLines * 1024)
		if seekBytes < maxDirectReadSize {
			seekBytes = maxDirectReadSize
		}
		if stat.Size() > seekBytes {
			if _, err := file.Seek(-seekBytes, io.SeekEnd); err == nil {
				seeked = true
			}
		}
	}

	var lines []string
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	if seeked {
		scanner.Scan()
	}

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > maxLines {
			lines = lines[1:]
		}
	}
	return lines, scanner.Err()
}

func followStreams(targetOut, targetErr string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ch := make(chan streamMsg, 64)
	both := targetOut != "" && targetErr != ""

	if targetOut != "" {
		go tailFile(ctx, targetOut, "STDOUT", ch)
	}
	if targetErr != "" {
		go tailFile(ctx, targetErr, "STDERR", ch)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-ch:
			if both {
				fmt.Printf("%s %s", ui.RenderStreamBadge(msg.stream), msg.line)
				continue
			}
			fmt.Print(msg.line)
		}
	}
}

func tailFile(ctx context.Context, filePath, stream string, ch chan<- streamMsg) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return
	}

	reader := bufio.NewReader(file)
	var pending strings.Builder
	for {
		select {
		case <-ctx.Done():
			return
		default:
			chunk, err := reader.ReadString('\n')
			if len(chunk) > 0 {
				pending.WriteString(chunk)
				if strings.HasSuffix(chunk, "\n") {
					line := pending.String()
					pending.Reset()
					select {
					case <-ctx.Done():
						return
					case ch <- streamMsg{stream: stream, line: line}:
					}
				}
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					select {
					case <-ctx.Done():
						return
					case <-time.After(200 * time.Millisecond):
					}
					continue
				}
				return
			}
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
