package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"sshx/internal/config"
	"sshx/internal/importer"
	"sshx/internal/ssh"
	"sshx/internal/tui"
)

var (
	version = "0.0.6"
	help    = `sshx - Terminal-first SSH helper

Usage:
  sshx [filter]         Launch TUI, optionally filtering by name
  sshx --configure <host>  Launch TUI in configure mode for host
  sshx --import         Import hosts from ~/.ssh/config
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
	importFlag := flag.Bool("import", false, "Import hosts from ~/.ssh/config")
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

	if *importFlag {
		imported, err := importer.ParseSSHConfig("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error importing config: %v\n", err)
			os.Exit(1)
		}

		// Dedup
		existing := make(map[string]bool)
		for _, d := range cfg.Destinations {
			existing[d.Alias] = true
		}

		count := 0
		for _, d := range imported {
			if !existing[d.Alias] {
				cfg.Destinations = append(cfg.Destinations, d)
				existing[d.Alias] = true
				count++
			}
		}

		if count > 0 {
			if err := cfg.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
				os.Exit(1)
			}
		}

		fmt.Printf("Imported %d new hosts from ~/.ssh/config\n", count)
		os.Exit(0)
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
	if d.Port != 0 {
		args = append(args, "-p", fmt.Sprintf("%d", d.Port))
	} else {
		// Default port
		// While ssh defaults to 22, explicit default helps if configs are messed up
		// or if we want to change default later.
		// However, passing -p 22 is safe.
		// For now we will rely on ssh default or explicit override.
		// Wait, user asked for "defaulting to 22, allowing override".
		// SSH client defaults to 22 anyway.
		// If we want to be explicit:
		// args = append(args, "-p", "22")
	}

	// Standard flags from PRD + sensible defaults
	args = append(args,
		"-o", "ConnectTimeout=10",
		"-o", "ServerAliveInterval=60",
		"-o", "ServerAliveCountMax=3",
		"-o", "TCPKeepAlive=yes",
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

	// Filter out existing TERM from env to avoid leaking it
	newEnv := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, "TERM=") {
			newEnv = append(newEnv, e)
		}
	}

	// Add configured TERM or default
	term := cfg.ResolveTerm()
	newEnv = append(newEnv, fmt.Sprintf("TERM=%s", term))

	if err := syscall.Exec(sshBin, args, newEnv); err != nil {
		fmt.Fprintf(os.Stderr, "Error executing ssh: %v\n", err)
		os.Exit(1)
	}
}
