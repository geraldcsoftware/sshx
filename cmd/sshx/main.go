package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"sshx/internal/config"
	"sshx/internal/ssh"
	"sshx/internal/tui"
)

var (
	version = "0.1.0"
	help    = `sshx - Terminal-first SSH helper

Usage:
  sshx [filter]         Launch TUI, optionally filtering by name
  sshx --configure <host>  Launch TUI in configure mode for host
  sshx --help           Show this help message
  sshx --version        Show version information

Keybindings:
  Type to filter
  Enter      Connect
  Ctrl+N     New Destination
  Ctrl+D     Delete Destination
  Esc/q      Quit
`
)

func main() {
	configureFlag := flag.String("configure", "", "Configure a new host")
	helpFlag := flag.Bool("help", false, "Show help")
	versionFlag := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *helpFlag {
		fmt.Print(help)
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Printf("sshx version %s\n", version)
		os.Exit(0)
	}

	// Load Config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Init SSH Manager
	sshMgr := ssh.NewManager(cfg)

	// Determine initial filter
	initialFilter := ""
	if flag.NArg() > 0 {
		initialFilter = flag.Arg(0)
	}

	// Check for single match to connect immediately
	// Only if NOT configuring and there IS a filter
	if initialFilter != "" && *configureFlag == "" {
		var matches []*config.Destination
		for i := range cfg.Destinations {
			if cfg.Destinations[i].Matches(initialFilter) {
				matches = append(matches, &cfg.Destinations[i])
			}
		}

		if len(matches) == 1 {
			connect(cfg, matches[0])
			return
		}
	}

	// Start TUI
	// configureFlag value is passed to handle --configure <host>
	p := tea.NewProgram(tui.NewModel(cfg, sshMgr, initialFilter, *configureFlag), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	// Check result
	if model, ok := m.(tui.Model); ok && model.SelectedDest != nil {
		connect(cfg, model.SelectedDest)
	}
}

func connect(cfg *config.Config, d *config.Destination) {
	// Update stats
	d.ConnectionCount++
	d.LastConnectedAt = time.Now()
	cfg.Save()

	// Construct SSH command
	user := cfg.ResolveUser(d)

	// Arguments
	args := []string{"ssh", d.Hostname}
	if user != "" {
		args = append(args, "-l", user)
	}
	if d.Key != "" {
		args = append(args, "-i", d.Key)
	}

	// Standard flags from PRD
	args = append(args, 
		"-o", "ServerAliveInterval=60",
		"-o", "ServerAliveCountMax=3",
		"-o", "ControlMaster=auto",
		"-o", "ControlPersist=5m",
	)

	// Find ssh binary
	sshBin, err := exec.LookPath("ssh")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: ssh binary not found: %v\n", err)
		os.Exit(1)
	}

	// Replace process
	// Note: syscall.Exec requires the full path to the binary as the first arg,
	// and the slice of arguments (including the command name) as the second.
	// Environment is passed as third arg.
	env := os.Environ()
	if err := syscall.Exec(sshBin, args, env); err != nil {
		fmt.Fprintf(os.Stderr, "Error executing ssh: %v\n", err)
		os.Exit(1)
	}
}