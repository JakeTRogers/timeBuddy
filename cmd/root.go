/*
Copyright © 2024 Jake Rogers <code@supportoss.org>
*/
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

var (
	colorEnabled               bool
	highlight                  string
	liveMode                   bool
	liveInterval               int
	twelveHourEnabled          bool
	date                       string
	timezones                  []string
	v                          = viper.New()
	l                          = logger.GetLogger()
	replaceHyphenWithCamelCase = false
)

type timezoneDetail = struct {
	name           string
	abbreviation   string
	currentTime    time.Time
	offsetMinutes  int
	halfHourOffset bool
	hours          []time.Time
}

type timezoneDetails = []timezoneDetail

// initializeConfig initializes the configuration for the root command.
// It sets up the configuration file path, reads the config file if it exists,
// creates a new config file if it doesn't exist, and binds command flags to environment variables.
// The function takes a pointer to the root command as a parameter and returns an error.
func initializeConfig(cmd *cobra.Command) error {
	verboseCount, _ := cmd.Flags().GetCount("verbose")
	logger.SetLogLevel(verboseCount)
	configName := ".timeBuddy"
	v.SetConfigName(configName)
	configType := "yaml"
	v.SetConfigType(configType)
	configPath := ""
	if runtime.GOOS == "windows" {
		configPath = os.Getenv("APPDATA")
	} else {
		configPath = filepath.Join(os.Getenv("HOME"), ".config")
	}
	if err := os.MkdirAll(configPath, 0o755); err != nil {
		return fmt.Errorf("unable to create config directory: %w", err)
	}

	configFile := filepath.Join(configPath, configName+"."+configType)
	l.Debug().Str("configPath", configPath).Str("configFile", configFile).Send()
	v.AddConfigPath(configPath)
	v.SetConfigFile(configFile)

	// Attempt to read the config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Create config file if it doesn't exist
			if err := v.SafeWriteConfigAs(configFile); err != nil {
				l.Error().Err(err).Msg("Failed to create config file")
			} else {
				l.Info().Str("configFile", configFile).Msg("New config file created")
			}
		} else {
			// Config file was found but another error was produced
			l.Error().Str("viper", err.Error()).Send()
		}
	}

	// When we bind flags to environment variables expect that the environment variables are prefixed, e.g. a flag like
	// --timezones binds to an environment variable TIMEBUDDY_TIMEZONES. This helps avoid conflicts.
	v.SetEnvPrefix("TIMEBUDDY")

	// Environment variables can't have dashes in them, so bind them to their equivalent keys with underscores
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Bind to environment variables works great for simple config names, but needs help for names with a `-`, we fix
	// those in the bindFlags function
	v.AutomaticEnv()

	// Bind the current command's flags to viper
	bindFlags(cmd, v)

	return nil
}

// bindFlags binds the command flags to the corresponding values in the viper configuration.
// It iterates over each flag, determines the naming convention of the flag in the config file,
// and applies the corresponding value from the viper configuration to the flag if it is not already set.
// If the value is an array, it loops through each element and adds it to the flag.
func bindFlags(cmd *cobra.Command, v *viper.Viper) {

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Determine the naming convention of the flags when represented in the config file
		configName := f.Name
		// If using camelCase in the config file, replace hyphens with a camelCased string.
		// Since viper does case-insensitive comparisons, we don't need to bother fixing the case, and only need to remove the hyphens.
		if replaceHyphenWithCamelCase {
			configName = strings.ReplaceAll(f.Name, "-", "")
		}

		// Apply the viper config value to the flag when the flag is not set and viper has a value
		l.Debug().Str("flag", f.Name).Str("configName", configName).Msg("Binding flag to viper config:")
		if !f.Changed && v.IsSet(configName) {
			val := v.Get(configName)
			// if the value is an array, loop through it and add each value
			if arr, ok := val.([]interface{}); ok {
				for _, v := range arr {
					if err := cmd.Flags().Set(f.Name, fmt.Sprintf("%v", v)); err != nil {
						l.Error().Str("viper", err.Error()).Send()
					}
				}
			} else {
				if err := cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val)); err != nil {
					l.Error().Str("viper", err.Error()).Send()
				}
			}
		}
	})
}

