package cmd

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// assertTimezoneExists checks if a timezone exists in the global list
func assertTimezoneExists(t *testing.T, timezone string) {
	t.Helper()
	if slices.Contains(timezonesAll, timezone) {
		return
	}
	t.Errorf("Expected timezone '%s' to be in timezonesAll", timezone)
}

// assertLocationInArea checks if a location exists in a specific area
func assertLocationInArea(t *testing.T, areas map[string][]string, area, location string) {
	t.Helper()
	locations, exists := areas[area]
	if !exists {
		t.Errorf("Area '%s' does not exist in result", area)
		return
	}
	for _, loc := range locations {
		if loc == location {
			return
		}
	}
	t.Errorf("Expected '%s' to be in %s locations", location, area)
}

func Test_timezonesAll_notEmpty(t *testing.T) {
	t.Parallel()
	if len(timezonesAll) == 0 {
		t.Error("timezonesAll should not be empty")
	}
}

func Test_timezonesAll_containsKnown(t *testing.T) {
	t.Parallel()
	knownTimezones := []string{
		"America/New_York",
		"Europe/London",
		"Asia/Tokyo",
		"Australia/Sydney",
		"UTC",
	}

	for _, tz := range knownTimezones {
		t.Run(tz, func(t *testing.T) {
			t.Parallel()
			assertTimezoneExists(t, tz)
		})
	}
}

func Test_NewListCmd_exists(t *testing.T) {
	t.Parallel()
	listCmd := NewListCmd()
	if listCmd == nil {
		t.Error("NewListCmd() should not return nil")
	}
}

func Test_listAreas(t *testing.T) {
	t.Parallel()
	result := listAreas()

	if len(result) == 0 {
		t.Fatal("listAreas() returned empty map")
	}

	t.Run("known areas exist", func(t *testing.T) {
		t.Parallel()
		knownAreas := []string{"America", "Europe", "Asia", "Australia", "Africa"}
		for _, area := range knownAreas {
			if _, exists := result[area]; !exists {
				t.Errorf("Expected area '%s' to exist in listAreas() result", area)
			}
		}
	})

	t.Run("timezone parsing", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			area     string
			location string
		}{
			{"America", "New_York"},
			{"Europe", "London"},
			{"Asia", "Tokyo"},
			{"Australia", "Sydney"},
		}

		for _, tt := range tests {
			t.Run(tt.area+"/"+tt.location, func(t *testing.T) {
				t.Parallel()
				assertLocationInArea(t, result, tt.area, tt.location)
			})
		}
	})
}

func Test_NewListCmd_flags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		flagName string
	}{
		{name: "areas flag exists", flagName: "areas"},
		{name: "locations flag exists", flagName: "locations"},
		{name: "timezones flag exists", flagName: "timezones"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			listCmd := NewListCmd()
			flag := listCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("listCmd should have a '%s' flag", tt.flagName)
			}
		})
	}
}

// Test_validateListArgs tests the validateListArgs function via the command
func Test_validateListArgs_via_cmd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		setupCmd      func(*cobra.Command)
		expectError   bool
		errorContains string
	}{
		{
			name: "no flags - valid",
			setupCmd: func(cmd *cobra.Command) {
				// No flags set
			},
			expectError: false,
		},
		{
			name: "valid location flag - America",
			setupCmd: func(cmd *cobra.Command) {
				_ = cmd.Flags().Set("locations", "America")
			},
			expectError: false,
		},
		{
			name: "invalid location flag",
			setupCmd: func(cmd *cobra.Command) {
				_ = cmd.Flags().Set("locations", "InvalidArea")
			},
			expectError:   true,
			errorContains: "invalid area name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Create a fresh list command for each test
			listCmd := NewListCmd()

			tt.setupCmd(listCmd)

			// Execute Args validator
			err := listCmd.Args(listCmd, nil)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// Test_runList tests the runList function via the command
func Test_runList_via_cmd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		setupCmd func(*cobra.Command)
	}{
		{
			name: "list areas",
			setupCmd: func(cmd *cobra.Command) {
				_ = cmd.Flags().Set("areas", "true")
			},
		},
		{
			name: "list locations for America",
			setupCmd: func(cmd *cobra.Command) {
				_ = cmd.Flags().Set("locations", "America")
			},
		},
		{
			name: "list all timezones",
			setupCmd: func(cmd *cobra.Command) {
				_ = cmd.Flags().Set("timezones", "true")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Create a fresh list command for each test
			listCmd := NewListCmd()

			tt.setupCmd(listCmd)

			// Execute RunE
			err := listCmd.RunE(listCmd, nil)
			if err != nil {
				t.Errorf("runList failed: %v", err)
			}
		})
	}
}

// Test_printAreas tests the printAreas function
func Test_printAreas(t *testing.T) {
	t.Parallel()
	// Test that it doesn't panic and returns nil
	err := printAreas()
	if err != nil {
		t.Errorf("printAreas failed: %v", err)
	}
}

// Test_printLocations tests the printLocations function
func Test_printLocations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		areaName    string
		expectError bool
	}{
		{
			name:        "valid area America",
			areaName:    "America",
			expectError: false,
		},
		{
			name:        "valid area Europe",
			areaName:    "Europe",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := printLocations(tt.areaName)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// Test_printAllTimezones tests the printAllTimezones function
func Test_printAllTimezones(t *testing.T) {
	t.Parallel()
	err := printAllTimezones()
	if err != nil {
		t.Errorf("printAllTimezones failed: %v", err)
	}
}
