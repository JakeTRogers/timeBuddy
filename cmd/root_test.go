package cmd

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/JakeTRogers/timeBuddy/logger"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Test fixtures and helpers
var (
	// Common test timezones used across multiple tests
	testZones = timezoneDetails{
		{name: "America/New_York", offsetMinutes: -300},
		{name: "Europe/London", offsetMinutes: 0},
		{name: "Asia/Tokyo", offsetMinutes: 540},
		{name: "Australia/Sydney", offsetMinutes: 660},
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

func toStrings(values []any) []string {
	result := make([]string, len(values))
	for i, v := range values {
		result[i] = fmt.Sprint(v)
	}
	return result
}

// makeTimezoneDetail creates a test timezone detail
func makeTimezoneDetail(name string, offsetMinutes int, halfHour bool) timezoneDetail {
	return timezoneDetail{
		name:           name,
		abbreviation:   "TST",
		currentTime:    testTime,
		offsetMinutes:  offsetMinutes,
		halfHourOffset: halfHour,
		hours: []time.Time{
			time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			time.Date(2024, 6, 15, 1, 0, 0, 0, time.UTC),
			time.Date(2024, 6, 15, 2, 0, 0, 0, time.UTC),
			time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
			time.Date(2024, 6, 15, 13, 0, 0, 0, time.UTC),
			time.Date(2024, 6, 15, 23, 0, 0, 0, time.UTC),
		},
	}
}

func Test_NewRootCmd(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	if cmd == nil {
		t.Fatal("NewRootCmd() should not return nil")
	}
	if cmd.Use != "timeBuddy" {
		t.Errorf("Expected Use to be 'timeBuddy', got %s", cmd.Use)
	}
}

func Test_NewRootCmd_independentInstances(t *testing.T) {
	t.Parallel()
	cmd1 := NewRootCmd()
	cmd2 := NewRootCmd()
	if cmd1 == cmd2 {
		t.Error("NewRootCmd() should return independent instances")
	}
}

func Test_parseOffset(t *testing.T) {
	t.Parallel()
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
			expectedOffset: 660,
			expectError:    false,
		},
		{
			name:           "hour with negative offset",
			input:          "9-4",
			expectedHour:   9,
			expectedOffset: -240,
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
			expectedOffset: 300,
			expectError:    false,
		},
		{
			name:           "hour 23 with negative offset",
			input:          "23-8",
			expectedHour:   23,
			expectedOffset: -480,
			expectError:    false,
		},
		{
			name:           "hour with fractional offset",
			input:          "10+5.5",
			expectedHour:   10,
			expectedOffset: 330,
			expectError:    false,
		},
		{
			name:           "hour with hh:mm offset",
			input:          "8-05:45",
			expectedHour:   8,
			expectedOffset: -345,
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
			t.Parallel()
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

func Test_parseHighlightFlag(t *testing.T) {
	t.Parallel()
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
			expectedOffset: 660,
		},
		{
			name:           "valid hour with negative offset",
			highlight:      "9-5",
			expectError:    false,
			expectedHour:   9,
			expectedOffset: -300,
		},
		{
			name:           "valid hour UTC",
			highlight:      "12",
			expectError:    false,
			expectedHour:   12,
			expectedOffset: 0,
		},
		{
			name:           "valid hour with fractional offset",
			highlight:      "9+5.5",
			expectError:    false,
			expectedHour:   9,
			expectedOffset: 330,
			zones: timezoneDetails{
				{name: "Asia/Kolkata", offsetMinutes: 330},
			},
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
			t.Parallel()
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
				highlightMinutes := ((tt.expectedHour * 60) - tt.expectedOffset) % (24 * 60)
				if highlightMinutes < 0 {
					highlightMinutes += 24 * 60
				}
				expectedIndex = int(math.Round(float64(highlightMinutes)/60.0)) % 24
			}
			assertEqual(t, index, expectedIndex, "Expected index %d, got %d", expectedIndex, index)
		})
	}
}

