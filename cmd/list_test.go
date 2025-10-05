package cmd

import (
	"slices"
	"testing"
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

func TestTimezonesAllNotEmpty(t *testing.T) {
	if len(timezonesAll) == 0 {
		t.Error("timezonesAll should not be empty")
	}
}

func TestTimezonesAllContainsKnownTimezones(t *testing.T) {
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

func TestListCmdExists(t *testing.T) {
	if listCmd == nil {
		t.Error("listCmd should not be nil")
	}
}

func TestListAreas(t *testing.T) {
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

func TestListCmdFlags(t *testing.T) {
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
