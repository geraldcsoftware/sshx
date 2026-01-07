package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
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
	ModeConnectWarn
)

// HostStatusMsg is sent when a host check completes
type HostStatusMsg struct {
	Result ssh.CheckResult
}

// Styles
var (
	primaryColor   = lipgloss.Color("#FF00FF") // Magenta
	secondaryColor = lipgloss.Color("#9B59B6") // Purple
	subtleColor    = lipgloss.Color("#626262") // Grey
	greenColor     = lipgloss.Color("#00FF00") // Green for OK
	yellowColor    = lipgloss.Color("#FFFF00") // Yellow for Warning
	redColor       = lipgloss.Color("#FF0000") // Red for Error

	docStyle       = lipgloss.NewStyle().Margin(1, 2)

	// List Styles
	titleStyle = lipgloss.NewStyle(). 
		Foreground(primaryColor).
		Bold(true).
		Padding(0, 1)

	itemStyle = lipgloss.NewStyle().PaddingLeft(2)

	selectedItemStyle = lipgloss.NewStyle(). 
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(primaryColor).
		Foreground(primaryColor).
		PaddingLeft(1)

	paginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)

	helpStyle = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)

	// Form Styles
	focusedStyle = lipgloss.NewStyle().Foreground(primaryColor)
	blurredStyle = lipgloss.NewStyle().Foreground(subtleColor)
	cursorStyle  = focusedStyle.Copy()
	noStyle      = lipgloss.NewStyle()
)

// DestinationItem implements list.Item
type DestinationItem struct {
	Dest config.Destination
}

func (i DestinationItem) Title() string       { return i.Dest.Alias }
func (i DestinationItem) Description() string {
	t := timeAgo(i.Dest.LastConnectedAt)
	statusIcon := "○" // Offline/Unknown (Gray)
	statusStyle := lipgloss.NewStyle().Foreground(subtleColor)

	switch i.Dest.Status {
	case config.StatusOk:
		statusIcon = "●" // OK (Green)
		statusStyle = lipgloss.NewStyle().Foreground(greenColor)
	case config.StatusOffline:
		statusIcon = "○"
		statusStyle = lipgloss.NewStyle().Foreground(subtleColor)
	case config.StatusKeyError:
		statusIcon = "▲" // Warning (Yellow)
		statusStyle = lipgloss.NewStyle().Foreground(yellowColor)
	}

	return fmt.Sprintf("%s %s • %s", statusStyle.Render(statusIcon), i.Dest.Hostname, t)
}
func (i DestinationItem) FilterValue() string { return i.Dest.Alias + " " + i.Dest.Hostname }

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

// SimpleItem implements list.Item for basic strings
type SimpleItem string

func (s SimpleItem) Title() string       { return string(s) }
func (s SimpleItem) Description() string { return "" }
func (s SimpleItem) FilterValue() string { return string(s) }

// Model is the main TUI model
type Model struct {
	cfg        *config.Config
	sshManager *ssh.Manager
	mode       Mode

	// Select Mode
	list list.Model

	// Configure Mode
	focusIndex    int
	inputs        []textinput.Model
	keyPicker     list.Model
	groupPicker   list.Model
	showKeyPicker bool
	showGroupPicker bool
	
	// Delete Mode
	deleteTarget *config.Destination

	// Connect Warn Mode
	warnTarget *config.Destination

	// State
	Quitting     bool
	SelectedDest *config.Destination
	windowWidth  int
	windowHeight int
}