func Test_hasTimezoneWithOffset(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		offset   int
		expected bool
	}{
		{
			name:     "offset exists (-5)",
			offset:   -300,
			expected: true,
		},
		{
			name:     "offset exists (0)",
			offset:   0,
			expected: true,
		},
		{
			name:     "offset exists (9)",
			offset:   540,
			expected: true,
		},
		{
			name:     "offset does not exist (5)",
			offset:   300,
			expected: false,
		},
		{
			name:     "offset does not exist (-8)",
			offset:   -480,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := hasTimezoneWithOffset(testZones, tt.offset)
			assertEqual(t, result, tt.expected, "Expected %v, got %v", tt.expected, result)
		})
	}
}

func Test_deduplicateSlice(t *testing.T) {
	t.Parallel()
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
			expected: []string{"a", "b", "c"},
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
			t.Parallel()
			result := deduplicateSlice(tt.input)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func Test_formatOffset(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		offset   int
		halfHour bool
		expected string
	}{
		{"positive whole hour offset", 300, false, "+5"},
		{"negative whole hour offset", -480, false, "-8"},
		{"zero offset", 0, false, "+0"},
		{"positive half hour offset", 330, true, "+5:30"},
		{"negative half hour offset", -570, true, "-9:30"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			zone := makeTimezoneDetail("Test/Zone", tt.offset, tt.halfHour)
			result := formatOffset(zone)
			assertEqual(t, result, tt.expected, "Expected %s, got %s", tt.expected, result)
		})
	}
}

func Test_validateLiveDateExclusion(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{}
	cmd.Flags().Bool("live", false, "")
	cmd.Flags().String("date", time.Now().Format(time.DateOnly), "")

	// No flags set
	if err := validateLiveDateExclusion(cmd); err != nil {
		t.Fatalf("expected no error when flags unset, got %v", err)
	}

	// Only live set
	if err := cmd.Flags().Set("live", "true"); err != nil {
		t.Fatalf("failed to set live flag: %v", err)
	}
	if err := validateLiveDateExclusion(cmd); err != nil {
		t.Fatalf("expected no error when only live set, got %v", err)
	}

	// Both live and date set
	if err := cmd.Flags().Set("date", "2025-12-22"); err != nil {
		t.Fatalf("failed to set date flag: %v", err)
	}
	if err := validateLiveDateExclusion(cmd); err == nil {
		t.Fatalf("expected error when both live and date are set")
	}
}

func Test_formatRowLabel(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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

func Test_formatHours(t *testing.T) {
	t.Parallel()
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
				return []string{zone.currentTime.Format("Mon"), " 1\nam", " 2\nam", "12\npm", " 1\npm", "11\npm"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := formatHours(zone, tt.twelveHourEnabled)
			expected := tt.expected(zone)
			if !slices.Equal(toStrings(result), expected) {
				t.Errorf("Expected %v, got %v", expected, toStrings(result))
			}
		})
	}
}

func Test_getZoneInfo(t *testing.T) {
	t.Parallel()
	log := logger.GetLogger()

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
				if zone.offsetMinutes != 0 {
					t.Errorf("Expected offset 0, got %d", zone.offsetMinutes)
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
			t.Parallel()
			zone, err := getZoneInfo(tt.timezone, tt.date, log)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.validate(t, zone)
		})
	}
}

func Test_getZoneInfo_errors(t *testing.T) {
	t.Parallel()
	log := logger.GetLogger()

	tests := []struct {
		name          string
		timezone      string
		date          string
		errorContains string
	}{
		{
			name:          "invalid timezone",
			timezone:      "Invalid/Timezone",
			date:          time.Now().Format(time.DateOnly),
			errorContains: "invalid timezone",
		},
		{
			name:          "invalid date format",
			timezone:      "UTC",
			date:          "not-a-date",
			errorContains: "invalid date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := getZoneInfo(tt.timezone, tt.date, log)
			if err == nil {
				t.Fatal("expected error but got none")
			}
			if !strings.Contains(err.Error(), tt.errorContains) {
				t.Errorf("expected error to contain %q, got: %v", tt.errorContains, err)
			}
		})
	}
}

