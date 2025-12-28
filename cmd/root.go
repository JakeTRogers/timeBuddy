// Copyright © 2025 Jake Rogers <code@supportoss.org>

// Package cmd provides CLI commands for timeBuddy using Cobra.
package cmd

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/JakeTRogers/timeBuddy/logger"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// replaceHyphenWithCamelCase controls viper config name conversion.
// Set to true if config file uses camelCase instead of hyphenated keys.
const replaceHyphenWithCamelCase = false

// Package-level state for CLI flags and configuration.
// These are bound to CLI flags and persisted via Viper.
var (
	colorEnabled      bool
	highlight         string
	liveMode          bool
	liveInterval      int
	twelveHourEnabled bool
	date              string
	timezones         []string
	v                 = viper.New()
	log               = logger.GetLogger()
)

// timezoneDetail holds timezone information for display.
type timezoneDetail struct {
	name           string
	abbreviation   string
	currentTime    time.Time
	offsetMinutes  int
	halfHourOffset bool
	hours          []time.Time
}

// timezoneDetails is a slice of timezoneDetail for table rendering.
type timezoneDetails []timezoneDetail

// initializeConfig initializes Viper configuration for the root command.
// It sets up the config file path, reads existing config, creates a new one
// if none exists, and binds command flags to configuration values.
func initializeConfig(cmd *cobra.Command) error {
	verboseCount, _ := cmd.Flags().GetCount("verbose")
	logger.SetLogLevel(verboseCount)

	configName := ".timeBuddy"
	configType := "yaml"
	v.SetConfigName(configName)
	v.SetConfigType(configType)

	configPath := getConfigPath()
	if err := os.MkdirAll(configPath, 0o755); err != nil {
		return fmt.Errorf("unable to create config directory: %w", err)
	}

	configFile := filepath.Join(configPath, configName+"."+configType)
	log.Debug().Str("configPath", configPath).Str("configFile", configFile).Send()

	v.AddConfigPath(configPath)
	v.SetConfigFile(configFile)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if writeErr := v.SafeWriteConfigAs(configFile); writeErr != nil {
				log.Error().Err(writeErr).Msg("failed to create config file")
			} else {
				log.Info().Str("configFile", configFile).Msg("new config file created")
			}
		} else {
			log.Error().Str("viper", err.Error()).Send()
		}
	}

	v.SetEnvPrefix("TIMEBUDDY")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	bindFlags(cmd, v)

	return nil
}

// getConfigPath returns the platform-appropriate config directory path.
func getConfigPath() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("APPDATA")
	}
	return filepath.Join(os.Getenv("HOME"), ".config")
}

// bindFlags binds command flags to Viper configuration values.
// It applies config file values to flags that haven't been explicitly set.
func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		configName := f.Name
		if replaceHyphenWithCamelCase {
			configName = strings.ReplaceAll(f.Name, "-", "")
		}

		log.Debug().Str("flag", f.Name).Str("configName", configName).Msg("binding flag to viper config")

		if f.Changed || !v.IsSet(configName) {
			return
		}

		val := v.Get(configName)
		if arr, ok := val.([]interface{}); ok {
			for _, item := range arr {
				if err := cmd.Flags().Set(f.Name, fmt.Sprintf("%v", item)); err != nil {
					log.Error().Str("viper", err.Error()).Send()
				}
			}
			return
		}

		if err := cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val)); err != nil {
			log.Error().Str("viper", err.Error()).Send()
		}
	})
}

