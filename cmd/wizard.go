// Copyright © 2025 Jake Rogers <code@supportoss.org>
package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/JakeTRogers/timeBuddy/logger"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// pane identifies which pane has focus in the wizard UI.
type pane int

const (
	// selectedPane is the left pane showing selected timezones.
	selectedPane pane = iota
	// availablePane is the right pane showing available timezones.
	availablePane
)

// nodeType identifies the type of a tree node.
type nodeType int

const (
	// areaNode represents a timezone area (e.g., "America").
	areaNode nodeType = iota
	// locationNode represents a specific timezone location.
	locationNode
)

// treeNode represents an item in the available timezones tree.
type treeNode struct {
	name       string
	fullPath   string // Full timezone path (e.g., "America/New_York")
	nodeType   nodeType
	expanded   bool
	children   []treeNode
	parent     *treeNode
	isSelected bool // Whether this timezone is in the selected list
}

// flatTreeEntry represents a visible item in the flattened tree view.
type flatTreeEntry struct {
	areaIdx  int
	childIdx int // -1 if this is an area node
}

// isArea returns true if this entry represents an area node.
func (f flatTreeEntry) isArea() bool {
	return f.childIdx == -1
}

// searchMatch represents a timezone that matches the search query.
type searchMatch struct {
	fullPath   string
	areaIdx    int
	childIdx   int
	isSelected bool
}

// wizardModel is the Bubbletea model for the timezone wizard.
type wizardModel struct {
	// Data
	selected  []string             // Currently selected timezones (ordered)
	tree      []treeNode           // Available timezones as a tree
	flatTree  []flatTreeEntry      // Visible items in the flattened tree view
	treeIndex map[string]*treeNode // Quick lookup by fullPath

	// UI State
	focusedPane    pane
	selectedCursor int // Cursor position in selected list
	treeCursor     int // Cursor position in flattened tree view

	// Search
	searchMode        bool
	searchQuery       string
	searchResults     []searchMatch // Filtered matches during search
	searchCursor      int           // Cursor position within search results
	preSearchExpanded map[int]bool  // Saved expansion state before search

	// Dimensions
	width  int
	height int

	// Exit state
	quitting bool
	saved    bool
}

// Key bindings
type keyMap struct {
	Up        key.Binding
	Down      key.Binding
	ShiftUp   key.Binding
	ShiftDown key.Binding
	Tab       key.Binding
	Space     key.Binding
	Enter     key.Binding
	Delete    key.Binding
	Search    key.Binding
	Escape    key.Binding
	Quit      key.Binding
	Save      key.Binding
}

var keys = keyMap{
	Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	ShiftUp:   key.NewBinding(key.WithKeys("shift+up", "K"), key.WithHelp("⇧↑/K", "move up")),
	ShiftDown: key.NewBinding(key.WithKeys("shift+down", "J"), key.WithHelp("⇧↓/J", "move down")),
	Tab:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane")),
	Space:     key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
	Enter:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "expand/collapse")),
	Delete:    key.NewBinding(key.WithKeys("backspace", "delete", "x"), key.WithHelp("del/x", "remove")),
	Search:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Escape:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel search")),
	Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "save & quit")),
	Save:      key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save")),
}