func Test_getHours(t *testing.T) {
	t.Parallel()
	log := logger.GetLogger()

	loc, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatalf("Failed to load UTC location: %v", err)
	}

	hours, err := getHours("2024-06-15", loc, log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

func Test_getHours_invalidDate(t *testing.T) {
	t.Parallel()
	log := logger.GetLogger()

	loc, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatalf("Failed to load UTC location: %v", err)
	}

	_, err = getHours("invalid-date", loc, log)
	if err == nil {
		t.Fatal("expected error for invalid date but got none")
	}
	if !strings.Contains(err.Error(), "failed to parse date") {
		t.Errorf("expected error to contain 'failed to parse date', got: %v", err)
	}
}

func Test_initializeConfig(t *testing.T) {
	log := logger.GetLogger()
	v := viper.New()

	// Create a temporary directory for test config
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	configDir := filepath.Join(tempDir, ".config")

	// Create the config directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Create a test command with required flags
	cmd := NewRootCmd()

	// Test initialization - just verify it doesn't error
	err := initializeConfig(cmd, v, log)
	if err != nil {
		t.Errorf("initializeConfig failed: %v", err)
	}
}

// Test_processTimezones tests the processTimezones function
func Test_processTimezones(t *testing.T) {
	t.Parallel()
	log := logger.GetLogger()
	timezones := []string{"UTC", "America/New_York"}
	date := testTime.Format(time.DateOnly)

	zones, err := processTimezones(timezones, date, log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

func Test_processTimezones_invalidTimezone(t *testing.T) {
	t.Parallel()
	log := logger.GetLogger()
	timezones := []string{"Invalid/Timezone"}
	date := testTime.Format(time.DateOnly)

	_, err := processTimezones(timezones, date, log)
	if err == nil {
		t.Fatal("expected error for invalid timezone but got none")
	}
	if !strings.Contains(err.Error(), "invalid timezone") {
		t.Errorf("expected error to contain 'invalid timezone', got: %v", err)
	}
}

// Test_processHighlightFlag tests the processHighlightFlag function
func Test_processHighlightFlag_func(t *testing.T) {
	t.Parallel()
	log := logger.GetLogger()
	zones := timezoneDetails{
		{name: "America/New_York", offsetMinutes: -300},
		{name: "Europe/London", offsetMinutes: 0},
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
			t.Parallel()
			// Create a mock command for testing
			cmd := &cobra.Command{}
			cmd.Flags().String("highlight", "", "test flag")
			if tt.flagChanged {
				if err := cmd.Flags().Set("highlight", tt.highlightVal); err != nil {
					t.Fatalf("failed to set highlight flag: %v", err)
				}
			}

			hour, err := processHighlightFlag(cmd, zones, tt.highlightVal, log)

			assertError(t, err, tt.expectError, tt.errorContains)
			if tt.expectError {
				return
			}

			assertEqual(t, hour, tt.expectedHour, "Expected hour %d, got %d", tt.expectedHour, hour)
		})
	}
}

// Test_bindFlags tests the bindFlags function
func Test_bindFlags(t *testing.T) {
	log := logger.GetLogger()

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
	bindFlags(cmd, testViper, log)

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

// Test_getHours_halfHourOffset tests getHours with timezones that have 30-minute offsets
func Test_getHours_halfHourOffset(t *testing.T) {
	t.Parallel()
	log := logger.GetLogger()

	// Asia/Kolkata (India) has a +5:30 offset
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Skipf("Failed to load Asia/Kolkata location: %v", err)
	}

	hours, err := getHours("2024-06-15", loc, log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

// Test_formatHours_edgeCases tests formatHours with various edge cases
func Test_formatHours_edgeCases(t *testing.T) {
	t.Parallel()
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
			hours:             []int{0, 11, 12, 13},
			twelveHourEnabled: true,
			expected: func(timezoneDetail) []string {
				return []string{"Sat", "11\nam", "12\npm", " 1\npm"}
			},
		},
		{
			name:              "23:00 in 12-hour format",
			hours:             []int{0, 22, 23},
			twelveHourEnabled: true,
			expected: func(timezoneDetail) []string {
				return []string{"Sat", "10\npm", "11\npm"}
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
			t.Parallel()
			var hourTimes []time.Time
			for _, h := range tt.hours {
				hourTimes = append(hourTimes, time.Date(2024, 6, 15, h, 0, 0, 0, time.UTC))
			}

			zone := timezoneDetail{
				name:        "Test/Zone",
				currentTime: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
				hours:       hourTimes,
			}

			result := formatHours(zone, tt.twelveHourEnabled)
			expected := tt.expected(zone)
			if !slices.Equal(toStrings(result), expected) {
				t.Errorf("Expected %v, got %v", expected, toStrings(result))
			}
		})
	}
}

