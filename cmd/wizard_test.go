/*
Copyright © 2024 Jake Rogers <code@supportoss.org>
*/
package cmd

import (
	"testing"
)

func TestBuildTree(t *testing.T) {
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

func TestFlattenTree(t *testing.T) {
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

func TestCountSelectedInArea(t *testing.T) {
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

func TestInitWizardModel(t *testing.T) {
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

func TestWizardModelIsInSelected(t *testing.T) {
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

func TestWizardModelUpdateSelectionState(t *testing.T) {
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

func TestWizardModelRemoveFromSelected(t *testing.T) {
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

func TestWizardCmdExists(t *testing.T) {
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
