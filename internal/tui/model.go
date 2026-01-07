package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"sshx/internal/config"
	"sshx/internal/ssh"
)

// Mode represents the current state of the TUI
type Mode int

const (
	ModeSelect Mode = iota
	ModeConfigure
	ModeDeleteConfirm
)

// Styles
var (
	primaryColor   = lipgloss.Color("#FF00FF") // Magenta
	secondaryColor = lipgloss.Color("#9B59B6") // Purple
	subtleColor    = lipgloss.Color("#626262") // Grey

	titleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			Padding(0, 1)

	selectedItemStyle = lipgloss.NewStyle().
				Background(primaryColor).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1)

	itemStyle = lipgloss.NewStyle().
				Padding(0, 1)

	groupStyle = lipgloss.NewStyle().
				Foreground(secondaryColor)

	helpStyle = lipgloss.NewStyle().
				Foreground(subtleColor).
				PaddingTop(1)
)

type Model struct {
	cfg        *config.Config
	sshManager *ssh.Manager
	mode       Mode

	// Select Mode State
	filterInput  textinput.Model
	filteredList []config.Destination
	cursor       int
	
	// Configure Mode State
	configInputs []textinput.Model
	configFocus  int

	// Delete Mode State
	deleteTarget *config.Destination

	// Output
	SelectedDest *config.Destination // Set when user selects a destination to connect
	Quitting     bool
}

func NewModel(cfg *config.Config, sshMgr *ssh.Manager, initialFilter string, configureHost string) Model {
	ti := textinput.New()
	ti.Placeholder = "Filter destinations..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40
	ti.SetValue(initialFilter)

	// Configure inputs
	inputs := make([]textinput.Model, 5)
	labels := []string{"Alias", "Hostname", "User", "SSH Key", "Group"}
	for i := range inputs {
		t := textinput.New()
		t.Placeholder = labels[i]
		t.CharLimit = 64
		t.Width = 40
		inputs[i] = t
	}

	m := Model{
		cfg:          cfg,
		sshManager:   sshMgr,
		mode:         ModeSelect,
		filterInput:  ti,
		configInputs: inputs,
	}

	m.updateFilteredList()
	
	// Handle explicit configure mode or no matches
	if configureHost != "" {
		m.mode = ModeConfigure
		m.configInputs[0].SetValue(configureHost)
		m.configInputs[1].SetValue(configureHost)
		m.configInputs[0].Focus()
	} else if initialFilter != "" && len(m.filteredList) == 0 {
		m.mode = ModeConfigure
		m.configInputs[0].SetValue(initialFilter) // Set Alias
		m.configInputs[1].SetValue(initialFilter) // Set Hostname (default)
		m.configInputs[0].Focus()
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case ModeSelect:
			return m.updateSelect(msg)
		case ModeConfigure:
			return m.updateConfigure(msg)
		case ModeDeleteConfirm:
			return m.updateDeleteConfirm(msg)
		}
	}

	// Handle blinking cursor for inputs
	if m.mode == ModeSelect {
		m.filterInput, cmd = m.filterInput.Update(msg)
	} else if m.mode == ModeConfigure {
		cmds := make([]tea.Cmd, len(m.configInputs))
		for i := range m.configInputs {
			m.configInputs[i], cmds[i] = m.configInputs[i].Update(msg)
		}
		cmd = tea.Batch(cmds...)
	}

	return m, cmd
}

func (m Model) View() string {
	if m.Quitting {
		return ""
	}

	switch m.mode {
	case ModeSelect:
		return m.viewSelect()
	case ModeConfigure:
		return m.viewConfigure()
	case ModeDeleteConfirm:
		return m.viewDeleteConfirm()
	}
	return ""
}

// --- Select Mode Logic ---

func (m Model) updateSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.Quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.filteredList)-1 {
			m.cursor++
		}
	case "enter":
		if len(m.filteredList) > 0 {
			m.SelectedDest = &m.filteredList[m.cursor]
			return m, tea.Quit
		}
	case "ctrl+n":
		m.mode = ModeConfigure
		m.configFocus = 0
		m.configInputs[0].Focus()
	case "ctrl+d":
		if len(m.filteredList) > 0 {
			m.deleteTarget = &m.filteredList[m.cursor]
			m.mode = ModeDeleteConfirm
		}
	}
	
	// Pass to text input if not handled
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.updateFilteredList()
	return m, cmd
}

func (m *Model) updateFilteredList() {
	filter := m.filterInput.Value()
	m.filteredList = []config.Destination{}
	
	// Sort config destinations by frecency first
	m.cfg.SortDestinations()

	for _, d := range m.cfg.Destinations {
		if d.Matches(filter) {
			m.filteredList = append(m.filteredList, d)
		}
	}
	
	// Reset cursor if out of bounds
	if m.cursor >= len(m.filteredList) {
		m.cursor = max(0, len(m.filteredList)-1)
	}
}