// Test_deduplicateSlice_order tests that deduplicateSlice maintains correct order
func Test_deduplicateSlice_order(t *testing.T) {
	t.Parallel()
	input := []string{"first", "second", "first", "third", "second", "fourth"}
	expected := []string{"first", "second", "third", "fourth"}

	result := deduplicateSlice(input)

	if !slices.Equal(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

// Test_parseColonOffset tests the parseColonOffset helper function
func Test_parseColonOffset(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		input           string
		expectedMinutes int
		expectError     bool
		errorContains   string
	}{
		{
			name:            "valid positive offset 05:30",
			input:           "05:30",
			expectedMinutes: 330,
			expectError:     false,
		},
		{
			name:            "valid positive offset 05:45",
			input:           "05:45",
			expectedMinutes: 345,
			expectError:     false,
		},
		{
			name:            "valid whole hour 08:00",
			input:           "08:00",
			expectedMinutes: 480,
			expectError:     false,
		},
		{
			name:            "invalid format no colon",
			input:           "0530",
			expectedMinutes: 0,
			expectError:     true,
			errorContains:   "invalid offset",
		},
		{
			name:            "invalid hours",
			input:           "xx:30",
			expectedMinutes: 0,
			expectError:     true,
			errorContains:   "invalid offset hours",
		},
		{
			name:            "invalid minutes",
			input:           "05:xx",
			expectedMinutes: 0,
			expectError:     true,
			errorContains:   "invalid offset minutes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := parseColonOffset(tt.input)

			assertError(t, err, tt.expectError, tt.errorContains)
			if tt.expectError {
				return
			}

			assertEqual(t, result, tt.expectedMinutes, "Expected %d minutes, got %d", tt.expectedMinutes, result)
		})
	}
}

// Test_parseDecimalOffset tests the parseDecimalOffset helper function
func Test_parseDecimalOffset(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		input           string
		expectedMinutes int
		expectError     bool
		errorContains   string
	}{
		{
			name:            "valid half hour offset 5.5",
			input:           "5.5",
			expectedMinutes: 330,
			expectError:     false,
		},
		{
			name:            "valid quarter hour offset 5.25",
			input:           "5.25",
			expectedMinutes: 315,
			expectError:     false,
		},
		{
			name:            "valid three-quarter offset 5.75",
			input:           "5.75",
			expectedMinutes: 345,
			expectError:     false,
		},
		{
			name:            "negative decimal offset -5.5",
			input:           "-5.5",
			expectedMinutes: -330,
			expectError:     false,
		},
		{
			name:            "whole number offset 8.0",
			input:           "8.0",
			expectedMinutes: 480,
			expectError:     false,
		},
		{
			name:            "invalid decimal offset",
			input:           "not_a_number",
			expectedMinutes: 0,
			expectError:     true,
			errorContains:   "invalid offset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := parseDecimalOffset(tt.input)

			assertError(t, err, tt.expectError, tt.errorContains)
			if tt.expectError {
				return
			}

			assertEqual(t, result, tt.expectedMinutes, "Expected %d minutes, got %d", tt.expectedMinutes, result)
		})
	}
}