// Styles
var (
	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("63")). // Purple/blue
				Padding(0, 1)

	unfocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240")). // Gray
				Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("63")).
			MarginBottom(1)

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")). // Bright pink
			Bold(true)

	checkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")). // Green
			Bold(true)

	partialCheckStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")). // Orange
				Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	searchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Background(lipgloss.Color("236"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	matchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")). // Yellow
			Bold(true)
)

// buildTree creates the tree structure from the flat timezone list
func buildTree(timezones []string, selected []string) ([]treeNode, map[string]*treeNode) {
	selectedSet := make(map[string]bool)
	for _, tz := range selected {
		selectedSet[tz] = true
	}

	areaMap := make(map[string]*treeNode)
	treeIndex := make(map[string]*treeNode)

	for _, tz := range timezones {
		if !strings.Contains(tz, "/") {
			// Skip timezones without area (like "UTC", "EST", etc.)
			continue
		}

		parts := strings.SplitN(tz, "/", 2)
		areaName := parts[0]
		location := parts[1]

		// Get or create area node
		area, exists := areaMap[areaName]
		if !exists {
			area = &treeNode{
				name:     areaName,
				fullPath: areaName,
				nodeType: areaNode,
				expanded: false,
				children: []treeNode{},
			}
			areaMap[areaName] = area
		}

		// Create location node
		locNode := treeNode{
			name:       location,
			fullPath:   tz,
			nodeType:   locationNode,
			parent:     area,
			isSelected: selectedSet[tz],
		}
		area.children = append(area.children, locNode)
	}

	// Convert map to sorted slice
	var tree []treeNode
	var areaNames []string
	for name := range areaMap {
		areaNames = append(areaNames, name)
	}
	sort.Strings(areaNames)

	// Create System area with Local timezone as the first entry
	systemArea := treeNode{
		name:     "System",
		fullPath: "System",
		nodeType: areaNode,
		expanded: true, // Auto-expand System area
		children: []treeNode{
			{
				name:       "Local",
				fullPath:   "Local",
				nodeType:   locationNode,
				isSelected: selectedSet["Local"],
			},
		},
	}

	// Add System area at the top of the tree
	tree = append(tree, systemArea)

	// Add all other areas
	for _, name := range areaNames {
		area := areaMap[name]
		// Sort children
		sort.Slice(area.children, func(i, j int) bool {
			return area.children[i].name < area.children[j].name
		})
		tree = append(tree, *area)
	}

	// Build index and update parent pointers
	for i := range tree {
		treeIndex[tree[i].fullPath] = &tree[i]
		for j := range tree[i].children {
			tree[i].children[j].parent = &tree[i]
			treeIndex[tree[i].children[j].fullPath] = &tree[i].children[j]
		}
	}

	return tree, treeIndex
}

// flattenTree creates a flat list of visible tree items for navigation
func flattenTree(tree []treeNode) []flatTreeEntry {
	var flat []flatTreeEntry
	for i := range tree {
		flat = append(flat, flatTreeEntry{areaIdx: i, childIdx: -1})
		if tree[i].expanded {
			for j := range tree[i].children {
				flat = append(flat, flatTreeEntry{areaIdx: i, childIdx: j})
			}
		}
	}
	return flat
}

// getNodeFromFlatIndex returns the tree node at a given flat index
func (m *wizardModel) getNodeFromFlatIndex(flatIdx int) *treeNode {
	if flatIdx < 0 || flatIdx >= len(m.flatTree) {
		return nil
	}
	entry := m.flatTree[flatIdx]
	if entry.isArea() {
		return &m.tree[entry.areaIdx]
	}
	return &m.tree[entry.areaIdx].children[entry.childIdx]
}

// countSelectedInArea returns how many locations are selected in an area
func countSelectedInArea(area *treeNode, selected []string) int {
	count := 0
	for _, child := range area.children {
		for _, tz := range selected {
			if child.fullPath == tz {
				count++
				break
			}
		}
	}
	return count
}

// initWizardModel creates a new wizard model
func initWizardModel(currentTimezones []string) wizardModel {
	tree, treeIndex := buildTree(timezonesAll, currentTimezones)

	// Auto-expand areas that have selected timezones
	for i := range tree {
		if countSelectedInArea(&tree[i], currentTimezones) > 0 {
			tree[i].expanded = true
		}
	}

	m := wizardModel{
		selected:       append([]string{}, currentTimezones...), // Copy
		tree:           tree,
		treeIndex:      treeIndex,
		focusedPane:    availablePane,
		selectedCursor: 0,
		treeCursor:     0,
		width:          80,
		height:         24,
	}
	m.flatTree = flattenTree(m.tree)
	m.updateSelectionState()

	return m
}

// updateSelectionState syncs the tree's isSelected state with the selected list
func (m *wizardModel) updateSelectionState() {
	selectedSet := make(map[string]bool)
	for _, tz := range m.selected {
		selectedSet[tz] = true
	}

	for i := range m.tree {
		for j := range m.tree[i].children {
			m.tree[i].children[j].isSelected = selectedSet[m.tree[i].children[j].fullPath]
		}
	}
}

// Init implements tea.Model
func (m wizardModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Handle search mode separately
		if m.searchMode {
			return m.handleSearchInput(msg)
		}

		switch {
		case key.Matches(msg, keys.Quit):
			m.quitting = true
			m.saved = true
			return m, tea.Quit

		case key.Matches(msg, keys.Tab):
			if m.focusedPane == selectedPane {
				m.focusedPane = availablePane
			} else {
				m.focusedPane = selectedPane
			}
			return m, nil

		case key.Matches(msg, keys.Search):
			m.enterSearchMode()
			return m, nil

		case key.Matches(msg, keys.Up):
			m.moveCursorUp()
			return m, nil

		case key.Matches(msg, keys.Down):
			m.moveCursorDown()
			return m, nil

		case key.Matches(msg, keys.ShiftUp):
			if m.focusedPane == selectedPane {
				m.moveSelectedUp()
			}
			return m, nil

		case key.Matches(msg, keys.ShiftDown):
			if m.focusedPane == selectedPane {
				m.moveSelectedDown()
			}
			return m, nil

		case key.Matches(msg, keys.Space):
			m.toggleSelection()
			return m, nil

		case key.Matches(msg, keys.Enter):
			if m.focusedPane == availablePane {
				m.toggleExpand()
			}
			return m, nil

		case key.Matches(msg, keys.Delete):
			if m.focusedPane == selectedPane {
				m.removeSelected()
			}
			return m, nil
		}
	}

	return m, nil
}