// getZoneInfo returns timezone details for the given timezone and date.
// It validates the timezone and date, then computes offset and hours.
func getZoneInfo(timezone string, date string) (timezoneDetail, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return timezoneDetail{}, fmt.Errorf("invalid timezone %q: %w", timezone, err)
	}

	parsedDate, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return timezoneDetail{}, fmt.Errorf("invalid date %q: %w", date, err)
	}

	var zone timezoneDetail
	zone.name = timezone

	// Use current time if date matches today, otherwise use midnight
	if date == time.Now().Format(time.DateOnly) {
		zone.currentTime = time.Now().In(loc)
	} else {
		zone.currentTime = time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), 0, 0, 0, 0, loc)
	}

	var offsetSeconds int
	zone.abbreviation, offsetSeconds = zone.currentTime.In(loc).Zone()
	zone.offsetMinutes = offsetSeconds / 60
	zone.halfHourOffset = zone.offsetMinutes%60 != 0

	log.Debug().
		Str("timezone", zone.name).
		Str("abbreviation", zone.abbreviation).
		Str("currentTime", zone.currentTime.String()).
		Int("offsetMinutes", zone.offsetMinutes).
		Send()

	hours, err := getHours(date, loc)
	if err != nil {
		return timezoneDetail{}, fmt.Errorf("failed to get hours for timezone %q: %w", timezone, err)
	}
	zone.hours = hours

	return zone, nil
}

// getHours returns 24 hours for the given date in the specified timezone.
// Each hour represents the time at that UTC hour converted to the location.
func getHours(date string, location *time.Location) ([]time.Time, error) {
	d, err := time.ParseInLocation(time.DateOnly, date, time.UTC)
	if err != nil {
		return nil, fmt.Errorf("failed to parse date %q: %w", date, err)
	}

	startUTC := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	hours := make([]time.Time, 24)
	for i := range hours {
		hours[i] = startUTC.Add(time.Duration(i) * time.Hour).In(location)
	}

	return hours, nil
}

// formatHours formats the hours for display in the time table.
// When twelveHourEnabled is true, uses 12-hour format with am/pm.
func formatHours(z timezoneDetail, twelveHourEnabled bool) []interface{} {
	hours := make([]interface{}, len(z.hours))
	for i, t := range z.hours {
		hour24 := t.Hour()
		if hour24 == 0 {
			hours[i] = t.Format("Mon")
			continue
		}

		if !twelveHourEnabled {
			hours[i] = fmt.Sprintf("%2d", hour24)
			continue
		}

		var meridiem string
		displayHour := hour24
		switch {
		case hour24 == 12:
			displayHour = 12
			meridiem = "pm"
		case hour24 > 12:
			displayHour = hour24 - 12
			meridiem = "pm"
		default:
			meridiem = "am"
		}
		hours[i] = fmt.Sprintf("%2d\n%s", displayHour, meridiem)
	}
	return hours
}

// formatOffset formats the UTC offset as a string with sign (e.g., "+5", "-8", "+5:30").
func formatOffset(z timezoneDetail) string {
	sign := "+"
	offsetMinutes := z.offsetMinutes
	if offsetMinutes < 0 {
		sign = "-"
		offsetMinutes = -offsetMinutes
	}

	hours := offsetMinutes / 60
	minutes := offsetMinutes % 60

	if minutes == 0 {
		return fmt.Sprintf("%s%d", sign, hours)
	}
	return fmt.Sprintf("%s%d:%02d", sign, hours, minutes)
}

// formatRowLabel creates the label for a timezone row in the table.
// Shows timezone name, abbreviation, and offset. For current date, also shows time.
func formatRowLabel(z timezoneDetail, date, offset string) string {
	if date != time.Now().Format(time.DateOnly) {
		return fmt.Sprintf("%s [%s,%s]", z.name, z.abbreviation, offset)
	}
	return fmt.Sprintf("%s [%s,%s]\n%s", z.name, z.abbreviation, offset, z.currentTime.Format("Monday, Jan 2 3:04PM"))
}

