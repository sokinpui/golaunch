package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sokinpui/golaunch/internal/manager"
	"github.com/sokinpui/golaunch/internal/ui"
)

func main() {
	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "gol",
		Short:         "Native macOS Launchd Process Manager",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(
		newListCmd(),
		newAddCmd(),
		newStartCmd(),
		newStopCmd(),
		newRestartCmd(),
		newRemoveCmd(),
		newLogsCmd(),
	)

	return cmd
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List all managed services",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := manager.NewManager()
			if err != nil {
				ui.PrintError(err)
				return err
			}
			services, err := mgr.List()
			if err != nil {
				ui.PrintError(err)
				return err
			}
			fmt.Println(ui.RenderServicesTable(services))
			return nil
		},
	}
}

func newAddCmd() *cobra.Command {
	var (
		cwd     string
		envList []string
	)

	cmd := &cobra.Command{
		Use:   `add <name> [flags] "<command>"`,
		Short: "Register and start a new service",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			rawCmd := strings.Join(args[1:], " ")
			cmdArgs, err := splitCommandLine(rawCmd)
			if err != nil {
				ui.PrintError(err)
				return err
			}
			if len(cmdArgs) == 0 {
				return errors.New("command arguments cannot be empty")
			}

			mgr, err := manager.NewManager()
			if err != nil {
				ui.PrintError(err)
				return err
			}

			envMap := make(map[string]string)
			for _, env := range envList {
				parts := strings.SplitN(env, "=", 2)
				if len(parts) == 2 {
					envMap[parts[0]] = parts[1]
				}
			}

			if err := mgr.Add(name, cmdArgs, cwd, envMap); err != nil {
				ui.PrintError(err)
				return err
			}

			ui.PrintSuccess(fmt.Sprintf("Service '%s' registered and started", name))
			return nil
		},
	}

	cmd.Flags().StringVar(&cwd, "cwd", "", "Working directory")
	cmd.Flags().StringArrayVar(&envList, "env", nil, "Environment variable in KEY=VALUE format")
	return cmd
}

func newStartCmd() *cobra.Command {
	return newLifecycleCmd("start", "Start a stopped service", func(m *manager.Manager, name string) error {
		return m.Start(name)
	})
}

func newStopCmd() *cobra.Command {
	return newLifecycleCmd("stop", "Stop a running service", func(m *manager.Manager, name string) error {
		return m.Stop(name)
	})
}

func newRestartCmd() *cobra.Command {
	return newLifecycleCmd("restart", "Restart a service", func(m *manager.Manager, name string) error {
		return m.Restart(name)
	})
}

func newRemoveCmd() *cobra.Command {
	cmd := newLifecycleCmd("rm", "Remove a service and its plist", func(m *manager.Manager, name string) error {
		return m.Remove(name)
	})
	cmd.Aliases = []string{"delete"}
	return cmd
}

func newLifecycleCmd(action, desc string, fn func(*manager.Manager, string) error) *cobra.Command {
	return &cobra.Command{
		Use:   fmt.Sprintf("%s <name>", action),
		Short: desc,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			mgr, err := manager.NewManager()
			if err != nil {
				ui.PrintError(err)
				return err
			}
			name, err := mgr.ResolveTarget(target)
			if err != nil {
				ui.PrintError(err)
				return err
			}
			if err := fn(mgr, name); err != nil {
				ui.PrintError(err)
				return err
			}
			ui.PrintSuccess(fmt.Sprintf("Service '%s' %sed successfully", name, action))
			return nil
		},
	}
}

func newLogsCmd() *cobra.Command {
	var (
		lines  int
		follow bool
	)

	cmd := &cobra.Command{
		Use:   "logs <name>",
		Short: "Inspect or tail service logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			mgr, err := manager.NewManager()
			if err != nil {
				ui.PrintError(err)
				return err
			}
			name, err := mgr.ResolveTarget(target)
			if err != nil {
				ui.PrintError(err)
				return err
			}
			if err := mgr.Logs(name, lines, follow); err != nil {
				ui.PrintError(err)
				return err
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&lines, "lines", "n", 50, "Number of lines to display")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log stream")
	return cmd
}

func splitCommandLine(command string) ([]string, error) {
	var args []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for _, r := range command {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && !inSingleQuote {
			escaped = true
			continue
		}
		if r == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if r == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if (r == ' ' || r == '\t') && !inSingleQuote && !inDoubleQuote {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if inSingleQuote || inDoubleQuote {
		return nil, errors.New("unclosed quote in command string")
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args, nil
}
