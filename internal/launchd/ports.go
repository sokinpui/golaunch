package launchd

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

func DiscoverListeningPorts(rootPIDs []int) map[int][]int {
	result := make(map[int][]int)
	if len(rootPIDs) == 0 {
		return result
	}

	pidPorts := queryListeningPorts()
	if len(pidPorts) == 0 {
		return result
	}

	childMap := queryProcessChildren()

	for _, rootPID := range rootPIDs {
		if rootPID <= 0 {
			continue
		}

		descendants := collectDescendants(rootPID, childMap)
		portSet := make(map[int]bool)
		for _, pid := range descendants {
			for _, port := range pidPorts[pid] {
				portSet[port] = true
			}
		}

		if len(portSet) == 0 {
			continue
		}

		var ports []int
		for port := range portSet {
			ports = append(ports, port)
		}
		sort.Ints(ports)
		result[rootPID] = ports
	}

	return result
}

func queryListeningPorts() map[int][]int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	lsofBin := resolveExecutable("lsof", "/usr/sbin/lsof")
	cmd := exec.CommandContext(ctx, lsofBin, "-nP", "-iTCP", "-sTCP:LISTEN", "-F", "pn")
	output, err := cmd.Output()
	if err != nil && len(output) == 0 {
		return nil
	}

	pidPorts := make(map[int][]int)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	currPID := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "p") {
			pid, err := strconv.Atoi(line[1:])
			if err != nil {
				currPID = 0
				continue
			}
			currPID = pid
			continue
		}

		if !strings.HasPrefix(line, "n") || currPID <= 0 {
			continue
		}

		port, ok := parsePort(line[1:])
		if !ok {
			continue
		}

		pidPorts[currPID] = append(pidPorts[currPID], port)
	}

	return pidPorts
}

func parsePort(addr string) (int, bool) {
	if strings.Contains(addr, "->") {
		return 0, false
	}
	if idx := strings.Index(addr, " "); idx != -1 {
		addr = addr[:idx]
	}

	colonIdx := strings.LastIndex(addr, ":")
	if colonIdx == -1 {
		return 0, false
	}

	portStr := addr[colonIdx+1:]
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return 0, false
	}

	return port, true
}

func queryProcessChildren() map[int][]int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	psBin := resolveExecutable("ps", "/bin/ps")
	cmd := exec.CommandContext(ctx, psBin, "-eo", "pid,ppid")
	output, err := cmd.Output()
	if err != nil {
		return make(map[int][]int)
	}

	childMap := make(map[int][]int)
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}

		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			continue
		}

		childMap[ppid] = append(childMap[ppid], pid)
	}

	return childMap
}

func collectDescendants(rootPID int, childMap map[int][]int) []int {
	descendants := []int{rootPID}
	visited := map[int]bool{rootPID: true}
	queue := []int{rootPID}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, child := range childMap[curr] {
			if visited[child] {
				continue
			}
			visited[child] = true
			descendants = append(descendants, child)
			queue = append(queue, child)
		}
	}

	return descendants
}

func resolveExecutable(name, fallback string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	if info, err := os.Stat(fallback); err == nil && !info.IsDir() {
		return fallback
	}
	return name
}