// parseOffset parses a highlight string like "hour+offset" or "hour-offset".
// Returns the hour (0-23), offset in minutes, and any parsing error.
func parseOffset(input string) (hour int, offsetMinutes int, err error) {
	sep := strings.IndexAny(input[1:], "+-")
	if sep != -1 {
		sep++ // account for slicing from index 1
	}

	if sep == -1 {
		hour, err = strconv.Atoi(input)
		return hour, 0, err
	}

	hourStr := input[:sep]
	offsetStr := input[sep+1:]
	if hourStr == "" || offsetStr == "" {
		return 0, 0, fmt.Errorf("invalid format, expected hour±offset")
	}

	hour, err = strconv.Atoi(hourStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid hour: %w", err)
	}

	sign := 1
	if input[sep] == '-' {
		sign = -1
	}

	offsetMinutes, err = parseOffsetMinutes(offsetStr)
	if err != nil {
		return 0, 0, err
	}

	return hour, offsetMinutes * sign, nil
}

// validateLiveDateExclusion ensures --live and --date are not both set.
func validateLiveDateExclusion(cmd *cobra.Command) error {
	if cmd.Flags().Changed("live") && cmd.Flags().Changed("date") {
		return fmt.Errorf("the --live and --date flags are mutually exclusive")
	}
	return nil
}

// parseOffsetMinutes parses an offset string and returns total minutes.
// Supports formats: "11" (hours), "5.5" (decimal hours), "5:30" (HH:MM), "0530" (HHMM).
func parseOffsetMinutes(part string) (int, error) {
	if strings.Contains(part, ":") {
		return parseColonOffset(part)
	}

	if strings.Contains(part, ".") {
		return parseDecimalOffset(part)
	}

	if len(part) == 4 {
		return parseHHMMOffset(part)
	}

	hours, err := strconv.Atoi(part)
	if err != nil {
		return 0, fmt.Errorf("invalid offset hours: %w", err)
	}
	return hours * 60, nil
}

// parseColonOffset parses "HH:MM" format offset.
func parseColonOffset(part string) (int, error) {
	pieces := strings.Split(part, ":")
	if len(pieces) != 2 {
		return 0, fmt.Errorf("invalid offset, expected HH:MM")
	}

	hours, err := strconv.Atoi(pieces[0])
	if err != nil {
		return 0, fmt.Errorf("invalid offset hours: %w", err)
	}

	minutes, err := strconv.Atoi(pieces[1])
	if err != nil {
		return 0, fmt.Errorf("invalid offset minutes: %w", err)
	}

	if minutes < 0 || minutes >= 60 {
		return 0, fmt.Errorf("offset minutes must be between 0 and 59")
	}
	return hours*60 + minutes, nil
}

// parseDecimalOffset parses decimal hour offset (e.g., "5.5" = 5h30m).
func parseDecimalOffset(part string) (int, error) {
	value, err := strconv.ParseFloat(part, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid offset: %w", err)
	}
	return int(math.Round(value * 60)), nil
}

// parseHHMMOffset parses "HHMM" format without colon (e.g., "0530").
func parseHHMMOffset(part string) (int, error) {
	hoursPart := part[:len(part)-2]
	minutesPart := part[len(part)-2:]

	hours, err := strconv.Atoi(hoursPart)
	if err != nil {
		return 0, fmt.Errorf("invalid offset hours: %w", err)
	}

	minutes, err := strconv.Atoi(minutesPart)
	if err != nil {
		return 0, fmt.Errorf("invalid offset minutes: %w", err)
	}

	if minutes < 0 || minutes >= 60 {
		return 0, fmt.Errorf("offset minutes must be between 0 and 59")
	}
	return hours*60 + minutes, nil
}

// parseHighlightFlag parses the highlight flag and returns the UTC hour column index.
// Returns -1 if no highlight is specified.
func parseHighlightFlag(highlight string, zones timezoneDetails) (int, error) {
	if highlight == "" {
		return -1, nil
	}

	hour, offsetMinutes, err := parseOffset(highlight)
	if err != nil {
		return -1, fmt.Errorf("invalid format: %v", err)
	}

	if hour < 0 || hour > 23 {
		return -1, fmt.Errorf("hour must be between 0 and 23")
	}

	// Validate offset exists in configured timezones
	if !hasTimezoneWithOffset(zones, offsetMinutes) {
		return -1, fmt.Errorf("no configured timezone with UTC offset of %+d minutes", offsetMinutes)
	}

	highlightMinutesUTC := ((hour * 60) - offsetMinutes) % (24 * 60)
	if highlightMinutesUTC < 0 {
		highlightMinutesUTC += 24 * 60
	}

	// Round to the nearest hour column
	roundedHour := int(math.Round(float64(highlightMinutesUTC)/60.0)) % 24

	return roundedHour, nil
}