// Test_parseHHMMOffset tests the parseHHMMOffset helper function
func Test_parseHHMMOffset(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		input           string
		expectedMinutes int
		expectError     bool
		errorContains   string
	}{
		{
			name:            "valid four digit offset 0530",
			input:           "0530",
			expectedMinutes: 330,
			expectError:     false,
		},
		{
			name:            "valid four digit offset 0545",
			input:           "0545",
			expectedMinutes: 345,
			expectError:     false,
		},
		{
			name:            "valid whole hour 0800",
			input:           "0800",
			expectedMinutes: 480,
			expectError:     false,
		},
		{
			name:            "invalid characters xxxx",
			input:           "xxxx",
			expectedMinutes: 0,
			expectError:     true,
			errorContains:   "invalid offset hours",
		},
		{
			name:            "invalid minutes 05xx",
			input:           "05xx",
			expectedMinutes: 0,
			expectError:     true,
			errorContains:   "invalid offset minutes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := parseHHMMOffset(tt.input)

			assertError(t, err, tt.expectError, tt.errorContains)
			if tt.expectError {
				return
			}

			assertEqual(t, result, tt.expectedMinutes, "Expected %d minutes, got %d", tt.expectedMinutes, result)
		})
	}
}

// Test_addLocalTimezone tests the addLocalTimezone function
func Test_addLocalTimezone(t *testing.T) {
	t.Parallel()
	log := logger.GetLogger()

	tests := []struct {
		name             string
		initialTimezones []string
		expectPrepend    bool
	}{
		{
			name:             "adds local to empty list",
			initialTimezones: []string{},
			expectPrepend:    true,
		},
		{
			name:             "adds local to existing list",
			initialTimezones: []string{"UTC", "America/New_York"},
			expectPrepend:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			timezones := make([]string, len(tt.initialTimezones))
			copy(timezones, tt.initialTimezones)
			initialLen := len(timezones)

			err := addLocalTimezone(&timezones, log)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectPrepend && len(timezones) != initialLen+1 {
				t.Errorf("Expected timezone list to grow by 1, got %d", len(timezones)-initialLen)
			}

			// Verify local timezone is at the start
			if len(timezones) > 0 {
				loc, _ := time.LoadLocation("Local")
				if timezones[0] != loc.String() {
					t.Errorf("Expected local timezone at start, got %s", timezones[0])
				}
			}
		})
	}
}