func (m Model) viewSelect() string {
	s := titleStyle.Render("sshx") + "\n"
	s += fmt.Sprintf("Filter: %s\n\n", m.filterInput.View())

	for i, d := range m.filteredList {
		cursor := "  "
		style := itemStyle
		if i == m.cursor {
			cursor = "▶ "
			style = selectedItemStyle
		}

		group := ""
		if d.Group != "" {
			group = groupStyle.Render(fmt.Sprintf("[%s] ", d.Group))
		}

		timeStr := timeAgo(d.LastConnectedAt)
		// Basic padding for alignment - can be improved but keeps it simple
		line := fmt.Sprintf("%s%s %s%s  (last: %s)", cursor, d.Alias, group, d.Hostname, timeStr)
		s += style.Render(line) + "\n"
	}

	if len(m.filteredList) == 0 {
		s += "\nNo matches. Press Ctrl+N to add new.\n"
	}

	s += helpStyle.Render("\n⏎ connect  Ctrl+N new  Ctrl+D delete  Esc quit")
	return s
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	diff := time.Since(t)
	if diff < time.Hour {
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	}
	if diff < 48*time.Hour {
		return "yesterday"
	}
	return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
}

// --- Configure Mode Logic ---

func (m Model) updateConfigure(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = ModeSelect
		return m, nil
	case "tab":
		m.configInputs[m.configFocus].Blur()
		m.configFocus = (m.configFocus + 1) % len(m.configInputs)
		m.configInputs[m.configFocus].Focus()
		return m, nil
	case "shift+tab":
		m.configInputs[m.configFocus].Blur()
		m.configFocus = (m.configFocus - 1 + len(m.configInputs)) % len(m.configInputs)
		m.configInputs[m.configFocus].Focus()
		return m, nil
	case "enter":
		if m.configFocus == len(m.configInputs)-1 {
			// Save
			m.saveDestination()
			m.mode = ModeSelect
			m.updateFilteredList()
			return m, nil
		}
		// Move to next field
		m.configInputs[m.configFocus].Blur()
		m.configFocus++
		m.configInputs[m.configFocus].Focus()
		return m, nil
	}

	var cmd tea.Cmd
	m.configInputs[m.configFocus], cmd = m.configInputs[m.configFocus].Update(msg)
	return m, cmd
}

func (m *Model) saveDestination() {
	alias := m.configInputs[0].Value()
	hostname := m.configInputs[1].Value()
	user := m.configInputs[2].Value()
	key := m.configInputs[3].Value()
	group := m.configInputs[4].Value()

	if alias == "" { return } // Basic validation
	if hostname == "" { hostname = alias }

	// Generate key if needed (simple check: if it looks like a path, use it, else generate)
	// For now, if key is empty or just a name, we might want to generate.
	// PRD: "Option to generate a new Ed25519 key".
	// I'll assume if they type "generate" or leave it blank, we generate?
	// Or maybe I should have a toggle.
	// For simplicity: If key doesn't start with /, ~, or ., assume it's a name to generate.
	if key == "" || (!strings.HasPrefix(key, "/") && !strings.HasPrefix(key, ".") && !strings.HasPrefix(key, "~")) {
		generatedKey, err := m.sshManager.GenerateKey(hostname)
		if err == nil {
			key = generatedKey
		}
	}

	dest := config.Destination{
		Alias:           alias,
		Hostname:        hostname,
		User:            user,
		Key:             key,
		Group:           group,
		ConnectionCount: 0,
	}

	m.cfg.Destinations = append(m.cfg.Destinations, dest)
	m.cfg.Save()
	m.sshManager.SyncConfig()
}

func (m Model) viewConfigure() string {
	s := titleStyle.Render("New Destination") + "\n\n"

	for i, input := range m.configInputs {
		s += input.View() + "\n"
		if i < len(m.configInputs)-1 {
			s += "\n"
		}
	}

	s += helpStyle.Render("\n[Enter] Next/Save  [Esc] Cancel")
	return s
}

// --- Delete Confirm Logic ---

func (m Model) updateDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		// Delete
		newDest := []config.Destination{}
		for _, d := range m.cfg.Destinations {
			if d.Alias != m.deleteTarget.Alias {
				newDest = append(newDest, d)
			}
		}
		m.cfg.Destinations = newDest
		m.cfg.Save()
		m.sshManager.SyncConfig()
		m.mode = ModeSelect
		m.updateFilteredList()
	case "n", "N", "esc":
		m.mode = ModeSelect
	}
	return m, nil
}

func (m Model) viewDeleteConfirm() string {
	s := titleStyle.Render("Delete Destination?") + "\n\n"
	s += fmt.Sprintf("Are you sure you want to delete %q?\n", m.deleteTarget.Alias)
	s += "This will also remove it from SSH config.\n\n"
	s += "[Y] Yes, Delete  [N] Cancel"
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}