// hasTimezoneWithOffset checks if any configured timezone has the given offset.
func hasTimezoneWithOffset(zones timezoneDetails, offsetMinutes int) bool {
	for _, z := range zones {
		if z.offsetMinutes == offsetMinutes {
			return true
		}
	}
	return false
}

// printTimeTable renders the timezone table to stdout.
// Uses go-pretty for table formatting with optional color styling.
func printTimeTable(zones timezoneDetails, colorEnabled bool, highlightHour int) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	if colorEnabled {
		configureColoredTable(t)
	} else {
		configurePlainTable(t)
	}

	t.Style().Title.Align = text.AlignCenter

	// Set highlight column: explicit highlight overrides current hour
	if highlightHour > -1 {
		t.SetIndexColumn(highlightHour + 2) // +2: first col=timezone, hours start at 0
	} else if date == time.Now().Format(time.DateOnly) {
		t.SetIndexColumn(time.Now().UTC().Hour() + 2)
	}

	// Set title based on whether viewing current date or a specific date
	if date != time.Now().Format(time.DateOnly) {
		d, _ := time.Parse(time.DateOnly, date)
		t.SetTitle("Showing Time For: %s", d.Format("Monday, January 2, 2006 MST"))
	} else {
		t.SetTitle("Current Local Time: %s", time.Now().Format("Monday, January 2, 2006 3:04:05 PM MST"))
	}

	for _, z := range zones {
		hours := formatHours(z, twelveHourEnabled)
		offset := formatOffset(z)
		rowLabel := formatRowLabel(z, date, offset)
		row := append([]interface{}{rowLabel}, hours...)
		t.AppendRow(row)
	}

	t.Render()
}

// configureColoredTable applies colored style to the table.
func configureColoredTable(t table.Writer) {
	t.SetStyle(table.StyleColoredBlackOnBlueWhite)
	t.Style().Title.Colors = text.Colors{text.BgHiBlue, text.FgHiWhite}
	t.Style().Color.IndexColumn = text.Colors{text.BgHiBlue, text.FgHiWhite, text.Bold}
	t.Style().Color.RowAlternate = text.Colors{text.Color(30), text.Color(47)}
}

// configurePlainTable applies non-colored style to the table.
func configurePlainTable(t table.Writer) {
	t.SetStyle(table.StyleRounded)
	t.Style().Options.DoNotColorBordersAndSeparators = true
	t.Style().Options.SeparateColumns = false
	t.Style().Options.SeparateRows = true
	t.Style().Color.IndexColumn = text.Colors{text.FgHiBlue, text.Bold}
}

