package cmd

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func Test_buildTree(t *testing.T) {
	selected := []string{"America/New_York", "Europe/London"}
	tree, treeIndex := buildTree(timezonesAll, selected)

	// Test that tree was built
	if len(tree) == 0 {
		t.Error("Tree should not be empty")
	}

	// Test that known areas exist
	knownAreas := []string{"America", "Europe", "Asia", "Africa", "Pacific", "Australia"}
	for _, area := range knownAreas {
		found := false
		for _, node := range tree {
			if node.name == area {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected area %s not found in tree", area)
		}
	}

	// Test that treeIndex contains selected timezones
	if _, ok := treeIndex["America/New_York"]; !ok {
		t.Error("treeIndex should contain America/New_York")
	}
	if _, ok := treeIndex["Europe/London"]; !ok {
		t.Error("treeIndex should contain Europe/London")
	}
}

func Test_flattenTree(t *testing.T) {
	selected := []string{}
	tree, _ := buildTree(timezonesAll, selected)

	// Test with System area auto-expanded (first area is always System with Local)
	flat := flattenTree(tree)
	// System area is auto-expanded, so we expect tree length + 1 (for the Local child)
	expectedLen := len(tree) + 1
	if len(flat) != expectedLen {
		t.Errorf("Expected flat length %d with System area expanded, got %d", expectedLen, len(flat))
	}

	// Expand second area (first non-System area)
	if len(tree) > 1 {
		tree[1].expanded = true
		flat = flattenTree(tree)
		expectedLen := len(tree) + len(tree[0].children) + len(tree[1].children)
		if len(flat) != expectedLen {
			t.Errorf("Expected flat length %d after expanding second area, got %d", expectedLen, len(flat))
		}
	}
}

func Test_countSelectedInArea(t *testing.T) {
	selected := []string{"America/New_York", "America/Chicago", "Europe/London"}
	tree, _ := buildTree(timezonesAll, selected)

	// Find America area
	var americaArea *treeNode
	for i := range tree {
		if tree[i].name == "America" {
			americaArea = &tree[i]
			break
		}
	}

	if americaArea == nil {
		t.Fatal("America area not found")
	}

	count := countSelectedInArea(americaArea, selected)
	if count != 2 {
		t.Errorf("Expected 2 selected in America, got %d", count)
	}

	// Find Europe area
	var europeArea *treeNode
	for i := range tree {
		if tree[i].name == "Europe" {
			europeArea = &tree[i]
			break
		}
	}

	if europeArea == nil {
		t.Fatal("Europe area not found")
	}

	count = countSelectedInArea(europeArea, selected)
	if count != 1 {
		t.Errorf("Expected 1 selected in Europe, got %d", count)
	}
}

func Test_initWizardModel(t *testing.T) {
	timezones := []string{"America/New_York", "Europe/London"}
	model := initWizardModel(timezones)

	// Test that selected list is populated
	if len(model.selected) != 2 {
		t.Errorf("Expected 2 selected timezones, got %d", len(model.selected))
	}

	// Test that tree is built
	if len(model.tree) == 0 {
		t.Error("Tree should not be empty")
	}

	// Test that flatTree is populated
	if len(model.flatTree) == 0 {
		t.Error("flatTree should not be empty")
	}

	// Test that areas with selected timezones are auto-expanded
	var americaExpanded, europeExpanded bool
	for _, node := range model.tree {
		if node.name == "America" {
			americaExpanded = node.expanded
		}
		if node.name == "Europe" {
			europeExpanded = node.expanded
		}
	}

	if !americaExpanded {
		t.Error("America should be auto-expanded since it has selected timezones")
	}
	if !europeExpanded {
		t.Error("Europe should be auto-expanded since it has selected timezones")
	}
}

func Test_wizardModel_isInSelected(t *testing.T) {
	timezones := []string{"America/New_York", "Europe/London"}
	model := initWizardModel(timezones)

	if !model.isInSelected("America/New_York") {
		t.Error("America/New_York should be in selected")
	}

	if !model.isInSelected("Europe/London") {
		t.Error("Europe/London should be in selected")
	}

	if model.isInSelected("Asia/Tokyo") {
		t.Error("Asia/Tokyo should not be in selected")
	}
}

func Test_wizardModel_updateSelectionState(t *testing.T) {
	timezones := []string{"America/New_York"}
	model := initWizardModel(timezones)

	// Verify New_York is marked as selected in tree
	found := false
	for i := range model.tree {
		for j := range model.tree[i].children {
			if model.tree[i].children[j].fullPath == "America/New_York" {
				if model.tree[i].children[j].isSelected {
					found = true
				}
				break
			}
		}
	}

	if !found {
		t.Error("America/New_York should be marked as selected in tree")
	}

	// Add a new timezone and update
	model.selected = append(model.selected, "Europe/London")
	model.updateSelectionState()

	// Verify London is now marked as selected
	for i := range model.tree {
		for j := range model.tree[i].children {
			if model.tree[i].children[j].fullPath == "Europe/London" {
				if !model.tree[i].children[j].isSelected {
					t.Error("Europe/London should be marked as selected after update")
				}
				break
			}
		}
	}
}

func Test_wizardModel_removeFromSelected(t *testing.T) {
	timezones := []string{"America/New_York", "Europe/London", "Asia/Tokyo"}
	model := initWizardModel(timezones)

	model.removeFromSelected("Europe/London")

	if len(model.selected) != 2 {
		t.Errorf("Expected 2 timezones after removal, got %d", len(model.selected))
	}

	if model.isInSelected("Europe/London") {
		t.Error("Europe/London should have been removed")
	}

	// Verify others are still present
	if !model.isInSelected("America/New_York") {
		t.Error("America/New_York should still be selected")
	}
	if !model.isInSelected("Asia/Tokyo") {
		t.Error("Asia/Tokyo should still be selected")
	}
}

func Test_wizardCmd_exists(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "wizard" {
			found = true
			break
		}
	}
	if !found {
		t.Error("wizard command should be registered")
	}
}

