package importer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sshx/internal/config"
	"strconv"
	"strings"
)

// ParseSSHConfig reads the SSH config file from the given path (or default ~/.ssh/config)
// and returns a list of Destinations.
func ParseSSHConfig(path string) ([]config.Destination, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home dir: %w", err)
		}
		path = filepath.Join(home, ".ssh", "config")
	} else if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home dir: %w", err)
		}
		path = filepath.Join(home, path[2:])
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []config.Destination{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var destinations []config.Destination
	var currentDest *config.Destination

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := strings.ToLower(parts[0])
		value := strings.Join(parts[1:], " ")

		switch key {
		case "host":
			// Save previous destination if it exists and is valid
			if currentDest != nil {
				if currentDest.Hostname == "" {
					currentDest.Hostname = currentDest.Alias
				}
				destinations = append(destinations, *currentDest)
			}

			// Check if it's a wildcard host which we want to ignore for specific destinations
			if strings.Contains(value, "*") || strings.Contains(value, "?") {
				currentDest = nil
				continue
			}

			// SSH config allows "Host alias1 alias2". We'll just take the first one for now
			// or create a new destination. Let's take the first alias.
			aliases := strings.Fields(value)
			currentDest = &config.Destination{
				Alias: aliases[0],
			}

		case "hostname":
			if currentDest != nil {
				currentDest.Hostname = value
			}

		case "user":
			if currentDest != nil {
				currentDest.User = value
			}

		case "port":
			if currentDest != nil {
				if p, err := strconv.Atoi(value); err == nil {
					currentDest.Port = p
				}
			}

		case "identityfile":
			if currentDest != nil {
				// Expand ~ in identity file
				if strings.HasPrefix(value, "~/") {
					home, _ := os.UserHomeDir()
					value = filepath.Join(home, value[2:])
				}
				currentDest.Key = value
			}
		}
	}

	// Add the last one
	if currentDest != nil {
		if currentDest.Hostname == "" {
			currentDest.Hostname = currentDest.Alias
		}
		destinations = append(destinations, *currentDest)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return destinations, nil
}
