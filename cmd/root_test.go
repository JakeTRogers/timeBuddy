package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Test fixtures and helpers
var (
	// Common test timezones used across multiple tests
	testZones = timezoneDetails{
		{name: "America/New_York", offset: -5},
		{name: "Europe/London", offset: 0},
		{name: "Asia/Tokyo", offset: 9},
		{name: "Australia/Sydney", offset: 11},
	}

	// Test time for consistent testing
	testTime = time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)
)

// assertError checks if error state matches expectation
func assertError(t *testing.T, err error, expectError bool, errorContains string) {
	t.Helper()
	if expectError {
		if err == nil {
			t.Fatal("expected error but got none")
		}
		if errorContains != "" && !strings.Contains(err.Error(), errorContains) {
			t.Fatalf("expected error to contain %q, got: %v", errorContains, err)
		}
	} else if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// assertEqual checks if two comparable values are equal
func assertEqual[T comparable](t *testing.T, got, want T, format string, args ...any) {
	t.Helper()
	if got == want {
		return
	}
	if format != "" {
		t.Errorf(format, args...)
		return
	}
	t.Errorf("expected %v, got %v", want, got)
}

func ptr[T any](value T) *T {
	return &value
}

func toStrings(values []interface{}) []string {
	result := make([]string, len(values))
	for i, v := range values {
		result[i] = fmt.Sprint(v)
	}
	return result
}

// makeTimezoneDetail creates a test timezone detail
func makeTimezoneDetail(name string, offset int, halfHour bool) timezoneDetail {
	return timezoneDetail{
		name:           name,
		abbreviation:   "TST",
		currentTime:    testTime,
		offset:         offset,
		halfHourOffset: halfHour,
		hours:          []int{0, 1, 2, 12, 13, 23},
	}
}

func TestParseOffset(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedHour   int
		expectedOffset int
		expectError    bool
	}{
		{
			name:           "hour with positive offset",
			input:          "15+11",
			expectedHour:   15,
			expectedOffset: 11,
			expectError:    false,
		},
		{
			name:           "hour with negative offset",
			input:          "9-4",
			expectedHour:   9,
			expectedOffset: -4,
			expectError:    false,
		},
		{
			name:           "hour only (UTC)",
			input:          "12",
			expectedHour:   12,
			expectedOffset: 0,
			expectError:    false,
		},
		{
			name:           "zero hour with offset",
			input:          "0+5",
			expectedHour:   0,
			expectedOffset: 5,
			expectError:    false,
		},
		{
			name:           "hour 23 with negative offset",
			input:          "23-8",
			expectedHour:   23,
			expectedOffset: -8,
			expectError:    false,
		},
		{
			name:           "invalid format with multiple plus signs",
			input:          "15+5+3",
			expectedHour:   0,
			expectedOffset: 0,
			expectError:    true,
		},
		{
			name:           "invalid format with multiple minus signs",
			input:          "15-5-3",
			expectedHour:   0,
			expectedOffset: 0,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hour, offset, err := parseOffset(tt.input)

			assertError(t, err, tt.expectError, "")
			if tt.expectError {
				return
			}

			assertEqual(t, hour, tt.expectedHour, "Expected hour %d, got %d", tt.expectedHour, hour)
			assertEqual(t, offset, tt.expectedOffset, "Expected offset %d, got %d", tt.expectedOffset, offset)
		})
	}
}