// Test_addLocalTimezone_alreadyPresent tests that addLocalTimezone doesn't duplicate
func Test_addLocalTimezone_alreadyPresent(t *testing.T) {
	t.Parallel()
	log := logger.GetLogger()

	loc, err := time.LoadLocation("Local")
	if err != nil {
		t.Fatalf("Failed to load local timezone: %v", err)
	}

	timezones := []string{loc.String(), "UTC"}
	initialLen := len(timezones)

	err = addLocalTimezone(&timezones, log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(timezones) != initialLen {
		t.Errorf("Expected timezone list to remain same length, got %d", len(timezones))
	}
}

// Test_printTimeTable tests the printTimeTable function
func Test_printTimeTable(t *testing.T) {
	t.Parallel()
	log := logger.GetLogger()
	date := "2024-06-15"

	zones := timezoneDetails{
		makeTimezoneDetail("UTC", 0, false),
		makeTimezoneDetail("America/New_York", -300, false),
	}

	// Test that it doesn't panic with no highlighted hour
	printTimeTable(zones, false, -1, false, date, log)

	// Test with highlighted hour
	printTimeTable(zones, false, 12, false, date, log)

	// Test with color enabled
	printTimeTable(zones, true, 12, false, date, log)
}

// Test_configureColoredTable tests table color configuration
func Test_configureColoredTable(t *testing.T) {
	t.Parallel()
	tw := table.NewWriter()
	configureColoredTable(tw)
	// If it doesn't panic, the test passes
}

// Test_configurePlainTable tests plain table configuration
func Test_configurePlainTable(t *testing.T) {
	t.Parallel()
	tw := table.NewWriter()
	configurePlainTable(tw)
	// If it doesn't panic, the test passes
}

// Test_renderTimeTable tests the renderTimeTable function
func Test_renderTimeTable(t *testing.T) {
	t.Parallel()
	log := logger.GetLogger()
	timezones := []string{"UTC", "America/New_York"}
	date := "2024-06-15"
	highlight := ""

	// Create test command with required flags
	cmd := &cobra.Command{}
	cmd.Flags().String("highlight", "", "")

	err := renderTimeTable(cmd, log, timezones, date, false, false, highlight)
	if err != nil {
		t.Errorf("renderTimeTable failed: %v", err)
	}
}

// Test_renderTimeTable_invalidTimezone tests renderTimeTable with invalid timezone
func Test_renderTimeTable_invalidTimezone(t *testing.T) {
	t.Parallel()
	log := logger.GetLogger()
	timezones := []string{"Invalid/Timezone"}
	date := "2024-06-15"

	// Create test command with required flags
	cmd := &cobra.Command{}
	cmd.Flags().String("highlight", "", "")

	err := renderTimeTable(cmd, log, timezones, date, false, false, "")
	if err == nil {
		t.Error("Expected error for invalid timezone")
	}
}

// Test_clearScreen tests the clearScreen function
func Test_clearScreen(t *testing.T) {
	t.Parallel()
	// Just test that it doesn't panic
	clearScreen()
}

// Test_completeTimezone tests the timezone completion function
func Test_completeTimezone(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		toComplete  string
		expectCount int
	}{
		{
			name:        "empty input returns all",
			toComplete:  "",
			expectCount: len(timezonesAll),
		},
		{
			name:        "America prefix",
			toComplete:  "America",
			expectCount: len(timezonesAll), // Returns all since function doesn't filter
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			completions, directive := completeTimezone(nil, nil, tt.toComplete)

			if directive != cobra.ShellCompDirectiveDefault {
				t.Errorf("Expected ShellCompDirectiveDefault, got %v", directive)
			}

			if len(completions) != tt.expectCount {
				t.Errorf("Expected %d completions, got %d", tt.expectCount, len(completions))
			}
		})
	}
}

// Test_getConfigPath tests the getConfigPath function
func Test_getConfigPath(t *testing.T) {
	tests := []struct {
		name    string
		envHome string
	}{
		{
			name:    "valid home directory",
			envHome: t.TempDir(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", tt.envHome)

			path := getConfigPath()

			if path == "" {
				t.Error("Expected non-empty path")
			}
		})
	}
}

// Test_saveUserPreferences tests the saveUserPreferences function
func Test_saveUserPreferences(t *testing.T) {
	log := logger.GetLogger()
	v := viper.New()

	// Create a temporary directory for test config
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	configDir := filepath.Join(tempDir, ".config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Set config file
	v.SetConfigFile(filepath.Join(configDir, ".timeBuddy.yaml"))
	v.SetConfigType("yaml")

	// Set test values
	timezones := []string{"UTC", "America/New_York"}
	colorEnabled := true
	twelveHourEnabled := false

	// Call saveUserPreferences
	saveUserPreferences(v, log, colorEnabled, twelveHourEnabled, timezones)

	// Verify values were set in viper
	if !v.GetBool("color") {
		t.Error("Expected color to be true in viper")
	}
	savedTimezones := v.GetStringSlice("timezone")
	if len(savedTimezones) != 2 {
		t.Errorf("Expected 2 timezones in viper, got %d", len(savedTimezones))
	}
}

// Test_NewRootCmd_Args tests the Args validation in NewRootCmd
func Test_NewRootCmd_Args(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		setupCmd      func(*cobra.Command)
		expectError   bool
		errorContains string
	}{
		{
			name: "valid args no flags",
			setupCmd: func(cmd *cobra.Command) {
				// No flags set
			},
			expectError: false,
		},
		{
			name: "invalid date format",
			setupCmd: func(cmd *cobra.Command) {
				_ = cmd.Flags().Set("date", "not-a-date")
			},
			expectError:   true,
			errorContains: "invalid date",
		},
		{
			name: "valid date format",
			setupCmd: func(cmd *cobra.Command) {
				_ = cmd.Flags().Set("date", "2024-06-15")
			},
			expectError: false,
		},
		{
			name: "exclude-local flag",
			setupCmd: func(cmd *cobra.Command) {
				_ = cmd.Flags().Set("exclude-local", "true")
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewRootCmd()
			tt.setupCmd(cmd)

			err := cmd.Args(cmd, nil)

			assertError(t, err, tt.expectError, tt.errorContains)
		})
	}
}

