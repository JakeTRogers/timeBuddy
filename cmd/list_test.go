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
	if len(timezonesAll) == 0 {
		t.Error("timezonesAll should not be empty")
	}
}

func Test_timezonesAll_containsKnown(t *testing.T) {
	knownTimezones := []string{
		"America/New_York",
		"Europe/London",
		"Asia/Tokyo",
		"Australia/Sydney",
		"UTC",
	}

	for _, tz := range knownTimezones {
		t.Run(tz, func(t *testing.T) {
			assertTimezoneExists(t, tz)
		})
	}
}

func Test_listCmd_exists(t *testing.T) {
	if listCmd == nil {
		t.Error("listCmd should not be nil")
	}
}

func Test_listAreas(t *testing.T) {
	result := listAreas()

	if len(result) == 0 {
		t.Fatal("listAreas() returned empty map")
	}

	t.Run("known areas exist", func(t *testing.T) {
		knownAreas := []string{"America", "Europe", "Asia", "Australia", "Africa"}
		for _, area := range knownAreas {
			if _, exists := result[area]; !exists {
				t.Errorf("Expected area '%s' to exist in listAreas() result", area)
			}
		}
	})

	t.Run("timezone parsing", func(t *testing.T) {
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
				assertLocationInArea(t, result, tt.area, tt.location)
			})
		}
	})
}

func Test_listCmd_flags(t *testing.T) {
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
			flag := listCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("listCmd should have a '%s' flag", tt.flagName)
			}
		})
	}
}

// Test_validateListArgs tests the validateListArgs function
func Test_validateListArgs(t *testing.T) {
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

	// Save original area value
	originalArea := area
	t.Cleanup(func() {
		area = originalArea
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test command with required flags
			cmd := &cobra.Command{}
			cmd.Flags().StringVarP(&area, "locations", "l", "", "")

			tt.setupCmd(cmd)

			err := validateListArgs(cmd, nil)

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

// Test_runList tests the runList function
func Test_runList(t *testing.T) {
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

	// Save original area value
	originalArea := area
	t.Cleanup(func() {
		area = originalArea
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test command with required flags
			cmd := &cobra.Command{}
			cmd.Flags().Bool("areas", false, "")
			cmd.Flags().StringVarP(&area, "locations", "l", "", "")
			cmd.Flags().Bool("timezones", false, "")

			tt.setupCmd(cmd)

			err := runList(cmd, nil)
			if err != nil {
				t.Errorf("runList failed: %v", err)
			}
		})
	}
}

// Test_printAreas tests the printAreas function
func Test_printAreas(t *testing.T) {
	// Test that it doesn't panic and returns nil
	err := printAreas()
	if err != nil {
		t.Errorf("printAreas failed: %v", err)
	}
}

// Test_printLocations tests the printLocations function
func Test_printLocations(t *testing.T) {
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
			// Create test command with required flags
			cmd := &cobra.Command{}
			cmd.Flags().String("locations", tt.areaName, "")

			err := printLocations(cmd)

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
	err := printAllTimezones()
	if err != nil {
		t.Errorf("printAllTimezones failed: %v", err)
	}
}