func TestParseHighlightFlag(t *testing.T) {
	tests := []struct {
		name           string
		highlight      string
		expectError    bool
		errorContains  string
		expectedHour   int
		expectedOffset int
		expectedIndex  *int
		zones          timezoneDetails
	}{
		{
			name:          "empty highlight",
			highlight:     "",
			expectError:   false,
			expectedIndex: ptr(-1),
		},
		{
			name:           "valid hour with positive offset",
			highlight:      "15+11",
			expectError:    false,
			expectedHour:   15,
			expectedOffset: 11,
		},
		{
			name:           "valid hour with negative offset",
			highlight:      "9-5",
			expectError:    false,
			expectedHour:   9,
			expectedOffset: -5,
		},
		{
			name:           "valid hour UTC",
			highlight:      "12",
			expectError:    false,
			expectedHour:   12,
			expectedOffset: 0,
		},
		{
			name:          "hour out of range (negative)",
			highlight:     "-1+5",
			expectError:   true,
			errorContains: "hour must be between 0 and 23",
		},
		{
			name:          "hour out of range (too large)",
			highlight:     "24+0",
			expectError:   true,
			errorContains: "hour must be between 0 and 23",
		},
		{
			name:          "offset not in configured timezones",
			highlight:     "15+6",
			expectError:   true,
			errorContains: "no configured timezone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zones := tt.zones
			if zones == nil {
				zones = testZones
			}
			index, err := parseHighlightFlag(tt.highlight, zones)

			assertError(t, err, tt.expectError, tt.errorContains)
			if tt.expectError {
				return
			}

			expectedIndex := 0
			if tt.expectedIndex != nil {
				expectedIndex = *tt.expectedIndex
			} else {
				expectedIndex = ((tt.expectedHour - tt.expectedOffset) + 24) % 24
			}
			assertEqual(t, index, expectedIndex, "Expected index %d, got %d", expectedIndex, index)
		})
	}
}

func TestHasTimezoneWithOffset(t *testing.T) {
	tests := []struct {
		name     string
		offset   int
		expected bool
	}{
		{
			name:     "offset exists (-5)",
			offset:   -5,
			expected: true,
		},
		{
			name:     "offset exists (0)",
			offset:   0,
			expected: true,
		},
		{
			name:     "offset exists (9)",
			offset:   9,
			expected: true,
		},
		{
			name:     "offset does not exist (5)",
			offset:   5,
			expected: false,
		},
		{
			name:     "offset does not exist (-8)",
			offset:   -8,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasTimezoneWithOffset(testZones, tt.offset)
			assertEqual(t, result, tt.expected, "Expected %v, got %v", tt.expected, result)
		})
	}
}

func TestDeduplicateSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no duplicates",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "with duplicates",
			input:    []string{"a", "b", "a", "c", "b"},
			expected: []string{"a", "c", "b"},
		},
		{
			name:     "all duplicates",
			input:    []string{"a", "a", "a"},
			expected: []string{"a"},
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "single element",
			input:    []string{"a"},
			expected: []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicateSlice(tt.input)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFormatOffset(t *testing.T) {
	tests := []struct {
		name     string
		offset   int
		halfHour bool
		expected string
	}{
		{"positive whole hour offset", 5, false, "+5"},
		{"negative whole hour offset", -8, false, "-8"},
		{"zero offset", 0, false, "+0"},
		{"positive half hour offset", 5, true, "+5.5"},
		{"negative half hour offset", -9, true, "-9.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zone := makeTimezoneDetail("Test/Zone", tt.offset, tt.halfHour)
			result := formatOffset(zone)
			assertEqual(t, result, tt.expected, "Expected %s, got %s", tt.expected, result)
		})
	}
}

func TestFormatRowLabel(t *testing.T) {
	today := time.Now().Format(time.DateOnly)
	pastDate := "2024-06-15"

	tests := []struct {
		name     string
		zoneName string
		abbrev   string
		date     string
		offset   string
		contains []string
	}{
		{
			name:     "current date with time",
			zoneName: "America/New_York",
			abbrev:   "EDT",
			date:     today,
			offset:   "-5",
			contains: []string{"America/New_York", "EDT", "-5", testTime.Format("3:04PM")},
		},
		{
			name:     "past date without time",
			zoneName: "Europe/London",
			abbrev:   "BST",
			date:     pastDate,
			offset:   "+1",
			contains: []string{"Europe/London", "BST", "+1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zone := timezoneDetail{
				name:         tt.zoneName,
				abbreviation: tt.abbrev,
				currentTime:  testTime,
			}
			result := formatRowLabel(zone, tt.date, tt.offset)
			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("Expected result to contain '%s', got: %s", substr, result)
				}
			}
		})
	}
}