// handleSearchInput handles keyboard input in search mode
func (m wizardModel) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		// Restore pre-search state
		m.exitSearchMode(false)
		return m, nil

	case msg.String() == "up":
		// Navigate up in search results (only arrow key, not k)
		if len(m.searchResults) > 0 && m.searchCursor > 0 {
			m.searchCursor--
		}
		return m, nil

	case msg.String() == "down":
		// Navigate down in search results (only arrow key, not j)
		if len(m.searchResults) > 0 && m.searchCursor < len(m.searchResults)-1 {
			m.searchCursor++
		}
		return m, nil

	case key.Matches(msg, keys.Space):
		// Toggle selection of current search result
		if len(m.searchResults) > 0 && m.searchCursor < len(m.searchResults) {
			match := m.searchResults[m.searchCursor]
			tz := match.fullPath
			if m.isInSelected(tz) {
				m.removeFromSelected(tz)
			} else {
				m.selected = append(m.selected, tz)
			}
			m.updateSelectionState()
			m.performSearch() // Refresh to update selection indicators
		}
		return m, nil

	case msg.Type == tea.KeyEnter:
		// Select current result and exit search
		if len(m.searchResults) > 0 && m.searchCursor < len(m.searchResults) {
			match := m.searchResults[m.searchCursor]
			// Expand the area containing this match and position cursor there
			m.tree[match.areaIdx].expanded = true
			m.exitSearchMode(true)
			// Find the position in the flat tree
			m.flatTree = flattenTree(m.tree)
			for i, entry := range m.flatTree {
				if entry.areaIdx == match.areaIdx && entry.childIdx == match.childIdx {
					m.treeCursor = i
					break
				}
			}
			m.focusedPane = availablePane
		} else {
			m.exitSearchMode(false)
		}
		return m, nil

	case msg.Type == tea.KeyBackspace:
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.performSearch()
		}
		return m, nil

	case msg.Type == tea.KeyRunes:
		m.searchQuery += string(msg.Runes)
		m.performSearch()
		return m, nil
	}

	return m, nil
}

// enterSearchMode initializes search mode and saves current state
func (m *wizardModel) enterSearchMode() {
	m.searchMode = true
	m.searchQuery = ""
	m.searchResults = nil
	m.searchCursor = 0
	// Save current expansion state
	m.preSearchExpanded = make(map[int]bool)
	for i, area := range m.tree {
		m.preSearchExpanded[i] = area.expanded
	}
}

// exitSearchMode cleans up search state
func (m *wizardModel) exitSearchMode(keepExpansion bool) {
	m.searchMode = false
	m.searchQuery = ""
	m.searchResults = nil
	m.searchCursor = 0

	if !keepExpansion && m.preSearchExpanded != nil {
		// Restore expansion state
		for i := range m.tree {
			if expanded, ok := m.preSearchExpanded[i]; ok {
				m.tree[i].expanded = expanded
			}
		}
		m.flatTree = flattenTree(m.tree)
	}
	m.preSearchExpanded = nil
}