// getZoneInfo returns the timezone details for a given timezone and date.
// It takes a timezone string and a date string as input and returns a timezoneDetail struct.
// The timezoneDetail struct contains information such as the timezone name, time, abbreviation, offset, and hours for the timezone.
func getZoneInfo(timezone string, date string) timezoneDetail {
	var zone timezoneDetail

	// validate timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		l.Fatal().Str("timezone", timezone).Err(err).Send()
	}
	zone.name = timezone

	parsedDate, _ := time.Parse(time.DateOnly, date)
	// if date == today, use current time, otherwise use midnight in the target location
	if date == time.Now().Format(time.DateOnly) {
		zone.currentTime = time.Now().In(loc)
	} else {
		zone.currentTime = time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), 0, 0, 0, 0, loc)
	}
	var offsetSeconds int
	zone.abbreviation, offsetSeconds = zone.currentTime.In(loc).Zone()
	zone.offsetMinutes = offsetSeconds / 60 // convert to minutes east of UTC
	zone.halfHourOffset = zone.offsetMinutes%60 != 0
	l.Debug().Str("timezone", zone.name).Str("abbreviation", zone.abbreviation).Str("currentTime", zone.currentTime.String()).Int("offsetMinutes", zone.offsetMinutes).Send()

	// get hours for the timezone
	zone.hours = getHours(date, loc)

	return zone
}

// getHours returns a slice of time.Time representing the hours of a given date in a specific time zone.
// It starts at the beginning of the day in UTC and generates the hours by adding each hour to the start time in the target time zone.
// The function takes a time.Time parameter 'date' representing the date for which the hours are generated.
// It also takes a time.Location pointer 'location' representing the time zone in which the hours are generated.
// The function checks if the time zone has a 30-minute offset and adjusts the hours accordingly.
// It returns a slice of time.Time containing the generated hours.
func getHours(date string, location *time.Location) []time.Time {
	// Parse the requested date in UTC to keep a consistent anchor day across all timezones
	d, err := time.ParseInLocation(time.DateOnly, date, time.UTC)
	if err != nil {
		// Fallback to today in UTC if parsing fails
		d = time.Now().UTC()
	}
	startUTC := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)

	hours := make([]time.Time, 24)
	for i := range hours {
		hours[i] = startUTC.Add(time.Duration(i) * time.Hour).In(location)
	}

	return hours
}

// formatHours formats the hours in a given timezone detail.
// It takes a timezoneDetail struct and a boolean flag indicating whether twelve-hour format is enabled.
// It returns a slice of interfaces representing the formatted hours.
func formatHours(z timezoneDetail, twelveHourEnabled bool) []interface{} {
	hours := make([]interface{}, len(z.hours))
	for i, t := range z.hours {
		hour24 := t.Hour()
		if hour24 == 0 {
			hours[i] = t.Format("Mon")
			continue
		}

		if twelveHourEnabled {
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
		} else {
			hours[i] = fmt.Sprintf("%2d", hour24)
		}
	}
	return hours
}

// formatOffset formats the offset of a timezoneDetail struct into a string representation.
// It takes a timezoneDetail struct as input and returns the formatted offset as a string with a +/- sign.
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

// formatRowLabel formats the row label for a timezone detail.
// It takes a timezoneDetail struct, a date string, and an offset string as input.
// If the date is not the current date, it returns the formatted row label with the timezone name, abbreviation, and offset.
// If the date is the current date, it returns the formatted row label with the timezone name, abbreviation, offset, and current time.
func formatRowLabel(z timezoneDetail, date, offset string) string {
	rowLabel := ""
	if date != time.Now().Format(time.DateOnly) {
		rowLabel = fmt.Sprintf("%s [%s,%s]", z.name, z.abbreviation, offset)
	} else {
		rowLabel = fmt.Sprintf("%s [%s,%s]\n%s", z.name, z.abbreviation, offset, z.currentTime.Format("Monday, Jan 2 3:04PM"))
	}
	return rowLabel
}

// parseOffset parses the input string to extract the hour and offset.
// It supports formats like "hour+offset" and "hour-offset".
// If the input does not contain a "+" or "-", it assumes the input is just the hour and sets the offset to 0 (UTC time).
// It returns the parsed hour, offset, and any error encountered during parsing.
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

func validateLiveDateExclusion(cmd *cobra.Command) error {
	if cmd.Flags().Changed("live") && cmd.Flags().Changed("date") {
		return fmt.Errorf("the --live and --date flags are mutually exclusive")
	}
	return nil
}