func TestFormatHours(t *testing.T) {
	zone := makeTimezoneDetail("America/New_York", -5, false)

	tests := []struct {
		name              string
		twelveHourEnabled bool
		expected          func(timezoneDetail) []string
	}{
		{
			name:              "24-hour format",
			twelveHourEnabled: false,
			expected: func(zone timezoneDetail) []string {
				return []string{zone.currentTime.Format("Mon"), " 1", " 2", "12", "13", "23"}
			},
		},
		{
			name:              "12-hour format",
			twelveHourEnabled: true,
			expected: func(zone timezoneDetail) []string {
				return []string{zone.currentTime.Format("Mon"), " 1\nam", " 2\nam", "12\nam", " 1\npm", "11\npm"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatHours(zone, tt.twelveHourEnabled)
			expected := tt.expected(zone)
			if !slices.Equal(toStrings(result), expected) {
				t.Errorf("Expected %v, got %v", expected, toStrings(result))
			}
		})
	}
}

func TestGetZoneInfo(t *testing.T) {
	tests := []struct {
		name     string
		timezone string
		date     string
		validate func(t *testing.T, zone timezoneDetail)
	}{
		{
			name:     "valid timezone UTC",
			timezone: "UTC",
			date:     time.Now().Format(time.DateOnly),
			validate: func(t *testing.T, zone timezoneDetail) {
				if zone.name != "UTC" {
					t.Errorf("Expected name 'UTC', got '%s'", zone.name)
				}
				if zone.offset != 0 {
					t.Errorf("Expected offset 0, got %d", zone.offset)
				}
				if len(zone.hours) != 24 {
					t.Errorf("Expected 24 hours, got %d", len(zone.hours))
				}
			},
		},
		{
			name:     "valid timezone America/New_York",
			timezone: "America/New_York",
			date:     "2024-06-15",
			validate: func(t *testing.T, zone timezoneDetail) {
				if zone.name != "America/New_York" {
					t.Errorf("Expected name 'America/New_York', got '%s'", zone.name)
				}
				if len(zone.hours) != 24 {
					t.Errorf("Expected 24 hours, got %d", len(zone.hours))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zone := getZoneInfo(tt.timezone, tt.date)
			tt.validate(t, zone)
		})
	}
}

func TestGetHours(t *testing.T) {
	loc, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatalf("Failed to load UTC location: %v", err)
	}

	date := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	hours := getHours(date, loc)

	if len(hours) != 24 {
		t.Errorf("Expected 24 hours, got %d", len(hours))
	}

	// Verify hours are sequential
	for i := 0; i < len(hours)-1; i++ {
		diff := hours[i+1].Sub(hours[i])
		if diff != time.Hour {
			t.Errorf("Expected 1 hour difference between hours[%d] and hours[%d], got %v", i, i+1, diff)
		}
	}
}

func TestInitializeConfig(t *testing.T) {
	// Create a temporary directory for test config
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	configDir := filepath.Join(tempDir, ".config")

	// Create the config directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Test initialization
	err := initializeConfig(rootCmd)
	if err != nil {
		t.Errorf("initializeConfig failed: %v", err)
	}
}

// TestProcessTimezones tests the processTimezones function
func TestProcessTimezones(t *testing.T) {
	// Save original timezones
	originalTimezones := timezones
	originalDate := date
	t.Cleanup(func() {
		timezones = originalTimezones
		date = originalDate
	})

	timezones = []string{"UTC", "America/New_York"}
	date = testTime.Format(time.DateOnly)

	zones := processTimezones()

	if len(zones) != 2 {
		t.Errorf("Expected 2 zones, got %d", len(zones))
	}

	if zones[0].name != "UTC" {
		t.Errorf("Expected first zone to be UTC, got %s", zones[0].name)
	}

	if zones[1].name != "America/New_York" {
		t.Errorf("Expected second zone to be America/New_York, got %s", zones[1].name)
	}
}

// TestProcessHighlightFlag tests the processHighlightFlag function
func TestProcessHighlightFlag(t *testing.T) {
	zones := timezoneDetails{
		{name: "America/New_York", offset: -5},
		{name: "Europe/London", offset: 0},
	}

	tests := []struct {
		name          string
		highlightVal  string
		flagChanged   bool
		expectedHour  int
		expectError   bool
		errorContains string
	}{
		{
			name:         "flag not changed",
			highlightVal: "",
			flagChanged:  false,
			expectedHour: -1,
			expectError:  false,
		},
		{
			name:         "valid highlight with offset",
			highlightVal: "15+0",
			flagChanged:  true,
			expectedHour: 15,
			expectError:  false,
		},
		{
			name:          "invalid highlight format",
			highlightVal:  "oops",
			flagChanged:   true,
			expectError:   true,
			errorContains: "invalid highlight specification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore original highlight value
			originalHighlight := highlight
			t.Cleanup(func() { highlight = originalHighlight })

			highlight = tt.highlightVal

			// Create a mock command for testing
			cmd := &cobra.Command{}
			cmd.Flags().String("highlight", "", "test flag")
			if tt.flagChanged {
				if err := cmd.Flags().Set("highlight", tt.highlightVal); err != nil {
					t.Fatalf("failed to set highlight flag: %v", err)
				}
			}

			hour, err := processHighlightFlag(cmd, zones)

			assertError(t, err, tt.expectError, tt.errorContains)
			if tt.expectError {
				return
			}

			assertEqual(t, hour, tt.expectedHour, "Expected hour %d, got %d", tt.expectedHour, hour)
		})
	}
}

