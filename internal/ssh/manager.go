package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"sshx/internal/config"
)

const sshConfigTemplate = `# Managed by sshx - do not edit manually
{{range .}}
Host {{.Alias}}
  HostName {{.Hostname}}
{{if .Port}}  Port {{.Port}}{{end}}
  User {{.User}}
  IdentityFile {{.Key}}
{{end}}
`

// Manager handles SSH configuration and key operations
type Manager struct {
	cfg *config.Config
}

// NewManager creates a new SSH manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg}
}

// SyncConfig writes the managed SSH config file
func (m *Manager) SyncConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}

	configDir := filepath.Join(home, ".ssh", "config.d")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config.d: %w", err)
	}

	// Prepare data for template
	type hostEntry struct {
		Alias    string
		Hostname string
		Port     int
		User     string
		Key      string
	}

	var entries []hostEntry
	for _, d := range m.cfg.Destinations {
		entries = append(entries, hostEntry{
			Alias:    d.Alias,
			Hostname: d.Hostname,
			Port:     d.Port,
			User:     m.cfg.ResolveUser(&d),
			Key:      d.Key,
		})
	}

	// Generate config content
	tmpl, err := template.New("ssh_config").Parse(sshConfigTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	f, err := os.OpenFile(filepath.Join(configDir, "sshx"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open managed config file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, entries); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// GenerateKey generates a new Ed25519 key pair
func (m *Manager) GenerateKey(hostname string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home dir: %w", err)
	}

	// Normalize hostname for filename
	normalized := strings.ReplaceAll(hostname, ".", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")
	keyName := fmt.Sprintf("sshx_%s", normalized)
	keyPath := filepath.Join(home, ".ssh", keyName)

	// Check if key already exists
	if _, err := os.Stat(keyPath); err == nil {
		return keyPath, nil // Key already exists, return it
	}

	// Ensure .ssh exists
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create .ssh dir: %w", err)
	}

	// Run ssh-keygen
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-C", "sshx-generated")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ssh-keygen failed: %s: %w", string(output), err)
	}

	return keyPath, nil
}

// ListKeys returns a list of private keys in ~/.ssh/
func (m *Manager) ListKeys() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	sshDir := filepath.Join(home, ".ssh")

	entries, err := os.ReadDir(sshDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var keys []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// Filter out common non-key files
		if strings.HasSuffix(name, ".pub") ||
			name == "known_hosts" ||
			name == "config" ||
			name == "authorized_keys" ||
			name == "environment" ||
			strings.HasPrefix(name, "known_hosts") {
			continue
		}

		keys = append(keys, filepath.Join(sshDir, name))
	}
	return keys, nil
}