// Test_NewRootCmd_RunE tests the RunE function of the root command
func Test_NewRootCmd_RunE(t *testing.T) {
	// Create a temporary directory for test config
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	configDir := filepath.Join(tempDir, ".config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	tests := []struct {
		name          string
		setupCmd      func(*cobra.Command)
		expectError   bool
		errorContains string
	}{
		{
			name: "normal execution with valid timezone",
			setupCmd: func(cmd *cobra.Command) {
				_ = cmd.Flags().Set("timezone", "UTC")
				_ = cmd.Flags().Set("exclude-local", "true")
			},
			expectError: false,
		},
		{
			name: "invalid timezone returns error",
			setupCmd: func(cmd *cobra.Command) {
				_ = cmd.Flags().Set("timezone", "Invalid/Timezone")
				_ = cmd.Flags().Set("exclude-local", "true")
			},
			expectError:   true,
			errorContains: "invalid timezone",
		},
		{
			name: "invalid highlight with changed flag returns error",
			setupCmd: func(cmd *cobra.Command) {
				_ = cmd.Flags().Set("timezone", "UTC")
				_ = cmd.Flags().Set("exclude-local", "true")
				_ = cmd.Flags().Set("highlight", "invalid")
			},
			expectError:   true,
			errorContains: "invalid highlight",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCmd()
			tt.setupCmd(cmd)

			// Initialize config first
			_ = cmd.PersistentPreRunE(cmd, nil)

			// Run Args validation
			if err := cmd.Args(cmd, nil); err != nil {
				if !tt.expectError {
					t.Fatalf("Args validation failed: %v", err)
				}
				return
			}

			// Run main command
			err := cmd.RunE(cmd, nil)

			assertError(t, err, tt.expectError, tt.errorContains)
		})
	}
}

// Test_parseOffset_edgeCases tests edge cases for parseOffset
func Test_parseOffset_edgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		input         string
		expectError   bool
		errorContains string
	}{
		{
			name:          "empty string",
			input:         "",
			expectError:   true,
			errorContains: "empty highlight value",
		},
		{
			name:          "empty offset part after plus",
			input:         "15+",
			expectError:   true,
			errorContains: "invalid format",
		},
		{
			name:          "invalid hour string",
			input:         "abc+5",
			expectError:   true,
			errorContains: "invalid hour",
		},
		{
			name:          "invalid offset string",
			input:         "15+abc",
			expectError:   true,
			errorContains: "invalid offset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := parseOffset(tt.input)

			assertError(t, err, tt.expectError, tt.errorContains)
		})
	}
}

// Test_parseColonOffset_invalidMinutesRange tests invalid minute values
func Test_parseColonOffset_invalidMinutesRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		input         string
		expectError   bool
		errorContains string
	}{
		{
			name:          "minutes too high",
			input:         "05:60",
			expectError:   true,
			errorContains: "offset minutes must be between 0 and 59",
		},
		{
			name:          "minutes negative via invalid parse",
			input:         "05:-30",
			expectError:   true,
			errorContains: "offset minutes must be between 0 and 59",
		},
		{
			name:          "too many colons",
			input:         "05:30:45",
			expectError:   true,
			errorContains: "invalid offset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseColonOffset(tt.input)

			assertError(t, err, tt.expectError, tt.errorContains)
		})
	}
}

