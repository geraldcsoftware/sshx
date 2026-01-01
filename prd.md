sshx — Product Requirements Document (PRD)

1. Overview

sshx is a terminal-first SSH helper tool designed to simplify and accelerate connecting to a large number of organizational servers. It provides a fast, discoverable TUI (Terminal User Interface) for host selection, configuration, and connection, while preserving native SSH behavior and workflows.

The tool is implemented in Go, using the Bubble Tea + Lip Gloss ecosystem for the TUI, and standard system SSH tooling for execution.

Primary goals:
  • Reduce cognitive load when connecting to remote servers
  • Eliminate memorization of full hostnames and key paths
  • Provide a scalable replacement for ad-hoc bash wrappers
  • Offer a polished, modern terminal UX

---

2. Target Users

  • Engineers working with many SSH destinations
  • DevOps / Platform / Backend developers
  • Power terminal users
  • Users with multiple SSH keys and environments

Non-goals:
  • Replacing SSH itself
  • Acting as a secrets manager
  • Managing credentials beyond SSH keys

---

3. Core Concepts

3.1 Destination

A destination represents a logical SSH target.

A destination consists of:
  • Alias (display name, used for filtering and selection)
  • Hostname (FQDN or IP, can differ from alias)
  • SSH user
  • SSH key path
  • Group (manually assigned, optional)

3.2 Frecency

Destinations are ordered using a Mozilla-style frecency algorithm:

  score = connection_count × recency_weight

Where recency_weight decays based on time buckets:
  • Last 4 hours: weight = 100
  • Last 24 hours: weight = 70
  • Last 7 days: weight = 50
  • Last 30 days: weight = 30
  • Older: weight = 10

This ensures commonly used and recently accessed hosts appear first.

3.3 Filtering

Destinations are filtered using fuzzy matching (fzf-style):
  • Tolerates typos and matches non-contiguous characters
  • Example: "apd" matches "app-prod-01"
  • Case-insensitive by default

---

4. CLI Usage & Behavior

4.1 sshx

Behavior:
  • Launches the TUI
  • Displays a list of configured destinations
  • List is ordered by frecency
  • Filter input is auto-focused (ready to type immediately)

User interactions:
  • Typing filters the list (fuzzy match)
  • Arrow keys or j/k navigate the list
  • Enter connects to selected destination
  • c opens Configure Mode to add new destination
  • d deletes selected destination (with confirmation)
  • Esc or q exits

---

4.2 sshx <dest_partial_name>

Behavior:
  1. Load all configured destinations
  2. Filter destinations using fuzzy matching on provided partial name

Outcomes:
  • 1 match → immediately launch SSH
  • >1 matches → launch TUI with filter prefilled
  • 0 matches → launch TUI in Configure Mode with hostname prefilled

---

4.3 sshx --configure <hostname>

Behavior:
  • Launch TUI in Configure Mode
  • Hostname field is prefilled
  • User guided through destination setup

---

4.4 sshx --help

Behavior:
  • Print a concise help menu
  • Describe common usage patterns
  • Exit without launching TUI

---

4.5 sshx --version

Behavior:
  • Print version number and build information
  • Exit without launching TUI

---

5. SSH Session Launching

When launching an SSH session:
  1. Resolve destination configuration
  2. Update frecency metrics (increment count, update timestamp)
  3. Construct SSH command using safe defaults

SSH flags used:
  • -i <keyfile>
  • -o ServerAliveInterval=60
  • -o ServerAliveCountMax=3
  • -o ControlMaster=auto
  • -o ControlPersist=5m

Port: Always uses standard port 22 (no custom port support).

  4. Replace the current process using exec
  5. Preserve TTY, signals, and terminal state

Connection errors are handled natively by SSH (no pre-flight checks).

---

6. Configure Mode

6.1 Purpose

Configure Mode is used to add new SSH destinations.

Note: Editing existing destinations is not supported. To modify a destination, delete it and recreate it.

Triggered when:
  • --configure is explicitly used
  • No destination matches a provided name

---

6.2 Configuration Workflow

Steps:
  1. Enter alias (display name for the destination)
     • Must be unique across all destinations.
  2. Enter hostname (FQDN or IP, defaults to alias if not specified)
  3. Select SSH user
     • Resolution order: Explicit destination override > Group default > Global default.
  4. Select or generate SSH key
  5. Assign group (optional, manual assignment only)
     • A destination can belong to exactly one group.
  6. Confirm and save

---

6.3 SSH Key Handling

Behavior:
  • Detect existing keys in ~/.ssh
  • Allow selecting an existing key
  • Option to generate a new Ed25519 key using ssh-keygen

Directory handling:
  • If ~/.ssh does not exist, create it with permissions 700
  • If ~/.ssh/config.d does not exist, create it with permissions 700

Generated keys:
  • Type: Ed25519 (no other types supported)
  • Stored under ~/.ssh/sshx_<normalized_hostname>
  • <normalized_hostname> is derived from the hostname by replacing special characters (e.g., ".", "-") with underscores (e.g., "prod.server.1" becomes "sshx_prod_server_1").
  • Proper permissions enforced (600)

---

6.4 SSH Config Integration

Approach:
  • Create a dedicated file: ~/.ssh/config.d/sshx
  • Add Include directive to ~/.ssh/config if not present
  • Managed section is idempotent

