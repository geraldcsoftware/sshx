package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"sshx/internal/config"
)

// CheckResult holds the result of a host check
type CheckResult struct {
	Alias   string
	Status  config.Status
	Message string
}

// CheckHost performs a connectivity and fingerprint check on the given hostname.
func CheckHost(alias, hostname string, timeout time.Duration) CheckResult {
	res := CheckResult{Alias: alias}

	// 1. Run ssh-keyscan
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	keyscanCmd := exec.CommandContext(ctx, "ssh-keyscan", "-q", hostname)
	var scanOut bytes.Buffer
	keyscanCmd.Stdout = &scanOut
	keyscanCmd.Stderr = io.Discard

	if err := keyscanCmd.Run(); err != nil {
		res.Status = config.StatusOffline
		res.Message = "Host unreachable"
		return res
	}

	if scanOut.Len() == 0 {
		res.Status = config.StatusOffline
		res.Message = "No keys returned"
		return res
	}

	// 2. Parse current fingerprint
	fingerprintCmd := exec.Command("ssh-keygen", "-q", "-lf", "-")
	fingerprintCmd.Stdin = &scanOut
	var fingerprintOut bytes.Buffer
	fingerprintCmd.Stdout = &fingerprintOut
	fingerprintCmd.Stderr = io.Discard

	if err := fingerprintCmd.Run(); err != nil {
		res.Status = config.StatusKeyError
		res.Message = "Failed to parse current keys"
		return res
	}

	currentFingerprints := parseFingerprints(fingerprintOut.String())

	if len(currentFingerprints) == 0 {
		res.Status = config.StatusKeyError
		res.Message = "No valid fingerprints found"
		return res
	}

	// 3. Get known_hosts fingerprint
	home, err := os.UserHomeDir()
	if err != nil {
		res.Status = config.StatusKeyError
		res.Message = "Cannot find home dir"
		return res
	}
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")

	knownCmd := exec.Command("ssh-keygen", "-q", "-F", hostname, "-f", knownHostsPath, "-l")
	var knownOut bytes.Buffer
	knownCmd.Stdout = &knownOut
	knownCmd.Stderr = io.Discard

	if err := knownCmd.Run(); err != nil {
		res.Status = config.StatusKeyError
		res.Message = "Not in known_hosts or error"
		return res
	}

	if knownOut.Len() == 0 {
		res.Status = config.StatusKeyError
		res.Message = "Host not known"
		return res
	}

	knownFingerprints := parseFingerprints(knownOut.String())

	// 4. Compare
	match := false
	for _, curr := range currentFingerprints {
		for _, known := range knownFingerprints {
			if curr == known {
				match = true
				break
			}
		}
		if match {
			break
		}
	}

	if match {
		res.Status = config.StatusOk
		res.Message = "Fingerprint verified"
	} else {
		res.Status = config.StatusKeyError
		res.Message = fmt.Sprintf("Fingerprint mismatch. Current: %s, Known: %s",
			firstOrEmpty(currentFingerprints), firstOrEmpty(knownFingerprints))
	}

	return res
}

func parseFingerprints(output string) []string {
	var fps []string
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		for _, f := range fields {
			if strings.HasPrefix(f, "SHA256:") || strings.HasPrefix(f, "MD5:") {
				fps = append(fps, f)
				break
			}
		}
	}
	return fps
}

func firstOrEmpty(s []string) string {
	if len(s) > 0 {
		return s[0]
	}
	return "?"
}