// Test_flatTreeEntry_isArea tests the isArea method on flatTreeEntry
func Test_flatTreeEntry_isArea(t *testing.T) {
	tests := []struct {
		name     string
		entry    flatTreeEntry
		expected bool
	}{
		{
			name:     "area entry (childIdx -1)",
			entry:    flatTreeEntry{areaIdx: 0, childIdx: -1},
			expected: true,
		},
		{
			name:     "location entry (childIdx 0)",
			entry:    flatTreeEntry{areaIdx: 0, childIdx: 0},
			expected: false,
		},
		{
			name:     "location entry (childIdx > 0)",
			entry:    flatTreeEntry{areaIdx: 1, childIdx: 5},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.entry.isArea()
			if result != tt.expected {
				t.Errorf("isArea() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// Test_wizardModel_getNodeFromFlatIndex tests the getNodeFromFlatIndex method
func Test_wizardModel_getNodeFromFlatIndex(t *testing.T) {
	selected := []string{"America/New_York"}
	model := initWizardModel(selected)

	// Expand first area to get more flat entries
	model.tree[0].expanded = true
	model.flatTree = flattenTree(model.tree)

	tests := []struct {
		name       string
		index      int
		expectNil  bool
		expectArea bool
	}{
		{
			name:       "first entry is System area",
			index:      0,
			expectNil:  false,
			expectArea: true,
		},
		{
			name:       "second entry (Local) is not area",
			index:      1,
			expectNil:  false,
			expectArea: false,
		},
		{
			name:      "negative index returns nil",
			index:     -1,
			expectNil: true,
		},
		{
			name:      "index out of bounds returns nil",
			index:     len(model.flatTree) + 10,
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := model.getNodeFromFlatIndex(tt.index)

			if tt.expectNil {
				if node != nil {
					t.Error("Expected nil node")
				}
				return
			}

			if node == nil {
				t.Fatal("Expected non-nil node")
			}

			if tt.expectArea && node.nodeType != areaNode {
				t.Errorf("Expected area node, got %v", node.nodeType)
			}
		})
	}
}

// Test_wizardModel_moveCursorUp tests cursor movement
func Test_wizardModel_moveCursorUp(t *testing.T) {
	timezones := []string{}
	model := initWizardModel(timezones)

	// Start at position 2
	model.treeCursor = 2
	model.moveCursorUp()

	if model.treeCursor != 1 {
		t.Errorf("Expected cursor at 1, got %d", model.treeCursor)
	}

	// Move up again
	model.moveCursorUp()
	if model.treeCursor != 0 {
		t.Errorf("Expected cursor at 0, got %d", model.treeCursor)
	}

	// Try to move past 0
	model.moveCursorUp()
	if model.treeCursor != 0 {
		t.Errorf("Expected cursor to stay at 0, got %d", model.treeCursor)
	}
}

// Test_wizardModel_moveCursorDown tests cursor movement
func Test_wizardModel_moveCursorDown(t *testing.T) {
	timezones := []string{}
	model := initWizardModel(timezones)

	initialCursor := model.treeCursor
	model.moveCursorDown()

	if model.treeCursor != initialCursor+1 {
		t.Errorf("Expected cursor at %d, got %d", initialCursor+1, model.treeCursor)
	}

	// Move to end
	model.treeCursor = len(model.flatTree) - 1
	lastPos := model.treeCursor
	model.moveCursorDown()

	if model.treeCursor != lastPos {
		t.Errorf("Expected cursor to stay at %d, got %d", lastPos, model.treeCursor)
	}
}

// Test_wizardModel_moveSelectedUp tests selected item movement
func Test_wizardModel_moveSelectedUp(t *testing.T) {
	timezones := []string{"America/New_York", "Europe/London", "Asia/Tokyo"}
	model := initWizardModel(timezones)
	model.focusedPane = selectedPane

	// Start at second item
	model.selectedCursor = 1
	model.moveSelectedUp()

	if model.selectedCursor != 0 {
		t.Errorf("Expected selectedCursor at 0, got %d", model.selectedCursor)
	}

	// Try to move past 0
	model.moveSelectedUp()
	if model.selectedCursor != 0 {
		t.Errorf("Expected selectedCursor to stay at 0, got %d", model.selectedCursor)
	}
}

// Test_wizardModel_moveSelectedDown tests selected item movement
func Test_wizardModel_moveSelectedDown(t *testing.T) {
	timezones := []string{"America/New_York", "Europe/London", "Asia/Tokyo"}
	model := initWizardModel(timezones)
	model.focusedPane = selectedPane

	// Start at first item
	model.selectedCursor = 0
	model.moveSelectedDown()

	if model.selectedCursor != 1 {
		t.Errorf("Expected selectedCursor at 1, got %d", model.selectedCursor)
	}

	// Move to end
	model.selectedCursor = len(model.selected) - 1
	lastPos := model.selectedCursor
	model.moveSelectedDown()

	if model.selectedCursor != lastPos {
		t.Errorf("Expected selectedCursor to stay at %d, got %d", lastPos, model.selectedCursor)
	}
}

// Test_wizardModel_toggleExpand tests the toggleExpand function
func Test_wizardModel_toggleExpand(t *testing.T) {
	timezones := []string{}
	model := initWizardModel(timezones)

	// Find an area node
	var areaIndex int
	for i, entry := range model.flatTree {
		if entry.isArea() {
			areaIndex = i
			break
		}
	}

	model.treeCursor = areaIndex
	model.tree[model.flatTree[areaIndex].areaIdx].expanded = true

	model.toggleExpand()

	// Refresh flatTree after expansion
	model.flatTree = flattenTree(model.tree)
}

// Test_wizardModel_enterSearchMode tests entering search mode
func Test_wizardModel_enterSearchMode(t *testing.T) {
	timezones := []string{}
	model := initWizardModel(timezones)

	if model.searchMode {
		t.Error("Should not start in search mode")
	}

	model.enterSearchMode()

	if !model.searchMode {
		t.Error("Should be in search mode after enterSearchMode")
	}
}

// Test_wizardModel_exitSearchMode tests exiting search mode
func Test_wizardModel_exitSearchMode(t *testing.T) {
	timezones := []string{}
	model := initWizardModel(timezones)

	model.enterSearchMode()
	model.searchQuery = "test"
	model.searchResults = []searchMatch{{}}

	model.exitSearchMode(false)

	if model.searchMode {
		t.Error("Should not be in search mode after exitSearchMode")
	}
	if model.searchQuery != "" {
		t.Error("Search query should be cleared")
	}
	if len(model.searchResults) != 0 {
		t.Error("Search results should be cleared")
	}
}

// Test_wizardModel_performSearch tests the search functionality
func Test_wizardModel_performSearch(t *testing.T) {
	timezones := []string{}
	model := initWizardModel(timezones)

	model.searchQuery = "New_York"
	model.performSearch()

	if len(model.searchResults) == 0 {
		t.Error("Expected search results for 'New_York'")
	}

	// Verify search result contains the expected timezone
	found := false
	for _, result := range model.searchResults {
		if result.fullPath == "America/New_York" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected America/New_York in search results")
	}
}

// Test_wizardModel_performSearch_caseInsensitive tests case-insensitive search
func Test_wizardModel_performSearch_caseInsensitive(t *testing.T) {
	timezones := []string{}
	model := initWizardModel(timezones)

	model.searchQuery = "new_york"
	model.performSearch()

	if len(model.searchResults) == 0 {
		t.Error("Expected search results for case-insensitive 'new_york'")
	}
}

// Test_wizardModel_highlightMatch tests the highlightMatch method
func Test_wizardModel_highlightMatch(t *testing.T) {
	model := initWizardModel([]string{})

	tests := []struct {
		name  string
		text  string
		query string
	}{
		{
			name:  "basic match",
			text:  "America/New_York",
			query: "New",
		},
		{
			name:  "no match",
			text:  "Europe/London",
			query: "Tokyo",
		},
		{
			name:  "case insensitive match",
			text:  "America/New_York",
			query: "new",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model.searchQuery = tt.query
			result := model.highlightMatch(tt.text)
			// Just verify it doesn't panic and returns something
			if result == "" {
				t.Error("Expected non-empty result")
			}
		})
	}
}

// Test_wizardModel_removeSelected tests removing from selected list
func Test_wizardModel_removeSelected(t *testing.T) {
	timezones := []string{"America/New_York", "Europe/London"}
	model := initWizardModel(timezones)
	model.focusedPane = selectedPane
	model.selectedCursor = 0

	model.removeSelected()

	if len(model.selected) != 1 {
		t.Errorf("Expected 1 selected timezone, got %d", len(model.selected))
	}
}

// Test_wizardModel_toggleSelection tests toggling timezone selection
func Test_wizardModel_toggleSelection(t *testing.T) {
	timezones := []string{}
	model := initWizardModel(timezones)

	// Find a non-area node (timezone) by expanding first area
	model.tree[0].expanded = true
	model.flatTree = flattenTree(model.tree)

	var tzIndex int
	for i, entry := range model.flatTree {
		if !entry.isArea() {
			tzIndex = i
			break
		}
	}

	model.treeCursor = tzIndex
	initialLen := len(model.selected)

	// Toggle on
	model.toggleSelection()

	if len(model.selected) != initialLen+1 {
		t.Errorf("Expected %d selected timezones, got %d", initialLen+1, len(model.selected))
	}

	// Toggle off
	model.toggleSelection()

	if len(model.selected) != initialLen {
		t.Errorf("Expected %d selected timezones after toggle off, got %d", initialLen, len(model.selected))
	}
}

// Test_wizardModel_toggleSelection_inSelectedPane tests toggling when focused on selected pane
func Test_wizardModel_toggleSelection_inSelectedPane(t *testing.T) {
	timezones := []string{"America/New_York", "Europe/London"}
	model := initWizardModel(timezones)
	model.focusedPane = selectedPane
	model.selectedCursor = 0

	model.toggleSelection()

	// Should remove the selected item
	if len(model.selected) != 1 {
		t.Errorf("Expected 1 selected timezone after toggle in selected pane, got %d", len(model.selected))
	}
}

// Test_wizardModel_toggleSelection_areaNode tests toggling an entire area
func Test_wizardModel_toggleSelection_areaNode(t *testing.T) {
	// Start with all timezones from first area selected
	model := initWizardModel([]string{})

	// Find first non-System area (since System only has Local)
	var areaIndex int
	for i := range model.flatTree {
		node := model.getNodeFromFlatIndex(i)
		if node != nil && node.nodeType == areaNode && node.name != "System" {
			areaIndex = i
			break
		}
	}

	model.treeCursor = areaIndex
	node := model.getNodeFromFlatIndex(areaIndex)
	if node == nil {
		t.Fatal("Could not find area node")
	}

	// Toggle should add all locations in the area
	model.toggleSelection()

	// Verify some children are selected
	childCount := len(node.children)
	if childCount > 0 {
		selectedCount := 0
		for _, child := range node.children {
			if model.isInSelected(child.fullPath) {
				selectedCount++
			}
		}
		if selectedCount != childCount {
			t.Errorf("Expected %d children selected, got %d", childCount, selectedCount)
		}
	}

	// Toggle again should remove all
	model.toggleSelection()

	for _, child := range node.children {
		if model.isInSelected(child.fullPath) {
			t.Errorf("Expected %s to be deselected", child.fullPath)
		}
	}
}

// Test_wizardModel_toggleSelection_nilNode tests toggle with invalid cursor
func Test_wizardModel_toggleSelection_nilNode(t *testing.T) {
	model := initWizardModel([]string{})
	model.treeCursor = -1

	// Should not panic
	model.toggleSelection()
}

// Test_wizardModel_moveCursorUp_selectedPane tests cursor up in selected pane
func Test_wizardModel_moveCursorUp_selectedPane(t *testing.T) {
	model := initWizardModel([]string{"America/New_York", "Europe/London"})
	model.focusedPane = selectedPane
	model.selectedCursor = 1

	model.moveCursorUp()

	if model.selectedCursor != 0 {
		t.Errorf("Expected selectedCursor at 0, got %d", model.selectedCursor)
	}

	// Try to go past 0
	model.moveCursorUp()
	if model.selectedCursor != 0 {
		t.Errorf("Expected selectedCursor to stay at 0, got %d", model.selectedCursor)
	}
}

// Test_wizardModel_moveCursorDown_selectedPane tests cursor down in selected pane
func Test_wizardModel_moveCursorDown_selectedPane(t *testing.T) {
	model := initWizardModel([]string{"America/New_York", "Europe/London"})
	model.focusedPane = selectedPane
	model.selectedCursor = 0

	model.moveCursorDown()

	if model.selectedCursor != 1 {
		t.Errorf("Expected selectedCursor at 1, got %d", model.selectedCursor)
	}

	// Try to go past end
	model.moveCursorDown()
	if model.selectedCursor != 1 {
		t.Errorf("Expected selectedCursor to stay at 1, got %d", model.selectedCursor)
	}
}

// Test_wizardModel_removeSelected_empty tests removing from empty selected
func Test_wizardModel_removeSelected_empty(t *testing.T) {
	model := initWizardModel([]string{})
	model.focusedPane = selectedPane

	// Should not panic
	model.removeSelected()

	if len(model.selected) != 0 {
		t.Error("Selected should remain empty")
	}
}

// Test_wizardModel_removeSelected_cursorAdjustment tests cursor adjusts after remove
func Test_wizardModel_removeSelected_cursorAdjustment(t *testing.T) {
	model := initWizardModel([]string{"A", "B", "C"})
	model.focusedPane = selectedPane
	model.selectedCursor = 2 // Last item

	model.removeSelected()

	// Cursor should adjust to new last position
	if model.selectedCursor != 1 {
		t.Errorf("Expected cursor to adjust to 1, got %d", model.selectedCursor)
	}
}

// Test_wizardModel_toggleExpand_nonArea tests toggleExpand on non-area node
func Test_wizardModel_toggleExpand_nonArea(t *testing.T) {
	model := initWizardModel([]string{})

	// Expand first area to get child nodes
	model.tree[0].expanded = true
	model.flatTree = flattenTree(model.tree)

	// Find a non-area node
	var childIndex int
	for i, entry := range model.flatTree {
		if !entry.isArea() {
			childIndex = i
			break
		}
	}

	model.treeCursor = childIndex
	flatLenBefore := len(model.flatTree)

	model.toggleExpand()

	// Should not change flatTree length (non-area nodes can't expand)
	if len(model.flatTree) != flatLenBefore {
		t.Error("FlatTree should not change when toggling non-area node")
	}
}

// Test_wizardModel_toggleExpand_nilNode tests toggleExpand with invalid cursor
func Test_wizardModel_toggleExpand_nilNode(t *testing.T) {
	model := initWizardModel([]string{})
	model.treeCursor = -1

	// Should not panic
	model.toggleExpand()
}

// Test_wizardModel_exitSearchMode_keepLocation tests exitSearchMode with keepExpansion=true
func Test_wizardModel_exitSearchMode_keepExpansion(t *testing.T) {
	model := initWizardModel([]string{})
	model.enterSearchMode()

	// Expand America during search
	var americaIdx int
	for i, node := range model.tree {
		if node.name == "America" {
			americaIdx = i
			model.tree[i].expanded = true
			break
		}
	}
	model.flatTree = flattenTree(model.tree)

	model.searchQuery = "New_York"
	model.performSearch()

	// Exit and keep current expansion state
	model.exitSearchMode(true)

	if model.searchMode {
		t.Error("Should not be in search mode")
	}
	// America should still be expanded since we kept expansion
	if !model.tree[americaIdx].expanded {
		t.Error("America should still be expanded when keeping expansion state")
	}
}

// Test_wizardModel_performSearch_empty tests search with no results
func Test_wizardModel_performSearch_empty(t *testing.T) {
	model := initWizardModel([]string{})
	model.searchQuery = "xyznonexistent123"

	model.performSearch()

	if len(model.searchResults) != 0 {
		t.Errorf("Expected no search results, got %d", len(model.searchResults))
	}
}

// Test_wizardModel_moveSelectedUp_swapsItems tests that moveSelectedUp swaps items correctly
func Test_wizardModel_moveSelectedUp_swapsItems(t *testing.T) {
	model := initWizardModel([]string{"A", "B", "C"})
	model.focusedPane = selectedPane
	model.selectedCursor = 2

	model.moveSelectedUp()

	if model.selected[1] != "C" || model.selected[2] != "B" {
		t.Errorf("Items should have swapped: got %v", model.selected)
	}
}

// Test_wizardModel_moveSelectedDown_swapsItems tests that moveSelectedDown swaps items correctly
func Test_wizardModel_moveSelectedDown_swapsItems(t *testing.T) {
	model := initWizardModel([]string{"A", "B", "C"})
	model.focusedPane = selectedPane
	model.selectedCursor = 0

	model.moveSelectedDown()

	if model.selected[0] != "B" || model.selected[1] != "A" {
		t.Errorf("Items should have swapped: got %v", model.selected)
	}
}

// Test_wizardModel_View tests the View method doesn't panic
func Test_wizardModel_View(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*wizardModel)
	}{
		{
			name: "basic model",
			setupFunc: func(_ *wizardModel) {
				// No setup needed
			},
		},
		{
			name: "with selected timezones",
			setupFunc: func(m *wizardModel) {
				m.selected = []string{"America/New_York", "Europe/London"}
			},
		},
		{
			name: "in search mode",
			setupFunc: func(m *wizardModel) {
				m.searchMode = true
				m.searchQuery = "test"
			},
		},
		{
			name: "quitting with saved",
			setupFunc: func(m *wizardModel) {
				m.quitting = true
				m.saved = true
			},
		},
		{
			name: "quitting without saved",
			setupFunc: func(m *wizardModel) {
				m.quitting = true
				m.saved = false
			},
		},
		{
			name: "with search results",
			setupFunc: func(m *wizardModel) {
				m.searchMode = true
				m.searchQuery = "New"
				m.searchResults = []searchMatch{
					{fullPath: "America/New_York", areaIdx: 0, childIdx: 0},
				}
			},
		},
		{
			name: "focused on selected pane",
			setupFunc: func(m *wizardModel) {
				m.focusedPane = selectedPane
				m.selected = []string{"America/New_York"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := initWizardModel([]string{})
			model.width = 120
			model.height = 40
			tt.setupFunc(&model)

			// Should not panic
			result := model.View()
			if result == "" {
				t.Error("View should return non-empty string")
			}
		})
	}
}

// Test_wizardModel_Init tests the Init method
func Test_wizardModel_Init(t *testing.T) {
	model := initWizardModel([]string{})
	cmd := model.Init()
	if cmd != nil {
		t.Error("Init should return nil")
	}
}

// Test_wizardModel_Update_windowSize tests Update handles window size messages
func Test_wizardModel_Update_windowSize(t *testing.T) {
	model := initWizardModel([]string{})

	newModel, cmd := model.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	if cmd != nil {
		t.Error("Expected no command from window size update")
	}

	updatedModel, ok := newModel.(wizardModel)
	if !ok {
		t.Fatal("Expected wizardModel type")
	}

	if updatedModel.width != 100 || updatedModel.height != 50 {
		t.Errorf("Expected dimensions 100x50, got %dx%d", updatedModel.width, updatedModel.height)
	}
}

// Test_wizardModel_Update_keyMessages tests Update with various key messages
func Test_wizardModel_Update_keyMessages(t *testing.T) {
	tests := []struct {
		name       string
		key        tea.KeyMsg
		setupFunc  func(*wizardModel)
		checkFunc  func(*testing.T, wizardModel)
		expectQuit bool
	}{
		{
			name: "quit key",
			key:  tea.KeyMsg{Type: tea.KeyCtrlC},
			checkFunc: func(t *testing.T, m wizardModel) {
				if !m.quitting {
					t.Error("Expected quitting to be true")
				}
				if !m.saved {
					t.Error("Expected saved to be true")
				}
			},
			expectQuit: true,
		},
		{
			name: "tab key switches pane from selected to available",
			key:  tea.KeyMsg{Type: tea.KeyTab},
			setupFunc: func(m *wizardModel) {
				m.focusedPane = selectedPane
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				if m.focusedPane != availablePane {
					t.Error("Expected focusedPane to be availablePane")
				}
			},
		},
		{
			name: "tab key switches pane from available to selected",
			key:  tea.KeyMsg{Type: tea.KeyTab},
			setupFunc: func(m *wizardModel) {
				m.focusedPane = availablePane
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				if m.focusedPane != selectedPane {
					t.Error("Expected focusedPane to be selectedPane")
				}
			},
		},
		{
			name: "up arrow moves cursor up",
			key:  tea.KeyMsg{Type: tea.KeyUp},
			setupFunc: func(m *wizardModel) {
				m.treeCursor = 2
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				if m.treeCursor != 1 {
					t.Errorf("Expected treeCursor 1, got %d", m.treeCursor)
				}
			},
		},
		{
			name: "down arrow moves cursor down",
			key:  tea.KeyMsg{Type: tea.KeyDown},
			setupFunc: func(m *wizardModel) {
				m.treeCursor = 0
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				if m.treeCursor != 1 {
					t.Errorf("Expected treeCursor 1, got %d", m.treeCursor)
				}
			},
		},
		{
			name: "space toggles selection",
			key:  tea.KeyMsg{Type: tea.KeySpace},
			setupFunc: func(m *wizardModel) {
				m.tree[0].expanded = true
				m.flatTree = flattenTree(m.tree)
				m.treeCursor = 1 // First child (Local in System)
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				// Just verify it ran without panic
			},
		},
		{
			name: "enter toggles expand on area",
			key:  tea.KeyMsg{Type: tea.KeyEnter},
			setupFunc: func(m *wizardModel) {
				m.focusedPane = availablePane
				m.treeCursor = 1 // Second area (first non-System)
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				// Just verify it ran without panic
			},
		},
		{
			name: "delete removes selected in selected pane",
			key:  tea.KeyMsg{Type: tea.KeyDelete},
			setupFunc: func(m *wizardModel) {
				m.focusedPane = selectedPane
				m.selected = []string{"America/New_York", "Europe/London"}
				m.selectedCursor = 0
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				if len(m.selected) != 1 {
					t.Errorf("Expected 1 selected, got %d", len(m.selected))
				}
			},
		},
		{
			name: "slash enters search mode",
			key:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}},
			checkFunc: func(t *testing.T, m wizardModel) {
				if !m.searchMode {
					t.Error("Expected searchMode to be true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := initWizardModel([]string{})
			if tt.setupFunc != nil {
				tt.setupFunc(&model)
			}

			newModel, cmd := model.Update(tt.key)

			if tt.expectQuit {
				if cmd == nil {
					t.Error("Expected quit command")
				}
			}

			updatedModel, ok := newModel.(wizardModel)
			if !ok {
				t.Fatal("Expected wizardModel type")
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, updatedModel)
			}
		})
	}
}

// Test_wizardModel_Update_shiftKeys tests shift+up and shift+down
func Test_wizardModel_Update_shiftKeys(t *testing.T) {
	tests := []struct {
		name      string
		key       tea.KeyMsg
		setupFunc func(*wizardModel)
		checkFunc func(*testing.T, wizardModel)
	}{
		{
			name: "shift+up moves selected up",
			key:  tea.KeyMsg{Type: tea.KeyShiftUp},
			setupFunc: func(m *wizardModel) {
				m.focusedPane = selectedPane
				m.selected = []string{"A", "B", "C"}
				m.selectedCursor = 1
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				if m.selected[0] != "B" || m.selected[1] != "A" {
					t.Errorf("Expected items to swap: got %v", m.selected)
				}
			},
		},
		{
			name: "shift+down moves selected down",
			key:  tea.KeyMsg{Type: tea.KeyShiftDown},
			setupFunc: func(m *wizardModel) {
				m.focusedPane = selectedPane
				m.selected = []string{"A", "B", "C"}
				m.selectedCursor = 1
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				if m.selected[1] != "C" || m.selected[2] != "B" {
					t.Errorf("Expected items to swap: got %v", m.selected)
				}
			},
		},
		{
			name: "shift+up does nothing in available pane",
			key:  tea.KeyMsg{Type: tea.KeyShiftUp},
			setupFunc: func(m *wizardModel) {
				m.focusedPane = availablePane
				m.selected = []string{"A", "B", "C"}
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				if m.selected[0] != "A" || m.selected[1] != "B" {
					t.Errorf("Expected items to remain unchanged: got %v", m.selected)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := initWizardModel([]string{})
			if tt.setupFunc != nil {
				tt.setupFunc(&model)
			}

			newModel, _ := model.Update(tt.key)
			updatedModel := newModel.(wizardModel)
			tt.checkFunc(t, updatedModel)
		})
	}
}

// Test_wizardModel_handleSearchInput tests the search input handler
func Test_wizardModel_handleSearchInput(t *testing.T) {
	tests := []struct {
		name      string
		key       tea.KeyMsg
		setupFunc func(*wizardModel)
		checkFunc func(*testing.T, wizardModel)
	}{
		{
			name: "escape exits search mode",
			key:  tea.KeyMsg{Type: tea.KeyEsc},
			setupFunc: func(m *wizardModel) {
				m.searchMode = true
				m.searchQuery = "test"
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				if m.searchMode {
					t.Error("Expected searchMode to be false")
				}
			},
		},
		{
			name: "up arrow moves search cursor up",
			key:  tea.KeyMsg{Type: tea.KeyUp},
			setupFunc: func(m *wizardModel) {
				m.searchMode = true
				m.searchQuery = "America"
				m.performSearch()
				m.searchCursor = 2
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				if m.searchCursor != 1 {
					t.Errorf("Expected searchCursor 1, got %d", m.searchCursor)
				}
			},
		},
		{
			name: "down arrow moves search cursor down",
			key:  tea.KeyMsg{Type: tea.KeyDown},
			setupFunc: func(m *wizardModel) {
				m.searchMode = true
				m.searchQuery = "America"
				m.performSearch()
				m.searchCursor = 0
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				if m.searchCursor != 1 {
					t.Errorf("Expected searchCursor 1, got %d", m.searchCursor)
				}
			},
		},
		{
			name: "space toggles selection in search",
			key:  tea.KeyMsg{Type: tea.KeySpace},
			setupFunc: func(m *wizardModel) {
				m.searchMode = true
				m.searchQuery = "New_York"
				m.performSearch()
				m.searchCursor = 0
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				// Just check it doesn't panic
			},
		},
		{
			name: "enter selects and exits search",
			key:  tea.KeyMsg{Type: tea.KeyEnter},
			setupFunc: func(m *wizardModel) {
				m.searchMode = true
				m.searchQuery = "New_York"
				m.performSearch()
				m.searchCursor = 0
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				if m.searchMode {
					t.Error("Expected searchMode to be false after enter")
				}
			},
		},
		{
			name: "backspace removes last char",
			key:  tea.KeyMsg{Type: tea.KeyBackspace},
			setupFunc: func(m *wizardModel) {
				m.searchMode = true
				m.searchQuery = "test"
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				if m.searchQuery != "tes" {
					t.Errorf("Expected 'tes', got %q", m.searchQuery)
				}
			},
		},
		{
			name: "backspace on empty query does nothing",
			key:  tea.KeyMsg{Type: tea.KeyBackspace},
			setupFunc: func(m *wizardModel) {
				m.searchMode = true
				m.searchQuery = ""
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				if m.searchQuery != "" {
					t.Errorf("Expected empty string, got %q", m.searchQuery)
				}
			},
		},
		{
			name: "typing adds to query",
			key:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}},
			setupFunc: func(m *wizardModel) {
				m.searchMode = true
				m.searchQuery = "test"
			},
			checkFunc: func(t *testing.T, m wizardModel) {
				if m.searchQuery != "testa" {
					t.Errorf("Expected 'testa', got %q", m.searchQuery)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := initWizardModel([]string{})
			if tt.setupFunc != nil {
				tt.setupFunc(&model)
			}

			newModel, _ := model.Update(tt.key)
			updatedModel := newModel.(wizardModel)
			tt.checkFunc(t, updatedModel)
		})
	}
}

// Test_wizardModel_renderSelectedPane tests the renderSelectedPane method
func Test_wizardModel_renderSelectedPane(t *testing.T) {
	tests := []struct {
		name     string
		selected []string
	}{
		{name: "empty", selected: []string{}},
		{name: "one item", selected: []string{"America/New_York"}},
		{name: "multiple items", selected: []string{"America/New_York", "Europe/London", "Asia/Tokyo"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := initWizardModel(tt.selected)
			model.width = 120
			model.height = 40

			// Should not panic
			result := model.renderSelectedPane(30, 20)
			if result == "" {
				t.Error("renderSelectedPane should return non-empty string")
			}
		})
	}
}

// Test_wizardModel_renderAvailablePane tests the renderAvailablePane method
func Test_wizardModel_renderAvailablePane(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*wizardModel)
	}{
		{
			name:      "basic tree",
			setupFunc: func(_ *wizardModel) {},
		},
		{
			name: "expanded area",
			setupFunc: func(m *wizardModel) {
				m.tree[0].expanded = true
				m.flatTree = flattenTree(m.tree)
			},
		},
		{
			name: "in search mode",
			setupFunc: func(m *wizardModel) {
				m.searchMode = true
				m.searchQuery = "New"
				m.performSearch()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := initWizardModel([]string{})
			model.width = 120
			model.height = 40
			tt.setupFunc(&model)

			// Should not panic
			result := model.renderAvailablePane(60, 30)
			if result == "" {
				t.Error("renderAvailablePane should return non-empty string")
			}
		})
	}
}

