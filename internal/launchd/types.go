package launchd

type ProcessStatus string

const (
	StatusRunning ProcessStatus = "RUNNING"
	StatusStopped ProcessStatus = "STOPPED"
	StatusError   ProcessStatus = "ERROR"
)

type PlistConfig struct {
	Label                string            `plist:"Label"`
	ProgramArguments     []string          `plist:"ProgramArguments"`
	WorkingDirectory     string            `plist:"WorkingDirectory,omitempty"`
	RunAtLoad            bool              `plist:"RunAtLoad"`
	KeepAlive            bool              `plist:"KeepAlive"`
	StandardOutPath      string            `plist:"StandardOutPath,omitempty"`
	StandardErrorPath    string            `plist:"StandardErrorPath,omitempty"`
	EnvironmentVariables map[string]string `plist:"EnvironmentVariables,omitempty"`
}

type JobState struct {
	PID      int
	ExitCode int
	Loaded   bool
}

type ServiceInfo struct {
	Name         string
	Label        string
	Status       ProcessStatus
	PID          int
	ExitCode     int
	Command      string
	WorkingDir   string
	PlistPath    string
	IsDisabled   bool
	StdoutPath   string
	StderrPath   string
}