// performSearch searches for timezones matching the query
func (m *wizardModel) performSearch() {
	if m.searchQuery == "" {
		m.searchResults = nil
		m.searchCursor = 0
		return
	}

	query := strings.ToLower(m.searchQuery)
	m.searchResults = nil

	// Find all matching location nodes (not areas)
	for i := range m.tree {
		for j := range m.tree[i].children {
			child := &m.tree[i].children[j]
			if strings.Contains(strings.ToLower(child.fullPath), query) {
				m.searchResults = append(m.searchResults, searchMatch{
					fullPath:   child.fullPath,
					areaIdx:    i,
					childIdx:   j,
					isSelected: m.isInSelected(child.fullPath),
				})
			}
		}
	}

	// Reset cursor if it's out of bounds
	if m.searchCursor >= len(m.searchResults) {
		m.searchCursor = 0
	}
}

// moveCursorUp moves the cursor up in the focused pane
func (m *wizardModel) moveCursorUp() {
	if m.focusedPane == selectedPane {
		if m.selectedCursor > 0 {
			m.selectedCursor--
		}
	} else {
		if m.treeCursor > 0 {
			m.treeCursor--
		}
	}
}

// moveCursorDown moves the cursor down in the focused pane
func (m *wizardModel) moveCursorDown() {
	if m.focusedPane == selectedPane {
		if m.selectedCursor < len(m.selected)-1 {
			m.selectedCursor++
		}
	} else {
		if m.treeCursor < len(m.flatTree)-1 {
			m.treeCursor++
		}
	}
}

// moveSelectedUp moves the selected timezone up in the list
func (m *wizardModel) moveSelectedUp() {
	if m.selectedCursor > 0 && len(m.selected) > 1 {
		m.selected[m.selectedCursor], m.selected[m.selectedCursor-1] =
			m.selected[m.selectedCursor-1], m.selected[m.selectedCursor]
		m.selectedCursor--
	}
}

// moveSelectedDown moves the selected timezone down in the list
func (m *wizardModel) moveSelectedDown() {
	if m.selectedCursor < len(m.selected)-1 && len(m.selected) > 1 {
		m.selected[m.selectedCursor], m.selected[m.selectedCursor+1] =
			m.selected[m.selectedCursor+1], m.selected[m.selectedCursor]
		m.selectedCursor++
	}
}

// toggleSelection toggles a timezone's selection state
func (m *wizardModel) toggleSelection() {
	if m.focusedPane == selectedPane {
		// In selected pane, space removes the item
		m.removeSelected()
		return
	}

	// In available pane
	node := m.getNodeFromFlatIndex(m.treeCursor)
	if node == nil {
		return
	}

	if node.nodeType == areaNode {
		// Toggle all locations in this area
		allSelected := true
		for _, child := range node.children {
			if !m.isInSelected(child.fullPath) {
				allSelected = false
				break
			}
		}

		if allSelected {
			// Remove all
			for _, child := range node.children {
				m.removeFromSelected(child.fullPath)
			}
		} else {
			// Add all not yet selected
			for _, child := range node.children {
				if !m.isInSelected(child.fullPath) {
					m.selected = append(m.selected, child.fullPath)
				}
			}
		}
	} else {
		// Toggle single location
		if m.isInSelected(node.fullPath) {
			m.removeFromSelected(node.fullPath)
		} else {
			m.selected = append(m.selected, node.fullPath)
		}
	}

	m.updateSelectionState()
}

// toggleExpand expands or collapses an area node
func (m *wizardModel) toggleExpand() {
	node := m.getNodeFromFlatIndex(m.treeCursor)
	if node == nil || node.nodeType != areaNode {
		return
	}

	// Find the area in the tree and toggle
	for i := range m.tree {
		if m.tree[i].fullPath == node.fullPath {
			m.tree[i].expanded = !m.tree[i].expanded
			break
		}
	}

	m.flatTree = flattenTree(m.tree)

	// Adjust cursor if needed
	if m.treeCursor >= len(m.flatTree) {
		m.treeCursor = len(m.flatTree) - 1
	}
}

