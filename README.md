# golaunchd (`gol`)

`golaunchd` (`gol`) is a lightweight, native macOS process and service manager written in Go. It acts as a clean wrapper around macOS `launchd` and `launchctl`, designed specifically for developers who want to manage local background services, servers, and scripts without dealing with the boilerplate of writing raw plist files or managing system-wide daemons.

## Features

- **Native Integration**: Directly hooks into macOS `launchd` via modern `launchctl` commands (`bootstrap`, `bootout`, `kickstart`).
- **User-Scoped Management**: Manages user/GUI domain services (`gui/<uid>`) without requiring root/sudo privileges.
- **Simple CLI**: Clean and concise commands to add, start, stop, restart, remove, and list services.
- **Interactive Log Streaming**: Real-time log tailing (`-f` / `--follow`) with automatic color-coded separation for `STDOUT` and `STDERR`.
- **Flexible Targeting**: Reference services by name or by their index ID from the service list.
- **Modern TUI Tables**: Gorgeous terminal UI powered by Charm (`lipgloss`) for clear process monitoring.

---

## Installation & Setup

Clone the repository and build or run the binary:

```bash
# Build the binary
go build -o gol ./cmd/gol

# Or use the runner script
./run.sh
```

---

## Usage

`golaunchd` uses the command name `gol`.

### 1. Register and Start a Service (`add`)
Register a new service by providing a name and a command string. You can optionally specify a working directory (`--cwd`) and environment variables (`--env`).

```bash
gol add web-server --cwd ./app "node server.js"
gol add worker --env PORT=8080 --env NODE_ENV=production "python3 worker.py"
```

### 2. List Managed Services (`ls`)
View the status, PID, exit codes, and paths of all managed services:

```bash
gol ls
```

### 3. Start, Stop, and Restart Services
You can target services by their name or by their table ID (e.g., `0`, `1`):

```bash
# Start a stopped service
gol start web-server

# Stop a running service
gol stop 0

# Restart a service
gol restart web-server
```

### 4. Stream and Inspect Logs (`logs`)
Inspect recent log output or stream logs live with multi-stream support:

```bash
# View the last 50 lines (default)
gol logs web-server

# View the last 100 lines of stdout only
gol logs web-server -n 100 -o

# Stream logs in real-time (follow)
gol logs web-server -f
```

### 5. Remove a Service (`rm`)
Unloads the job from `launchd` and deletes its configuration and log tracking files:

```bash
gol rm web-server
```

---

## License

MIT
