package importer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSSHConfig(t *testing.T) {
	// Create a temporary ssh config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")

	content := `
Host test-server
    HostName 192.168.1.1
    User admin
    IdentityFile ~/.ssh/id_rsa

Host prod
    HostName prod.example.com
    User deploy

Host *
    User root
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	dests, err := ParseSSHConfig(configPath)
	if err != nil {
		t.Fatalf("ParseSSHConfig failed: %v", err)
	}

	if len(dests) != 2 {
		t.Errorf("expected 2 destinations, got %d", len(dests))
	}

	// Verify test-server
	foundTestServer := false
	for _, d := range dests {
		if d.Alias == "test-server" {
			foundTestServer = true
			if d.Hostname != "192.168.1.1" {
				t.Errorf("expected HostName 192.168.1.1, got %s", d.Hostname)
			}
			if d.User != "admin" {
				t.Errorf("expected User admin, got %s", d.User)
			}
			// We can't easily check IdentityFile expansion as it depends on user home, 
			// but we can check it's not empty/literal if we mocked home, 
			// but here we just check it was parsed.
			if d.Key == "" {
				t.Error("expected Key to be set")
			}
		} else if d.Alias == "prod" {
			if d.Hostname != "prod.example.com" {
				t.Errorf("expected HostName prod.example.com, got %s", d.Hostname)
			}
			if d.User != "deploy" {
				t.Errorf("expected User deploy, got %s", d.User)
			}
		}
	}

	if !foundTestServer {
		t.Error("test-server not found in parsed destinations")
	}
}