// removeSelected removes the currently selected timezone
func (m *wizardModel) removeSelected() {
	if len(m.selected) == 0 || m.selectedCursor >= len(m.selected) {
		return
	}

	m.selected = append(m.selected[:m.selectedCursor], m.selected[m.selectedCursor+1:]...)

	if m.selectedCursor >= len(m.selected) && m.selectedCursor > 0 {
		m.selectedCursor--
	}

	m.updateSelectionState()
}

// removeFromSelected removes a specific timezone from the selected list
func (m *wizardModel) removeFromSelected(tz string) {
	for i, s := range m.selected {
		if s == tz {
			m.selected = append(m.selected[:i], m.selected[i+1:]...)
			break
		}
	}
}

// isInSelected checks if a timezone is in the selected list
func (m *wizardModel) isInSelected(tz string) bool {
	for _, s := range m.selected {
		if s == tz {
			return true
		}
	}
	return false
}

// View implements tea.Model
func (m wizardModel) View() string {
	if m.quitting {
		if m.saved {
			return "Timezones saved!\n"
		}
		return "Cancelled.\n"
	}

	// Calculate pane widths
	totalWidth := m.width - 4 // Account for borders
	leftWidth := totalWidth / 3
	rightWidth := totalWidth - leftWidth - 3 // -3 for gap

	// Ensure minimum widths
	if leftWidth < 25 {
		leftWidth = 25
	}
	if rightWidth < 40 {
		rightWidth = 40
	}

	// Calculate content height (leave room for title, help, and search)
	contentHeight := m.height - 8
	if contentHeight < 10 {
		contentHeight = 10
	}

	// Render left pane (selected timezones)
	leftContent := m.renderSelectedPane(leftWidth-4, contentHeight)
	leftStyle := unfocusedBorderStyle
	if m.focusedPane == selectedPane {
		leftStyle = focusedBorderStyle
	}
	leftPane := leftStyle.Width(leftWidth).Height(contentHeight + 2).Render(leftContent)

	// Render right pane (available timezones)
	rightContent := m.renderAvailablePane(rightWidth-4, contentHeight)
	rightStyle := unfocusedBorderStyle
	if m.focusedPane == availablePane {
		rightStyle = focusedBorderStyle
	}
	rightPane := rightStyle.Width(rightWidth).Height(contentHeight + 2).Render(rightContent)

	// Combine panes
	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, "  ", rightPane)

	// Title
	title := titleStyle.Render("⏰ Timezone Wizard")

	// Search bar
	searchBar := ""
	if m.searchMode {
		searchBar = searchStyle.Render(fmt.Sprintf(" 🔍 Search: %s█ ", m.searchQuery))
		if len(m.searchResults) > 0 {
			searchBar += dimStyle.Render(fmt.Sprintf(" (%d matches)", len(m.searchResults)))
		} else if m.searchQuery != "" {
			searchBar += dimStyle.Render(" (no matches)")
		}
		searchBar += "\n"
	}

	// Help text
	help := m.renderHelp()

	return fmt.Sprintf("%s\n%s%s\n%s", title, searchBar, panes, help)
}

