package launchd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DomainPrefix = "dev.golaunch."
	DisabledExt  = ".disabled"
)

func UserHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if home == "" {
		return "", errors.New("empty user home directory")
	}
	return home, nil
}

func AgentsDir() (string, error) {
	home, err := UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func ServiceLogDir(name string) (string, error) {
	home, err := UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Library", "Logs", "golaunch", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func FormatLabel(name string) string {
	return DomainPrefix + name
}

func NameFromLabel(label string) string {
	return strings.TrimPrefix(label, DomainPrefix)
}

func PlistPath(name string) (string, error) {
	agentsDir, err := AgentsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(agentsDir, fmt.Sprintf("%s%s.plist", DomainPrefix, name)), nil
}

func DisabledPlistPath(name string) (string, error) {
	path, err := PlistPath(name)
	if err != nil {
		return "", err
	}
	return path + DisabledExt, nil
}