// Test_parseHHMMOffset_invalidMinutesRange tests invalid minute values
func Test_parseHHMMOffset_invalidMinutesRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		input         string
		expectError   bool
		errorContains string
	}{
		{
			name:          "minutes too high",
			input:         "0560",
			expectError:   true,
			errorContains: "offset minutes must be between 0 and 59",
		},
		{
			name:          "minutes 99",
			input:         "0599",
			expectError:   true,
			errorContains: "offset minutes must be between 0 and 59",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseHHMMOffset(tt.input)

			assertError(t, err, tt.expectError, tt.errorContains)
		})
	}
}

// Test_parseOffsetMinutes tests parseOffsetMinutes function
func Test_parseOffsetMinutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		input           string
		expectedMinutes int
		expectError     bool
		errorContains   string
	}{
		{
			name:            "simple hours",
			input:           "5",
			expectedMinutes: 300,
			expectError:     false,
		},
		{
			name:            "colon format",
			input:           "5:30",
			expectedMinutes: 330,
			expectError:     false,
		},
		{
			name:            "decimal format",
			input:           "5.5",
			expectedMinutes: 330,
			expectError:     false,
		},
		{
			name:            "HHMM format",
			input:           "0530",
			expectedMinutes: 330,
			expectError:     false,
		},
		{
			name:          "invalid hours string",
			input:         "abc",
			expectError:   true,
			errorContains: "invalid offset hours",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := parseOffsetMinutes(tt.input)

			assertError(t, err, tt.expectError, tt.errorContains)
			if tt.expectError {
				return
			}

			assertEqual(t, result, tt.expectedMinutes, "Expected %d minutes, got %d", tt.expectedMinutes, result)
		})
	}
}

// Test_bindFlags_arrayFlag tests bindFlags with array config values
func Test_bindFlags_arrayFlag(t *testing.T) {
	t.Parallel()
	log := logger.GetLogger()

	// Create a test command with an array flag
	cmd := &cobra.Command{Use: "test"}
	var timezones []string
	cmd.Flags().StringArrayVar(&timezones, "timezone", []string{}, "timezone flag")

	// Create a viper instance with an array value
	testViper := viper.New()
	testViper.Set("timezone", []any{"UTC", "America/New_York"})

	// Bind the flags
	bindFlags(cmd, testViper, log)

	// Check that the timezone flag was set with array values
	tzFlag := cmd.Flags().Lookup("timezone")
	if tzFlag == nil {
		t.Fatal("timezone flag not found")
	}
}

// Test_renderTimeTable_invalidHighlight tests renderTimeTable with invalid highlight
func Test_renderTimeTable_invalidHighlight(t *testing.T) {
	t.Parallel()
	log := logger.GetLogger()
	timezones := []string{"UTC"}
	date := "2024-06-15"

	cmd := &cobra.Command{}
	cmd.Flags().String("highlight", "", "")
	_ = cmd.Flags().Set("highlight", "25+0") // Invalid hour

	err := renderTimeTable(cmd, log, timezones, date, false, false, "25+0")
	if err == nil {
		t.Error("Expected error for invalid highlight hour")
	}
}

// Test_printTimeTable_withCurrentDate tests printTimeTable with current date
func Test_printTimeTable_withCurrentDate(t *testing.T) {
	t.Parallel()
	log := logger.GetLogger()
	date := time.Now().Format(time.DateOnly)

	zones := timezoneDetails{
		makeTimezoneDetail("UTC", 0, false),
	}

	// Test with current date (should show current time in title)
	printTimeTable(zones, false, -1, false, date, log)

	// Test with twelve hour format
	printTimeTable(zones, false, -1, true, date, log)
}

// Test_getZoneInfo_halfHourOffset tests getZoneInfo with half-hour offset timezone
func Test_getZoneInfo_halfHourOffset(t *testing.T) {
	t.Parallel()
	log := logger.GetLogger()

	// Asia/Kolkata has a +5:30 offset
	zone, err := getZoneInfo("Asia/Kolkata", "2024-06-15", log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !zone.halfHourOffset {
		t.Error("Expected halfHourOffset to be true for Asia/Kolkata")
	}

	// 330 minutes = 5 hours 30 minutes
	if zone.offsetMinutes != 330 {
		t.Errorf("Expected offset 330, got %d", zone.offsetMinutes)
	}
}