// Test_wizardModel_renderSearchResults tests the renderSearchResults method
func Test_wizardModel_renderSearchResults(t *testing.T) {
	model := initWizardModel([]string{})
	model.searchMode = true
	model.searchQuery = "New"
	model.performSearch()
	model.width = 120
	model.height = 40

	// Should not panic
	result := model.renderSearchResults(60, 30)
	if result == "" {
		t.Error("renderSearchResults should return non-empty string")
	}
}

// Test_wizardModel_renderTreeNode tests the renderTreeNode method
func Test_wizardModel_renderTreeNode(t *testing.T) {
	model := initWizardModel([]string{"America/New_York"})
	model.width = 120
	model.height = 40

	// Get an area node - expand first area to get children
	model.tree[0].expanded = true
	model.flatTree = flattenTree(model.tree)

	tests := []struct {
		name     string
		nodeType nodeType
	}{
		{name: "area node", nodeType: areaNode},
		{name: "location node", nodeType: locationNode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test node
			node := &treeNode{
				name:     "TestNode",
				nodeType: tt.nodeType,
				fullPath: "Test/TestNode",
				children: []treeNode{
					{name: "Child", fullPath: "Test/Child", nodeType: locationNode},
				},
			}

			// Should not panic
			result := model.renderTreeNode(node, 50)
			if result == "" {
				t.Error("renderTreeNode should return non-empty string")
			}
		})
	}
}