func NewModel(cfg *config.Config, sshMgr *ssh.Manager, initialFilter string, configureHost string) Model {
	// Initialize Inputs
	inputs := make([]textinput.Model, 3)
	labels := []string{"Alias", "Hostname", "User"}
	for i := range inputs {
		t := textinput.New()
		t.Cursor.Style = cursorStyle
		t.CharLimit = 64
		t.Prompt = labels[i] + ": "
		
		switch i {
		case 0:
			t.Placeholder = "my-server"
			t.Focus()
			t.PromptStyle = focusedStyle
			t.TextStyle = focusedStyle
		case 1:
			t.Placeholder = "server.example.com"
		case 2:
			t.Placeholder = "root (optional)"
		}
		inputs[i] = t
	}

	// Initialize Pickers
	// Key Picker
	keyList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	keyList.Title = "Select SSH Key"
	keyList.SetShowHelp(false)
	keyList.SetHeight(6)
	keyList.SetShowTitle(false)
	keyList.SetShowStatusBar(false)
	keyList.SetFilteringEnabled(true)
	keyList.DisableQuitKeybindings()

	// Group Picker
	groupList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	groupList.Title = "Select Group"
	groupList.SetShowHelp(false)
	groupList.SetHeight(6)
	groupList.SetShowTitle(false)
	groupList.SetShowStatusBar(false)
	groupList.SetFilteringEnabled(true)
	groupList.DisableQuitKeybindings()

	// Main List
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = selectedItemStyle
	delegate.Styles.SelectedDesc = selectedItemStyle.Copy().Foreground(secondaryColor)

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "SSH Destinations"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle
	l.FilterInput.SetValue(initialFilter)

	m := Model{
		cfg:         cfg,
		sshManager:  sshMgr,
		mode:        ModeSelect,
		list:        l,
		inputs:      inputs,
		keyPicker:   keyList,
		groupPicker: groupList,
	}

	m.refreshList()
	m.refreshPickers()

	// Handle initial state
	if configureHost != "" {
		m.mode = ModeConfigure
		m.inputs[0].SetValue(configureHost)
		m.inputs[1].SetValue(configureHost)
	} else if initialFilter != "" && len(m.list.Items()) == 0 {
		m.mode = ModeConfigure
		m.inputs[0].SetValue(initialFilter)
		m.inputs[1].SetValue(initialFilter)
	}

	return m
}

func (m *Model) refreshList() {
	m.cfg.SortDestinations()
	items := make([]list.Item, len(m.cfg.Destinations))
	for i, d := range m.cfg.Destinations {
		items[i] = DestinationItem{Dest: d}
	}
	m.list.SetItems(items)
}

func (m *Model) refreshPickers() {
	// Keys
	keys, _ := m.sshManager.ListKeys()
	keyItems := []list.Item{SimpleItem("[Generate New]")}
	for _, k := range keys {
		parts := strings.Split(k, "/")
		name := parts[len(parts)-1]
		keyItems = append(keyItems, SimpleItem(name))
	}
	m.keyPicker.SetItems(keyItems)

	// Groups
	uniqueGroups := make(map[string]bool)
	for _, d := range m.cfg.Destinations {
		if d.Group != "" {
		
uniqueGroups[d.Group] = true
		}
	}
	for g := range m.cfg.Groups {
		uniqueGroups[g] = true
	}

	groupItems := []list.Item{SimpleItem("[None]")}
	for g := range uniqueGroups {
		groupItems = append(groupItems, SimpleItem(g))
	}
	m.groupPicker.SetItems(groupItems)
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.checkAllHosts())
}

func (m *Model) checkAllHosts() tea.Cmd {
	var cmds []tea.Cmd
	for _, d := range m.cfg.Destinations {
		cmds = append(cmds, checkHostCmd(d.Alias, d.Hostname))
	}
	return tea.Batch(cmds...)
}