// deduplicateSlice removes duplicates from a string slice while preserving order.
func deduplicateSlice(s []string) []string {
	result := make([]string, 0, len(s))
	seen := make(map[string]struct{}, len(s))

	for _, v := range s {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	return result
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "timeBuddy",
	Version: "v2.0.1",
	Short:   "CLI version of World Time Buddy",
	Long: `timeBuddy is a Command Line Interface (CLI) tool designed to display the current time across multiple time zones. This
tool is particularly useful for scheduling meetings with participants in various time zones. By default, timeBuddy
includes your local time zone in its output. You can exclude your local time zone using the --exclude-local flag.

timeBuddy saves your most recent time zone selections in a configuration file. This feature ensures that you need to
specify your preferred time zones only once. The order in which you specify the time zones is retained and reflected in
the table output. You can find the configuration file at the following locations:

  - Linux/Mac: $HOME/.config/.timeBuddy.yaml
  - Windows: %APPDATA%\.timeBuddy.yaml

Examples:

  # Display your local time zone or those saved in the config file from your last session:
  $ timeBuddy

  # Display the current time for a selection of time zones:
  $ timeBuddy --timezone America/New_York --timezone Europe/Vilnius --timezone Australia/Sydney

  # Display Time for a specific date and highlight 3pm AEDT(useful for Daylight Saving Time changes):
  $ timeBuddy --date 2023-11-05 --highlight 15+11

  # Exclude your local time zone from the output:
   $ timeBuddy --exclude-local --timezone --timezone Europe/London --timezone Asia/Tokyo

  # Enable colorized table output:
   $ timeBuddy --color

  # Enable live mode to continuously update the display:
   $ timeBuddy --live

  # Enable live mode with a custom refresh interval (every 5 seconds):
   $ timeBuddy --live --interval 5

Learn More:
  To submit feature requests, bugs, or to check for new versions, visit https://github.com/JakeTRogers/timeBuddy`,
	Args:              validateArgs,
	PersistentPreRunE: persistentPreRunE,
	RunE:              runRoot,
}

// validateArgs validates command arguments before execution.
func validateArgs(cmd *cobra.Command, args []string) error {
	if err := validateLiveDateExclusion(cmd); err != nil {
		return err
	}

	if cmd.Flags().Changed("date") {
		if _, err := time.Parse(time.DateOnly, date); err != nil {
			return fmt.Errorf("invalid date %q: %w", date, err)
		}
	}

	if !cmd.Flags().Changed("exclude-local") {
		if err := addLocalTimezone(); err != nil {
			return err
		}
	}

	timezones = deduplicateSlice(timezones)
	return nil
}

// addLocalTimezone adds the local timezone to the timezones list if not present.
func addLocalTimezone() error {
	ltz, err := time.LoadLocation("Local")
	if err != nil {
		return fmt.Errorf("failed to load local timezone: %w", err)
	}

	for _, tz := range timezones {
		if tz == ltz.String() {
			return nil
		}
	}

	timezones = append([]string{ltz.String()}, timezones...)
	return nil
}

// persistentPreRunE initializes configuration before command execution.
func persistentPreRunE(cmd *cobra.Command, args []string) error {
	return initializeConfig(cmd)
}

// runRoot executes the main timeBuddy command logic.
func runRoot(cmd *cobra.Command, args []string) error {
	if wizardMode, _ := cmd.Flags().GetBool("wizard"); wizardMode {
		return handleWizardMode()
	}

	for k, val := range v.AllSettings() {
		log.Debug().Str(k, fmt.Sprintf("%v", val)).Msg("viper")
	}

	saveUserPreferences()

	if liveMode {
		return runLiveMode(cmd)
	}

	zones, err := processTimezones()
	if err != nil {
		return err
	}

	highlightHour, err := processHighlightFlag(cmd, zones)
	if err != nil {
		return fmt.Errorf("invalid highlight specification: %w", err)
	}

	printTimeTable(zones, colorEnabled, highlightHour)
	return nil
}

// handleWizardMode runs the interactive timezone selector.
func handleWizardMode() error {
	selected, err := runWizard()
	if err != nil {
		return fmt.Errorf("wizard failed: %w", err)
	}

	if selected == nil {
		return nil
	}

	timezones = selected
	v.Set("timezone", selected)
	if err := v.WriteConfig(); err != nil {
		log.Error().Err(err).Msg("failed to save config")
	}
	return nil
}

// clearScreen clears the terminal using ANSI escape codes.
func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

// runLiveMode continuously refreshes the time table at the configured interval.
func runLiveMode(cmd *cobra.Command) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(liveInterval) * time.Second)
	defer ticker.Stop()

	if err := renderTimeTable(cmd); err != nil {
		return err
	}

	fmt.Println("\nLive mode active. Press Ctrl+C to exit.")

	for {
		select {
		case <-sigChan:
			fmt.Println("\nExiting live mode...")
			return nil
		case <-ticker.C:
			clearScreen()
			if err := renderTimeTable(cmd); err != nil {
				log.Error().Err(err).Msg("failed to render time table")
			}
			fmt.Println("\nLive mode active. Press Ctrl+C to exit.")
		}
	}
}