func parseOffsetMinutes(part string) (int, error) {
	// Support formats like 11, 5.5, 5:30, 0530
	if strings.Contains(part, ":") {
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

	if strings.Contains(part, ".") {
		value, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid offset: %w", err)
		}
		return int(math.Round(value * 60)), nil
	}

	if len(part) == 4 { // handle HHMM without colon, e.g., 0530
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

	hours, err := strconv.Atoi(part)
	if err != nil {
		return 0, fmt.Errorf("invalid offset hours: %w", err)
	}
	return hours * 60, nil
}

// parseHighlightFlag parses the highlight flag and returns the index of the
// highlighted timezone in the provided timezone details. If the highlight flag
// is invalid or the timezone is not found, it returns an error.
//
// Parameters:
//
//	highlight - A string representing the highlight flag.
//	zones - A timezoneDetails object containing the available timezones.
//
// Returns:
//
//	int - The index of the highlighted timezone.
//	error - An error if the highlight flag is invalid or the timezone is not found.
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

// hasTimezoneWithOffset checks if there is a timezone with the specified offset in the provided timezone details.
// It returns true if a timezone with the offset is found, otherwise false.
func hasTimezoneWithOffset(zones timezoneDetails, offsetMinutes int) bool {
	for _, z := range zones {
		if z.offsetMinutes == offsetMinutes {
			return true
		}
	}
	return false
}

// printTimeTable prints the time table for the given zones.
// It takes a slice of timezoneDetails and a boolean flag indicating whether color is enabled.
// The function uses the table package to create a table and display the time information.
// If colorEnabled is true, the table is styled with colored text, otherwise it is styled with rounded borders.
// If the requested date is not today, a table caption is added with the date in the format "Monday, January 2, 2006".
// If the requested date is today, the current local time is displayed in the table title.
// The function iterates over the zones and formats the hours, offset, and row label for each zone.
// The formatted data is then appended to the table row and the row is added to the table.
// Finally, the table is rendered and displayed on the console.
func printTimeTable(zones timezoneDetails, colorEnabled bool, highlightHour int) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	if colorEnabled {
		t.SetStyle(table.StyleColoredBlackOnBlueWhite)
		t.Style().Title.Colors = text.Colors{text.BgHiBlue, text.FgHiWhite}
		t.Style().Color.IndexColumn = text.Colors{text.BgHiBlue, text.FgHiWhite, text.Bold}
		t.Style().Color.RowAlternate = text.Colors{text.Color(30), text.Color(47)}
	} else {
		t.SetStyle(table.StyleRounded)
		t.Style().Options.DoNotColorBordersAndSeparators = true
		t.Style().Options.SeparateColumns = false
		t.Style().Options.SeparateRows = true
		t.Style().Color.IndexColumn = text.Colors{text.FgHiBlue, text.Bold}
	}
	t.Style().Title.Align = text.AlignCenter

	// --highlight should override the current hour
	if highlightHour > -1 {
		t.SetIndexColumn(highlightHour + 2) // +2 because first col=timezone and hours count from 0
	} else if date == time.Now().Format(time.DateOnly) {
		t.SetIndexColumn(time.Now().UTC().Hour() + 2) // +2 because first col=timezone and hours count from 0
	}

	if date != time.Now().Format(time.DateOnly) {
		// add table caption if requested date is not today
		d, _ := time.Parse(time.DateOnly, date)
		t.SetTitle("Showing Time For: %s", d.Format("Monday, January 2, 2006 MST"))
	} else {
		// date requested == today, identify the table column holding the current hour
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

// deduplicateSlice removes duplicate elements from a string slice.
// It iterates through the input slice and checks if each element exists in the rest of the slice.
// If an element is not found in the rest of the slice, it is added to the result slice.
// The function returns the deduplicated string slice.
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
	Version: "v1.2.6",
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
	Args: func(cmd *cobra.Command, args []string) error {
		if err := validateLiveDateExclusion(cmd); err != nil {
			return err
		}

		// if the --date flag was provided, validate it
		if cmd.Flags().Changed("date") {
			_, err := time.Parse(time.DateOnly, date)
			if err != nil {
				l.Fatal().Str("date", date).Err(err).Send()
			}
		}

		// if the --exclude-local flag was NOT provided explicitly, add the local timezone to the timezones slice
		if !cmd.Flags().Changed("exclude-local") {
			ltz, err := time.LoadLocation("Local")
			if err != nil {
				l.Fatal().Err(err).Send()
			}
			found := false
			for _, tz := range timezones {
				if tz == ltz.String() {
					found = true
					break
				}
			}
			if !found {
				timezones = append([]string{ltz.String()}, timezones...)
			}
		}

		// deduplicate timezones in case the user specified the same timezone multiple times
		timezones = deduplicateSlice(timezones)

		return nil
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// bind cobra and viper
		return initializeConfig(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Check if wizard mode was requested
		if wizardMode, _ := cmd.Flags().GetBool("wizard"); wizardMode {
			selected, err := runWizard()
			if err != nil {
				l.Fatal().Err(err).Msg("Wizard failed")
			}
			if selected != nil {
				// Update timezones with wizard selections
				timezones = selected
				v.Set("timezone", selected)
				if err := v.WriteConfig(); err != nil {
					l.Error().Err(err).Msg("Failed to save config")
				}
			}
			return
		}

		// Log all settings at debug level
		for k, v := range v.AllSettings() {
			l.Debug().Str(k, fmt.Sprintf("%v", v)).Msg("viper")
		}

		// Save user preferences to config file
		saveUserPreferences()

		// Live mode: continuously refresh the time table
		if liveMode {
			runLiveMode(cmd)
			return
		}

		// Process timezone data
		zones := processTimezones()

		// Handle highlight flag
		highlightHour, err := processHighlightFlag(cmd, zones)
		if err != nil {
			l.Error().Err(err).Msg("Invalid highlight specification")
			os.Exit(1)
		}

		// Render the time table
		printTimeTable(zones, colorEnabled, highlightHour)
	},
}

// clearScreen clears the terminal screen using ANSI escape codes
func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

// runLiveMode continuously refreshes the time table at the specified interval
func runLiveMode(cmd *cobra.Command) {
	// Set up signal handling for graceful exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create a ticker for the refresh interval
	ticker := time.NewTicker(time.Duration(liveInterval) * time.Second)
	defer ticker.Stop()

	// Initial render
	renderTimeTable(cmd)

	fmt.Println("\nLive mode active. Press Ctrl+C to exit.")

	for {
		select {
		case <-sigChan:
			fmt.Println("\nExiting live mode...")
			return
		case <-ticker.C:
			clearScreen()
			renderTimeTable(cmd)
			fmt.Println("\nLive mode active. Press Ctrl+C to exit.")
		}
	}
}

// renderTimeTable processes timezones and renders the time table
func renderTimeTable(cmd *cobra.Command) {
	zones := processTimezones()
	highlightHour, err := processHighlightFlag(cmd, zones)
	if err != nil {
		l.Error().Err(err).Msg("Invalid highlight specification")
		return
	}
	printTimeTable(zones, colorEnabled, highlightHour)
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.SetVersionTemplate(`{{printf "timeBuddy %s\n" .Version}}`)
	rootCmd.Flags().BoolVarP(&colorEnabled, "color", "c", false, "enable colorized table output. If previously enabled, use --color=false to disable it,")
	rootCmd.Flags().StringVarP(&date, "date", "d", time.Now().Format(time.DateOnly), "``date to use for time conversion. Expects YYYY-MM-DD format. Defaults to current date/time.")
	rootCmd.Flags().StringVarP(&highlight, "highlight", "H", "", "highlight hour column (0-23), optionally with UTC offset (e.g., '15+11' or '9-4')")
	rootCmd.Flags().BoolVarP(&liveMode, "live", "l", false, "enable live mode to continuously refresh the time display (press Ctrl+C to exit)")
	rootCmd.Flags().IntVarP(&liveInterval, "interval", "i", 1, "refresh interval in seconds for live mode")
	rootCmd.Flags().BoolVarP(&twelveHourEnabled, "twelve-hour", "t", false, "use 12-hour time format instead of 24-hour. If previously enabled, use --twelve-hour=false to disable it.")
	rootCmd.PersistentFlags().CountP("verbose", "v", "``increase logging verbosity, 1=warn, 2=info, 3=debug, 4=trace")
	rootCmd.Flags().BoolP("exclude-local", "x", false, "disable default behavior of including local timezone in output")
	rootCmd.Flags().BoolP("wizard", "w", false, "launch interactive timezone selector wizard")
	rootCmd.Flags().StringArrayVarP(&timezones, "timezone", "z", []string{}, "``timezone to use for time conversion. Accepts timezone name, like America/New_York. Can be used multiple times.")

	// Enforce mutual exclusion for live and date at the flag level
	rootCmd.MarkFlagsMutuallyExclusive("live", "date")

	err := rootCmd.RegisterFlagCompletionFunc("timezone", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return timezonesAll, cobra.ShellCompDirectiveDefault
	})
	if err != nil {
		l.Error().Err(err).Send()
	}
}

// saveUserPreferences persists current user preferences to the config file
func saveUserPreferences() {
	v.Set("color", colorEnabled)
	v.Set("timezone", timezones)
	v.Set("twelve-hour", twelveHourEnabled)

	if err := v.WriteConfig(); err != nil {
		l.Error().Err(err).Msg("Failed to save preferences")
	}
}

// processTimezones collects timezone information for display
func processTimezones() timezoneDetails {
	zones := make(timezoneDetails, 0, len(timezones))
	for _, z := range timezones {
		zones = append(zones, getZoneInfo(z, date))
	}
	return zones
}

// processHighlightFlag parses and validates the highlight flag if provided
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