func checkHostCmd(alias, hostname string) tea.Cmd {
	return func() tea.Msg {
		// 3 second timeout for checks
		res := ssh.CheckHost(alias, hostname, 3*time.Second)
		return HostStatusMsg{Result: res}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
		m.keyPicker.SetWidth(msg.Width - h - 4)
		m.groupPicker.SetWidth(msg.Width - h - 4)

	case HostStatusMsg:
		// Update status in config (memory only)
		for i := range m.cfg.Destinations {
			if m.cfg.Destinations[i].Alias == msg.Result.Alias {
				m.cfg.Destinations[i].Status = msg.Result.Status
				// Trigger list refresh to show new status
				// Note: this might be inefficient if many hosts return at once,
				// but Bubble Tea handles batching somewhat.
				// In a larger app, we'd optimize.
				m.refreshList()
				break
			}
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.Quitting = true
			return m, tea.Quit
		}

		switch m.mode {
		case ModeSelect:
			if msg.String() == "ctrl+n" || msg.String() == "c" && m.list.FilterState() != list.Filtering {
				m.mode = ModeConfigure
				m.focusIndex = 0
				return m, nil
			}
			if msg.String() == "enter" {
				if i, ok := m.list.SelectedItem().(DestinationItem); ok {
					// Check status before connecting
					if i.Dest.Status == config.StatusKeyError {
						m.warnTarget = &i.Dest
						m.mode = ModeConnectWarn
						return m, nil
					}
				m.SelectedDest = &i.Dest
					return m, tea.Quit
				}
			}
			if msg.String() == "d" && m.list.FilterState() != list.Filtering {
				if i, ok := m.list.SelectedItem().(DestinationItem); ok {
					m.deleteTarget = &i.Dest
					m.mode = ModeDeleteConfirm
					return m, nil
				}
			}

		case ModeConfigure:
			return m.updateConfigure(msg)
			
		case ModeDeleteConfirm:
			if msg.String() == "y" || msg.String() == "Y" || msg.String() == "enter" {
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
				m.refreshList()
				return m, nil
			}
			if msg.String() == "n" || msg.String() == "N" || msg.String() == "esc" {
				m.mode = ModeSelect
				return m, nil
			}

		case ModeConnectWarn:
			if msg.String() == "y" || msg.String() == "Y" || msg.String() == "enter" {
				m.SelectedDest = m.warnTarget
				return m, tea.Quit
			}
			if msg.String() == "n" || msg.String() == "N" || msg.String() == "esc" {
				m.mode = ModeSelect
				return m, nil
			}
		}
	}

	if m.mode == ModeSelect {
		m.list, cmd = m.list.Update(msg)
	}

	return m, cmd
}

func (m Model) View() string {
	if m.Quitting {
		return ""
	}

	switch m.mode {
	case ModeSelect:
		return docStyle.Render(m.list.View())
	case ModeConfigure:
		return docStyle.Render(m.viewConfigure())
	case ModeDeleteConfirm:
		return docStyle.Render(m.viewDeleteConfirm())
	case ModeConnectWarn:
		return docStyle.Render(m.viewConnectWarn())
	}
	return ""
}

// --- Configure Mode Logic ---