// Test_wizardModel_renderTreeNode_selected tests renderTreeNode with selected nodes
func Test_wizardModel_renderTreeNode_selected(t *testing.T) {
	model := initWizardModel([]string{"Test/TestNode"})

	node := &treeNode{
		name:       "TestNode",
		nodeType:   locationNode,
		fullPath:   "Test/TestNode",
		isSelected: true,
	}

	result := model.renderTreeNode(node, 50)
	if result == "" {
		t.Error("renderTreeNode should return non-empty string")
	}
}

// Test_wizardModel_renderTreeNode_expanded tests renderTreeNode with expanded area
func Test_wizardModel_renderTreeNode_expanded(t *testing.T) {
	model := initWizardModel([]string{"America/New_York"})

	node := &treeNode{
		name:     "America",
		nodeType: areaNode,
		fullPath: "America",
		expanded: true,
		children: []treeNode{
			{name: "New_York", fullPath: "America/New_York", nodeType: locationNode, isSelected: true},
			{name: "Chicago", fullPath: "America/Chicago", nodeType: locationNode},
		},
	}

	result := model.renderTreeNode(node, 50)
	if result == "" {
		t.Error("renderTreeNode should return non-empty string")
	}
}

// Test_wizardModel_renderHelp tests the renderHelp method
func Test_wizardModel_renderHelp(t *testing.T) {
	tests := []struct {
		name       string
		searchMode bool
		pane       pane
	}{
		{name: "normal mode available pane", searchMode: false, pane: availablePane},
		{name: "normal mode selected pane", searchMode: false, pane: selectedPane},
		{name: "search mode", searchMode: true, pane: availablePane},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := initWizardModel([]string{})
			model.searchMode = tt.searchMode
			model.focusedPane = tt.pane

			// Should not panic
			result := model.renderHelp()
			if result == "" {
				t.Error("renderHelp should return non-empty string")
			}
		})
	}
}

// Test_wizardModel_exitSearchMode_restoresExpansion tests that exitSearchMode restores expansion state
func Test_wizardModel_exitSearchMode_restoresExpansion(t *testing.T) {
	model := initWizardModel([]string{})

	// Note the initial expansion state
	var initiallyExpanded bool
	for _, node := range model.tree {
		if node.name == "America" {
			initiallyExpanded = node.expanded
			break
		}
	}

	model.enterSearchMode()

	// Expand America during search
	for i := range model.tree {
		if model.tree[i].name == "America" {
			model.tree[i].expanded = true
			break
		}
	}
	model.flatTree = flattenTree(model.tree)

	// Exit without keeping expansion (should restore)
	model.exitSearchMode(false)

	var afterExpanded bool
	for _, node := range model.tree {
		if node.name == "America" {
			afterExpanded = node.expanded
			break
		}
	}

	if afterExpanded != initiallyExpanded {
		t.Errorf("Expected America expanded=%v after restore, got %v", initiallyExpanded, afterExpanded)
	}
}