// renderSelectedPane renders the left pane showing selected timezones
func (m wizardModel) renderSelectedPane(width, height int) string {
	var b strings.Builder

	header := titleStyle.Render("Selected Timezones")
	b.WriteString(header)
	b.WriteString("\n")

	if len(m.selected) == 0 {
		b.WriteString(dimStyle.Render("  (none selected)"))
		return b.String()
	}

	// Calculate visible range for scrolling
	startIdx := 0
	visibleCount := height - 2 // Account for header
	if visibleCount < 1 {
		visibleCount = 1
	}

	if m.selectedCursor >= visibleCount {
		startIdx = m.selectedCursor - visibleCount + 1
	}

	endIdx := startIdx + visibleCount
	if endIdx > len(m.selected) {
		endIdx = len(m.selected)
	}

	for i := startIdx; i < endIdx; i++ {
		tz := m.selected[i]

		// Truncate if needed
		displayTz := tz
		maxLen := width - 6
		if len(displayTz) > maxLen {
			displayTz = "…" + displayTz[len(displayTz)-maxLen+1:]
		}

		line := fmt.Sprintf("  %d. %s", i+1, displayTz)

		if i == m.selectedCursor && m.focusedPane == selectedPane {
			b.WriteString(cursorStyle.Render("► " + line[2:]))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(m.selected) > visibleCount {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  [%d/%d]", m.selectedCursor+1, len(m.selected))))
	}

	return b.String()
}

// renderAvailablePane renders the right pane showing the timezone tree
func (m wizardModel) renderAvailablePane(width, height int) string {
	var b strings.Builder

	// Different header and rendering for search mode
	if m.searchMode && m.searchQuery != "" {
		return m.renderSearchResults(width, height)
	}

	header := titleStyle.Render("Available Timezones")
	b.WriteString(header)
	b.WriteString("\n")

	// Calculate visible range for scrolling
	startIdx := 0
	visibleCount := height - 2
	if visibleCount < 1 {
		visibleCount = 1
	}

	if m.treeCursor >= visibleCount {
		startIdx = m.treeCursor - visibleCount + 1
	}

	endIdx := startIdx + visibleCount
	if endIdx > len(m.flatTree) {
		endIdx = len(m.flatTree)
	}

	for i := startIdx; i < endIdx; i++ {
		node := m.getNodeFromFlatIndex(i)
		if node == nil {
			continue
		}

		line := m.renderTreeNode(node, width-4)

		if i == m.treeCursor && m.focusedPane == availablePane {
			b.WriteString(cursorStyle.Render("► ") + line)
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(m.flatTree) > visibleCount {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  [%d/%d]", m.treeCursor+1, len(m.flatTree))))
	}

	return b.String()
}

// renderSearchResults renders the filtered search results
func (m wizardModel) renderSearchResults(width, height int) string {
	var b strings.Builder

	header := titleStyle.Render(fmt.Sprintf("Search Results (%d)", len(m.searchResults)))
	b.WriteString(header)
	b.WriteString("\n")

	if len(m.searchResults) == 0 {
		b.WriteString(dimStyle.Render("  No matches found"))
		return b.String()
	}

	// Calculate visible range for scrolling
	startIdx := 0
	visibleCount := height - 2
	if visibleCount < 1 {
		visibleCount = 1
	}

	if m.searchCursor >= visibleCount {
		startIdx = m.searchCursor - visibleCount + 1
	}

	endIdx := startIdx + visibleCount
	if endIdx > len(m.searchResults) {
		endIdx = len(m.searchResults)
	}

	for i := startIdx; i < endIdx; i++ {
		match := m.searchResults[i]

		// Show checkbox and full path
		checkBox := "[ ]"
		if m.isInSelected(match.fullPath) {
			checkBox = checkStyle.Render("[✓]")
		}

		// Truncate if needed, but show full path for context
		displayPath := match.fullPath
		maxLen := width - 8
		if len(displayPath) > maxLen {
			displayPath = "…" + displayPath[len(displayPath)-maxLen+1:]
		}

		// Highlight the matching part
		displayPath = m.highlightMatch(displayPath)

		line := fmt.Sprintf("%s %s", checkBox, displayPath)

		if i == m.searchCursor {
			b.WriteString(cursorStyle.Render("► ") + line)
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(m.searchResults) > visibleCount {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  [%d/%d]", m.searchCursor+1, len(m.searchResults))))
	}

	return b.String()
}

// highlightMatch highlights the search query within a string
func (m wizardModel) highlightMatch(s string) string {
	if m.searchQuery == "" {
		return s
	}

	lower := strings.ToLower(s)
	queryLower := strings.ToLower(m.searchQuery)
	idx := strings.Index(lower, queryLower)
	if idx == -1 {
		return s
	}

	// Rebuild string with highlighted portion
	before := s[:idx]
	match := s[idx : idx+len(m.searchQuery)]
	after := s[idx+len(m.searchQuery):]

	return before + matchStyle.Render(match) + after
}

// renderTreeNode renders a single tree node
func (m wizardModel) renderTreeNode(node *treeNode, maxWidth int) string {
	var b strings.Builder

	if node.nodeType == areaNode {
		// Area node
		expandIcon := "▸"
		if node.expanded {
			expandIcon = "▾"
		}

		// Count selected in this area
		selectedCount := countSelectedInArea(node, m.selected)
		totalCount := len(node.children)

		indicator := ""
		if selectedCount == totalCount && totalCount > 0 {
			indicator = checkStyle.Render(" [✓ all]")
		} else if selectedCount > 0 {
			indicator = partialCheckStyle.Render(fmt.Sprintf(" [%d/%d]", selectedCount, totalCount))
		}

		b.WriteString(fmt.Sprintf("%s %s%s", expandIcon, node.name, indicator))
	} else {
		// Location node
		checkBox := "[ ]"
		if node.isSelected {
			checkBox = checkStyle.Render("[✓]")
		}

		// Truncate if needed
		displayName := node.name
		maxLen := maxWidth - 8
		if len(displayName) > maxLen {
			displayName = displayName[:maxLen-1] + "…"
		}

		b.WriteString(fmt.Sprintf("  %s %s", checkBox, displayName))
	}

	return b.String()
}

// renderHelp renders the help bar at the bottom
func (m wizardModel) renderHelp() string {
	if m.searchMode {
		return helpStyle.Render("↑↓: navigate • Space: toggle • Enter: select & exit • Esc: cancel")
	}

	var parts []string

	if m.focusedPane == selectedPane {
		parts = []string{
			"↑↓: navigate",
			"⇧↑↓/JK: reorder",
			"Space/Del: remove",
			"Tab: switch pane",
			"/: search",
			"q: save & quit",
		}
	} else {
		parts = []string{
			"↑↓: navigate",
			"Enter: expand/collapse",
			"Space: toggle",
			"Tab: switch pane",
			"/: search",
			"q: save & quit",
		}
	}

	return helpStyle.Render(strings.Join(parts, " • "))
}

// runWizard starts the interactive timezone wizard.
// It returns the selected timezones or nil if cancelled.
func runWizard(v *viper.Viper, log *zerolog.Logger) ([]string, error) {
	// Disable logging before starting TUI to prevent interference with display
	log.Warn().Msg("disabling logging for interactive wizard")
	logger.Disable()

	currentTimezones := v.GetStringSlice("timezone")
	if len(currentTimezones) == 0 {
		currentTimezones = []string{"Local"}
	}

	model := initWizardModel(currentTimezones)

	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running wizard: %w", err)
	}

	m, ok := finalModel.(wizardModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type: %T", finalModel)
	}

	if m.saved {
		return m.selected, nil
	}

	return nil, nil
}

// NewWizardCmd creates and returns a new wizard command.
// Each call returns a fresh instance for test isolation.
func NewWizardCmd(v *viper.Viper) *cobra.Command {
	log := logger.GetLogger()

	wizardCmd := &cobra.Command{
		Use:   "wizard",
		Short: "Interactive timezone selector",
		Long: `Launch an interactive wizard to select and reorder timezones.

The wizard displays two panes:
  - Left pane: Your currently selected timezones (ordered)
  - Right pane: All available timezones organized by area

Navigation:
  - Tab: Switch between panes
  - ↑/↓ or j/k: Navigate up/down
  - Enter: Expand/collapse area in the available pane
  - Space: Toggle timezone selection
  - Shift+↑/↓ or J/K: Reorder selected timezones
  - Del/Backspace/x: Remove selected timezone
  - /: Start search mode
  - q: Save and quit

Example:
  $ timeBuddy wizard`,
	}

	// runWizardCmd executes the wizard command.
	runWizardCmd := func(cmd *cobra.Command, args []string) error {
		selected, err := runWizard(v, log)
		if err != nil {
			return fmt.Errorf("wizard failed: %w", err)
		}

		if selected == nil {
			return nil
		}

		v.Set("timezone", selected)
		if err := v.WriteConfig(); err != nil {
			log.Error().Err(err).Msg("failed to save config")
			return nil
		}

		fmt.Printf("Saved %d timezone(s) to config.\n", len(selected))
		return nil
	}

	wizardCmd.RunE = runWizardCmd

	return wizardCmd
}
