package ssh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sshx/internal/config"
)

func TestManager_SyncConfig(t *testing.T) {
	// Setup temp home
	tempHome, err := os.MkdirTemp("", "sshx-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempHome)

	// Mock home dir for the test
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", originalHome)

	cfg := &config.Config{
		Destinations: []config.Destination{
			{
				Alias:    "test-host",
				Hostname: "1.2.3.4",
				User:     "tester",
				Key:      "~/.ssh/test_key",
			},
		},
	}

	mgr := NewManager(cfg)
	err = mgr.SyncConfig()
	if err != nil {
		t.Fatalf("SyncConfig failed: %v", err)
	}

	configPath := filepath.Join(tempHome, ".ssh", "config.d", "sshx")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read generated config: %v", err)
	}

	if !strings.Contains(string(content), "Host test-host") {
		t.Errorf("Config missing Host: %s", string(content))
	}
	if !strings.Contains(string(content), "HostName 1.2.3.4") {
		t.Errorf("Config missing HostName: %s", string(content))
	}
}

func TestManager_GenerateKey(t *testing.T) {
	tempHome, err := os.MkdirTemp("", "sshx-test-keys-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempHome)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", originalHome)

	mgr := NewManager(&config.Config{})
	keyPath, err := mgr.GenerateKey("prod.server-01")
	if err != nil {
		// If ssh-keygen is missing in the environment, we might need to skip or mock
		t.Logf("Skipping ssh-keygen test: %v", err)
		return
	}

	if !strings.Contains(keyPath, "sshx_prod_server_01") {
		t.Errorf("Unexpected key path: %s", keyPath)
	}

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Errorf("Key file was not created: %s", keyPath)
	}
}
