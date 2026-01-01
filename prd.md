sshx — Product Requirements Document (PRD)

1. Overview

sshx is a terminal-first SSH helper tool designed to simplify and accelerate connecting to a large number of organizational servers that follow predictable naming conventions. It provides a fast, discoverable TUI (Terminal User Interface) for host selection, configuration, and connection, while preserving native SSH behavior and workflows.

The tool is implemented in Go, using the Bubble Tea + Lip Gloss ecosystem for the TUI, and standard system SSH tooling for execution.

Primary goals:
	•	Reduce cognitive load when connecting to remote servers
	•	Eliminate memorization of full hostnames and key paths
	•	Provide a scalable replacement for ad-hoc bash wrappers
	•	Offer a polished, modern terminal UX

⸻

2. Target Users
	•	Engineers working with many SSH destinations
	•	DevOps / Platform / Backend developers
	•	Power terminal users
	•	Users with multiple SSH keys and environments

Non-goals:
	•	Replacing SSH itself
	•	Acting as a secrets manager
	•	Managing credentials beyond SSH keys

⸻

3. Core Concepts

3.1 Destination

A destination represents a logical SSH target.

A destination resolves to:
	•	Hostname (FQDN or IP)
	•	SSH user
	•	SSH key
	•	Optional metadata (environment, group, tags)

3.2 Frecency

Destinations are ordered using a frecency score:
	•	Frequency: how often a destination is used
	•	Recency: how recently it was last used

This ensures commonly used and recently accessed hosts appear first.

⸻

4. CLI Usage & Behavior

4.1 sshx

Behavior:
	•	Launches the TUI
	•	Displays a list of configured destinations
	•	List is ordered by frecency
	•	Focus is placed on a filter input

User interactions:
	•	Typing filters the list (substring / fuzzy match)
	•	Arrow keys navigate the list
	•	Enter connects to selected destination
	•	Esc or q exits

⸻

4.2 sshx <dest_partial_name>

Behavior:
	1.	Load all configured destinations
	2.	Filter destinations using the provided partial name

Outcomes:
	•	1 match → immediately launch SSH
	•	>1 matches → launch TUI with filter prefilled
	•	0 matches → launch TUI in Configure Mode

⸻

4.3 sshx --configure <hostname>

Behavior:
	•	Launch TUI in Configure Mode
	•	Hostname field is prefilled
	•	User guided through destination setup

⸻

4.4 sshx --help

Behavior:
	•	Print a concise help menu
	•	Describe common usage patterns
	•	Exit without launching TUI

⸻

5. SSH Session Launching

When launching an SSH session:
	1.	Resolve destination configuration
	2.	Construct SSH command using safe defaults

Example flags:
	•	-i <keyfile>
	•	-o ServerAliveInterval=60
	•	-o ServerAliveCountMax=3
	•	-o ControlMaster=auto
	•	-o ControlPersist=5m

	3.	Replace the current process using exec
	4.	Preserve TTY, signals, and terminal state

⸻

6. Configure Mode

6.1 Purpose

Configure Mode is used to add or modify SSH destinations.

Triggered when:
	•	--configure is explicitly used
	•	No destination matches a provided name

⸻

6.2 Configuration Workflow

Steps:
	1.	Enter hostname
	2.	Detect naming pattern/group
	3.	Select or generate SSH key
	4.	Select SSH user
	5.	Confirm and save

⸻

6.3 SSH Key Handling

Behavior:
	•	Detect existing keys in ~/.ssh
	•	Allow selecting an existing key
	•	Option to generate a new key using ssh-keygen

Generated keys:
	•	Stored under ~/.ssh/sshx_<name>
	•	Proper permissions enforced

⸻

6.4 SSH Config Integration
	•	Entries added to ~/.ssh/config
	•	Managed section marked by sshx
	•	Idempotent updates

Example:

# BEGIN sshx
Host app-prod-01
  HostName app-prod-01.internal
  User deploy
  IdentityFile ~/.ssh/sshx_prod
# END sshx


⸻

7. TUI Design

7.1 Main Screen (Select Mode)

┌────────────────────────────────────────────┐
│ 🔐 sshx                                    │
├────────────────────────────────────────────┤
│ Filter: prod                               │
│                                            │
│ ▶ app-prod-01   [prod]   last: 2h ago      │
│   app-prod-02   [prod]   last: yesterday   │
│   db-prod-01    [prod]   last: 3d ago      │
│                                            │
│ ⏎ connect   c configure   q quit           │
└────────────────────────────────────────────┘


⸻

7.2 Configure Mode

┌────────────────────────────────────────────┐
│ ⚙ Configure Destination                   │
├────────────────────────────────────────────┤
│ Hostname: app-stage-03                     │
│ User: deploy                               │
│ SSH Key: sshx_stage                        │
│ Group: stage                               │
│                                            │
│ ✔ save   ✖ cancel                          │
└────────────────────────────────────────────┘


⸻

8. Data Storage

8.1 Local Config
	•	Stored under ~/.config/sshx/
	•	YAML or JSON format
	•	Includes:
	•	destinations
	•	usage metrics
	•	naming patterns

8.2 Usage Tracking

For each destination:
	•	last_connected_at
	•	connection_count

Used to compute frecency.

⸻

9. Naming Patterns & Groups

sshx can recognize naming patterns:

Examples:
	•	app-prod-XX
	•	db-stage-XX

Patterns allow:
	•	automatic grouping
	•	default user/key selection

⸻

10. Error Handling & UX Expectations
	•	Clear, human-readable errors
	•	Never corrupt ~/.ssh/config
	•	Graceful fallback to TUI on ambiguity
	•	Always leave terminal in a clean state

⸻

11. Non-Functional Requirements
	•	Fast startup (<100ms for local operations)
	•	Single static binary
	•	No background daemon
	•	Works with zsh, bash, fish

⸻

12. Future Enhancements (Out of Scope)
	•	tmux auto-attach
	•	Jump host support
	•	Remote bootstrap scripts
	•	Sync across machines
	•	Plugins

⸻

13. Success Criteria
	•	User can connect to any known host in <3 keystrokes
	•	Tool fully replaces previous SSH wrapper scripts
	•	No regressions vs native SSH experience

⸻

sshx aims to feel like a natural extension of SSH, not a replacement.