// renderTimeTable processes timezones and renders the table.
func renderTimeTable(cmd *cobra.Command) error {
	zones, err := processTimezones()
	if err != nil {
		return err
	}
	highlightHour, err := processHighlightFlag(cmd, zones)
	if err != nil {
		return fmt.Errorf("invalid highlight specification: %w", err)
	}
	printTimeTable(zones, colorEnabled, highlightHour)
	return nil
}

// Execute runs the root command. Called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.SetVersionTemplate(`{{printf "timeBuddy %s\n" .Version}}`)

	// Display flags
	rootCmd.Flags().BoolVarP(&colorEnabled, "color", "c", false, "enable colorized table output. If previously enabled, use --color=false to disable it,")
	rootCmd.Flags().StringVarP(&date, "date", "d", time.Now().Format(time.DateOnly), "``date to use for time conversion. Expects YYYY-MM-DD format. Defaults to current date/time.")
	rootCmd.Flags().StringVarP(&highlight, "highlight", "H", "", "highlight hour column (0-23), optionally with UTC offset (e.g., '15+11' or '9-4')")
	rootCmd.Flags().BoolVarP(&twelveHourEnabled, "twelve-hour", "t", false, "use 12-hour time format instead of 24-hour. If previously enabled, use --twelve-hour=false to disable it.")

	// Live mode flags
	rootCmd.Flags().BoolVarP(&liveMode, "live", "l", false, "enable live mode to continuously refresh the time display (press Ctrl+C to exit)")
	rootCmd.Flags().IntVarP(&liveInterval, "interval", "i", 1, "refresh interval in seconds for live mode")

	// Timezone selection flags
	rootCmd.Flags().BoolP("exclude-local", "x", false, "disable default behavior of including local timezone in output")
	rootCmd.Flags().BoolP("wizard", "w", false, "launch interactive timezone selector wizard")
	rootCmd.Flags().StringArrayVarP(&timezones, "timezone", "z", []string{}, "``timezone to use for time conversion. Accepts timezone name, like America/New_York. Can be used multiple times.")

	// Logging flags
	rootCmd.PersistentFlags().CountP("verbose", "v", "``increase logging verbosity, 1=warn, 2=info, 3=debug, 4=trace")

	// Mutual exclusion
	rootCmd.MarkFlagsMutuallyExclusive("live", "date")

	// Tab completion for timezone flag
	if err := rootCmd.RegisterFlagCompletionFunc("timezone", completeTimezone); err != nil {
		log.Error().Err(err).Send()
	}
}

// completeTimezone provides tab completion for the timezone flag.
func completeTimezone(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return timezonesAll, cobra.ShellCompDirectiveDefault
}

// saveUserPreferences persists current preferences to the config file.
func saveUserPreferences() {
	v.Set("color", colorEnabled)
	v.Set("timezone", timezones)
	v.Set("twelve-hour", twelveHourEnabled)

	if err := v.WriteConfig(); err != nil {
		log.Error().Err(err).Msg("failed to save preferences")
	}
}

// processTimezones collects timezone information for all configured timezones.
func processTimezones() (timezoneDetails, error) {
	zones := make(timezoneDetails, 0, len(timezones))
	for _, tz := range timezones {
		zone, err := getZoneInfo(tz, date)
		if err != nil {
			return nil, fmt.Errorf("failed to process timezone %q: %w", tz, err)
		}
		zones = append(zones, zone)
	}
	return zones, nil
}

// processHighlightFlag parses and validates the highlight flag if provided.
func processHighlightFlag(cmd *cobra.Command, zones timezoneDetails) (int, error) {
	if !cmd.Flags().Changed("highlight") {
		return -1, nil
	}

	highlightHour, err := parseHighlightFlag(highlight, zones)
	if err != nil {
		return -1, fmt.Errorf("invalid highlight specification: %w", err)
	}

	return highlightHour, nil
}