func (m Model) updateConfigure(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.mode = ModeSelect
		return m, nil
	}

	changeFocus := func(newIndex int) {
		if m.focusIndex < 3 {
			m.inputs[m.focusIndex].Blur()
			m.inputs[m.focusIndex].PromptStyle = noStyle
			m.inputs[m.focusIndex].TextStyle = noStyle
		}
		m.focusIndex = newIndex
		if m.focusIndex < 3 {
			m.inputs[m.focusIndex].Focus()
			m.inputs[m.focusIndex].PromptStyle = focusedStyle
			m.inputs[m.focusIndex].TextStyle = focusedStyle
		}
	}

	if msg.String() == "tab" {
		changeFocus((m.focusIndex + 1) % 5)
		return m, nil
	}
	if msg.String() == "shift+tab" {
		changeFocus((m.focusIndex - 1 + 5) % 5)
		return m, nil
	}

	if m.focusIndex < 3 {
		switch msg.String() {
		case "up":
			if m.focusIndex > 0 {
				changeFocus(m.focusIndex - 1)
			}
			return m, nil
		case "down":
			changeFocus(m.focusIndex + 1)
			return m, nil
		case "enter":
			changeFocus(m.focusIndex + 1)
			return m, nil
		}
		var cmd tea.Cmd
		m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
		return m, cmd
	}

	if m.focusIndex == 3 {
		if msg.String() == "up" && m.keyPicker.Index() == 0 {
			changeFocus(2)
			return m, nil
		}
		if msg.String() == "enter" {
			changeFocus(4)
			return m, nil
		}
		var cmd tea.Cmd
		m.keyPicker, cmd = m.keyPicker.Update(msg)
		return m, cmd
	}

	if m.focusIndex == 4 {
		if msg.String() == "up" && m.groupPicker.Index() == 0 {
			changeFocus(3)
			return m, nil
		}
		if msg.String() == "enter" {
			m.saveDestination()
			m.mode = ModeSelect
			m.refreshList()
			// Check the new host
			newAlias := m.inputs[0].Value()
			newHost := m.inputs[1].Value()
			return m, checkHostCmd(newAlias, newHost)
		}
		var cmd tea.Cmd
		m.groupPicker, cmd = m.groupPicker.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) viewConfigure() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("New Destination") + "\n\n")

	for i := 0; i < 3; i++ {
		b.WriteString(m.inputs[i].View() + "\n")
	}

	b.WriteString("\n" + m.renderPickerLabel("SSH Key", m.focusIndex == 3))
	if m.focusIndex == 3 {
		b.WriteString("\n" + m.keyPicker.View())
	} else {
		val := m.keyPicker.SelectedItem()
		txt := "[Select...]"
		if val != nil {
			txt = val.FilterValue()
		}
		b.WriteString(" " + txt + "\n")
	}

	b.WriteString("\n" + m.renderPickerLabel("Group", m.focusIndex == 4))
	if m.focusIndex == 4 {
		b.WriteString("\n" + m.groupPicker.View())
	} else {
		val := m.groupPicker.SelectedItem()
		txt := "[None]"
		if val != nil {
			txt = val.FilterValue()
		}
		b.WriteString(" " + txt + "\n")
	}

	b.WriteString("\n\n" + helpStyle.Render("[Enter] Next/Save  [Up/Down] Navigate  [Esc] Cancel"))
	return b.String()
}

func (m Model) renderPickerLabel(text string, focused bool) string {
	style := noStyle
	if focused {
		style = focusedStyle
	}
	return style.Render(text + ":")
}

func (m *Model) saveDestination() {
	alias := m.inputs[0].Value()
	hostname := m.inputs[1].Value()
	user := m.inputs[2].Value()
	
	keyItem, _ := m.keyPicker.SelectedItem().(SimpleItem)
	groupItem, _ := m.groupPicker.SelectedItem().(SimpleItem)
	
	key := string(keyItem)
	group := string(groupItem)

	if alias == "" { return }
	if hostname == "" { hostname = alias }

	if key == "[Generate New]" {
		generatedKey, err := m.sshManager.GenerateKey(hostname)
		if err == nil {
			key = generatedKey
		} else {
			key = ""
		}
	} else {
		if !strings.HasPrefix(key, "/") && !strings.HasPrefix(key, "~") && key != "" {
			home, _ := os.UserHomeDir()
			key = home + "/.ssh/" + key
		}
	}

	if group == "[None]" {
		group = ""
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

func (m Model) viewDeleteConfirm() string {
	s := titleStyle.Render("Delete Destination?") + "\n\n"
	s += fmt.Sprintf("Are you sure you want to delete %q?\n", m.deleteTarget.Alias)
	s += "This will also remove it from SSH config.\n\n"
	s += "[Y] Yes, Delete  [N] Cancel"
	return s
}

func (m Model) viewConnectWarn() string {
	s := titleStyle.Render("Warning: Host Identity Changed!") + "\n\n"
	s += fmt.Sprintf("The fingerprint for %q does not match known_hosts.\n", m.warnTarget.Alias)
	s += "This could mean the server has been reinstalled or a MITM attack.\n\n"
	s += "[Y] Connect Anyway  [N] Cancel"
	return s
}