// TestBindFlags tests the bindFlags function
func TestBindFlags(t *testing.T) {
	// Create a temporary directory for test config
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	configDir := filepath.Join(tempDir, ".config")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Create a test command with flags
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("color", false, "color flag")
	cmd.Flags().String("timezone", "", "timezone flag")

	// Create a viper instance and set some values
	testViper := viper.New()
	testViper.Set("color", true)
	testViper.Set("timezone", "America/New_York")

	// Bind the flags
	bindFlags(cmd, testViper)

	// Check that the color flag was set
	colorFlag := cmd.Flags().Lookup("color")
	if colorFlag == nil {
		t.Fatal("color flag not found")
	}

	colorVal, err := cmd.Flags().GetBool("color")
	if err != nil {
		t.Errorf("Failed to get color flag: %v", err)
	}
	if colorVal != true {
		t.Errorf("Expected color flag to be true, got %v", colorVal)
	}
}

// TestGetHoursWithHalfHourOffset tests getHours with timezones that have 30-minute offsets
func TestGetHoursWithHalfHourOffset(t *testing.T) {
	// Asia/Kolkata (India) has a +5:30 offset
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Skipf("Failed to load Asia/Kolkata location: %v", err)
	}

	date := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	hours := getHours(date, loc)

	if len(hours) != 24 {
		t.Errorf("Expected 24 hours, got %d", len(hours))
	}

	// Verify all hours are in the correct location
	for i, h := range hours {
		loc := h.Location()
		if loc.String() != "Asia/Kolkata" {
			t.Errorf("Hour %d has wrong location: expected Asia/Kolkata, got %s", i, loc.String())
		}
	}
}

// TestFormatHoursEdgeCases tests formatHours with various edge cases
func TestFormatHoursEdgeCases(t *testing.T) {
	tests := []struct {
		name              string
		hours             []int
		twelveHourEnabled bool
		expected          func(timezoneDetail) []string
	}{
		{
			name:              "midnight in 12-hour format",
			hours:             []int{0, 1, 2},
			twelveHourEnabled: true,
			expected: func(zone timezoneDetail) []string {
				return []string{zone.currentTime.Format("Mon"), " 1\nam", " 2\nam"}
			},
		},
		{
			name:              "noon in 12-hour format",
			hours:             []int{11, 12, 13},
			twelveHourEnabled: true,
			expected: func(timezoneDetail) []string {
				return []string{"11\nam", "12\nam", " 1\npm"}
			},
		},
		{
			name:              "23:00 in 12-hour format",
			hours:             []int{22, 23},
			twelveHourEnabled: true,
			expected: func(timezoneDetail) []string {
				return []string{"10\npm", "11\npm"}
			},
		},
		{
			name:              "24-hour format",
			hours:             []int{0, 12, 23},
			twelveHourEnabled: false,
			expected: func(zone timezoneDetail) []string {
				return []string{zone.currentTime.Format("Mon"), "12", "23"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zone := timezoneDetail{
				name:        "Test/Zone",
				currentTime: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
				hours:       tt.hours,
			}

			result := formatHours(zone, tt.twelveHourEnabled)
			expected := tt.expected(zone)
			if !slices.Equal(toStrings(result), expected) {
				t.Errorf("Expected %v, got %v", expected, toStrings(result))
			}
		})
	}
}

// TestDeduplicateSliceOrder tests that deduplicateSlice maintains correct order
func TestDeduplicateSliceOrder(t *testing.T) {
	input := []string{"first", "second", "first", "third", "second", "fourth"}
	expected := []string{"first", "third", "second", "fourth"}

	result := deduplicateSlice(input)

	if !slices.Equal(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}