Include directive (added to the TOP of ~/.ssh/config to ensure precedence):

Include config.d/*

Managed file (~/.ssh/config.d/sshx):

# Managed by sshx - do not edit manually
Host app-prod-01
  HostName app-prod-01.internal
  User deploy
  IdentityFile ~/.ssh/sshx_prod

Host db-prod-01
  HostName db-prod-01.internal
  User deploy
  IdentityFile ~/.ssh/sshx_prod

Sync behavior:
  • When a destination is deleted, its entry is removed from the managed SSH config file

---

7. TUI Design

7.1 Color Theme

Purple/magenta modern palette:
  • Primary accent: Magenta (#FF00FF or similar)
  • Secondary accent: Purple (#9B59B6)
  • Selection highlight: Bright magenta background
  • Group tags: Purple text
  • Borders: Dim purple
  • Text: White on dark background

7.2 Main Screen (Select Mode)

┌────────────────────────────────────────────┐
│ sshx                                       │
├────────────────────────────────────────────┤
│ Filter: prod█                              │
│                                            │
│ ▶ app-prod-01   [prod]   last: 2h ago      │
│   app-prod-02   [prod]   last: yesterday   │
│   db-prod-01    [prod]   last: 3d ago      │
│                                            │
│ ⏎ connect  c new  d delete  q quit         │
└────────────────────────────────────────────┘

Key behaviors:
  • Filter input is auto-focused on launch
  • List scrolls if destinations exceed visible area
  • Selected item highlighted with accent color
  • Empty state: When no destinations match filter, display "No matches. Press c to add a new destination."

---

7.3 Configure Mode

┌────────────────────────────────────────────┐
│ New Destination                            │
├────────────────────────────────────────────┤
│ Alias: app-stage-03                        │
│ Hostname: app-stage-03.internal            │
│ User: deploy                               │
│ SSH Key: sshx_stage                        │
│ Group: stage                               │
│                                            │
│ [Save]  [Cancel]                           │
└────────────────────────────────────────────┘

---

7.4 Delete Confirmation

┌────────────────────────────────────────────┐
│ Delete Destination?                        │
├────────────────────────────────────────────┤
│                                            │
│ Are you sure you want to delete            │
│ "app-prod-01"?                             │
│                                            │
│ This will also remove it from SSH config.  │
│                                            │
│ [Yes, Delete]  [Cancel]                    │
└────────────────────────────────────────────┘

---

7.5 Keyboard Shortcuts

| Key        | Action                              |
|------------|-------------------------------------|
| ↑ / k      | Move selection up                   |
| ↓ / j      | Move selection down                 |
| Enter      | Connect to selected destination     |
| c          | Open Configure Mode (add new)       |
| d          | Delete selected destination         |
| Esc / q    | Quit application                    |
| (typing)   | Filter destinations (fuzzy match)   |

---

8. Data Storage

8.1 Local Config

Location: ~/.config/sshx/config.yaml

Format: YAML

Structure:

# Global settings
defaults:
  user: deploy

# Group definitions with group-specific defaults
groups:
  prod:
    user: admin
  stage:
    user: deploy

# Destination list
destinations:
  - alias: app-prod-01
    hostname: app-prod-01.internal
    user: deploy # Explicit override (takes precedence)
    key: ~/.ssh/sshx_prod_server_1
    group: prod
    last_connected_at: 2024-01-15T10:30:00Z
    connection_count: 47

  - alias: db-prod-01
    hostname: db-prod-01.internal
    user: admin
    key: ~/.ssh/sshx_db
    group: prod
    last_connected_at: 2024-01-14T08:00:00Z
    connection_count: 12

8.2 Usage Tracking

For each destination:
  • last_connected_at: ISO 8601 timestamp
  • connection_count: integer

Used to compute frecency. No separate history log is maintained.

---

9. Groups

Groups provide visual organization and shared configuration defaults.

Behavior:
  • Manual assignment only (no auto-detection from hostname patterns).
  • Optional field: A destination can belong to exactly one group, or none.
  • Configuration Resolution: Groups can define default properties (like SSH user) that apply to all members unless overridden at the destination level.
  • Displayed as tags in the TUI: [prod], [stage], etc.
  • Groups are not predefined; created implicitly when assigned.

---

10. Error Handling & UX Expectations

  • Clear, human-readable error messages
  • Never corrupt user's SSH configuration files
  • Graceful fallback to TUI on ambiguity
  • Always leave terminal in a clean state on exit
  • Validate inputs before saving (hostname format, key file exists, etc.)

---

11. Non-Functional Requirements

  • Fast startup (<100ms for local operations)
  • Single static binary (no external dependencies)
  • No background daemon
  • Works with zsh, bash, fish
  • Minimal memory footprint

---

12. Future Enhancements (Out of Scope)

  • tmux auto-attach
  • Jump host / bastion support
  • Remote bootstrap scripts
  • Sync across machines
  • Plugins
  • Custom SSH ports
  • Editing existing destinations (delete + recreate workflow)
  • Auto-grouping based on hostname patterns

---

13. Success Criteria

  • User can connect to any known host in <3 keystrokes
  • Tool fully replaces previous SSH wrapper scripts
  • No regressions vs native SSH experience
  • TUI renders correctly in standard terminal emulators (iTerm2, Terminal.app, Alacritty, etc.)

---

sshx aims to feel like a natural extension of SSH, not a replacement.
