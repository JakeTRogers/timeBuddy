/*
Copyright © 2024 Jake Rogers <code@supportoss.org>
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	offset         int
	halfHourOffset bool
	hours          []int
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
	l.Debug().Str("configPath", configPath).Send()
	v.AddConfigPath(configPath)

	// Attempt to read the config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Create config file if it doesn't exist
			if err := v.SafeWriteConfig(); err != nil {
				l.Error().Err(err).Send()
			}
			l.Info().Str("configFile", filepath.Join(configPath, configName+"."+configType)).Msg("New config file created:")
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
	// if date == today, use current time, otherwise use midnight
	if date == time.Now().Format(time.DateOnly) {
		zone.currentTime = time.Now().Local().In(loc)
	} else {
		d, _ := time.Parse(time.DateOnly, date)
		zone.currentTime = time.Date(d.Year(), d.Month(), d.Day(), d.Hour(), d.Minute(), d.Second(), d.Nanosecond(), loc)
	}
	zone.abbreviation, zone.offset = zone.currentTime.In(loc).Zone()
	zone.halfHourOffset = zone.offset%3600 != 0
	zone.offset = zone.offset / 3600 // convert offset from seconds east of UTC to hours
	l.Debug().Str("timezone", zone.name).Str("abbreviation", zone.abbreviation).Str("currentTime", zone.currentTime.String()).Int("offset", zone.offset).Send()

	// get hours for the timezone
	hours := getHours(zone.currentTime, loc)
	for _, h := range hours {
		zone.hours = append(zone.hours, h.Hour())
	}

	return zone
}

// getHours returns a slice of time.Time representing the hours of a given date in a specific time zone.
// It starts at the beginning of the day in UTC and generates the hours by adding each hour to the start time in the target time zone.
// The function takes a time.Time parameter 'date' representing the date for which the hours are generated.
// It also takes a time.Location pointer 'location' representing the time zone in which the hours are generated.
// The function checks if the time zone has a 30-minute offset and adjusts the hours accordingly.
// It returns a slice of time.Time containing the generated hours.
func getHours(date time.Time, location *time.Location) []time.Time {
	// Start at the beginning of the day in UTC
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)

	// Check if the timezone has a 30-minute offset
	_, offset := time.Now().In(location).Zone()
	halfHourOffset := offset%3600 != 0

	// Generate the hours
	hours := make([]time.Time, 24)
	for i := range hours {
		// if the timezone has a 30-minute offset and the current time is past the half hour mark, add 30 minutes to the hour
		if halfHourOffset && time.Now().UTC().Minute() >= 30 {
			hours[i] = start.Add(time.Duration(i)*time.Hour + 30*time.Minute).In(location)
		} else {
			hours[i] = start.Add(time.Duration(i) * time.Hour).In(location)
		}
	}

	return hours
}

// formatHours formats the hours in a given timezone detail.
// It takes a timezoneDetail struct and a boolean flag indicating whether twelve-hour format is enabled.
// It returns a slice of interfaces representing the formatted hours.
func formatHours(z timezoneDetail, twelveHourEnabled bool) []interface{} {
	hours := make([]interface{}, len(z.hours))
	for i, v := range z.hours {
		if v == 0 {
			hours[i] = fmt.Sprintf("%v", z.currentTime.Format("Mon"))
		} else if twelveHourEnabled {
			if v > 12 {
				hours[i] = fmt.Sprintf("%2v\npm", v-12)
			} else {
				hours[i] = fmt.Sprintf("%2v\nam", v)
			}
		} else {
			hours[i] = fmt.Sprintf("%2v", v)
		}
	}
	return hours
}

// formatOffset formats the offset of a timezoneDetail struct into a string representation.
// It takes a timezoneDetail struct as input and returns the formatted offset as a string with a +/- sign.
func formatOffset(z timezoneDetail) string {
	offset := ""
	if z.offset >= 0 {
		offset = fmt.Sprintf("+%d", z.offset)
	} else {
		offset = fmt.Sprintf("%d", z.offset)
	}
	if z.halfHourOffset {
		offset = fmt.Sprintf("%s.5", offset)
	}
	return offset
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

// printTimeTable prints the time table for the given zones.
// It takes a slice of timezoneDetails and a boolean flag indicating whether color is enabled.
// The function uses the table package to create a table and display the time information.
// If colorEnabled is true, the table is styled with colored text, otherwise it is styled with rounded borders.
// If the requested date is not today, a table caption is added with the date in the format "Monday, January 2, 2006".
// If the requested date is today, the current local time is displayed in the table title.
// The function iterates over the zones and formats the hours, offset, and row label for each zone.
// The formatted data is then appended to the table row and the row is added to the table.
// Finally, the table is rendered and displayed on the console.
func printTimeTable(zones timezoneDetails, colorEnabled bool) {
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

	if date != time.Now().Format(time.DateOnly) {
		// add table caption if requested date is not today
		d, _ := time.Parse(time.DateOnly, date)
		t.SetTitle("Showing Time For: %s", d.Format("Monday, January 2, 2006 MST"))
	} else {
		// date requested == today, identify the table column holding the current hour
		t.SetIndexColumn(time.Now().UTC().Hour() + 2) // +2 because first col=timezone and hours count from 0
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
	var result []string
	for i, v := range s {
		// Check if v is in the rest of the slice
		exists := false
		for j := i + 1; j < len(s); j++ {
			if s[j] == v {
				exists = true
				break
			}
		}
		// If v is not in the rest of the slice, add it to the result
		if !exists {
			result = append(result, v)
		}
	}
	return result
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "timeBuddy",
	Version: "v1.1.3",
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

  # Display Time for a specific date(useful for checking times during Daylight Saving Time changes):
  $ timeBuddy --date 2023-11-05 --timezone America/New_York --timezone Europe/Vilnius --timezone Australia/Sydney

  # Exclude your local time zone from the output:
   $ timeBuddy --exclude-local --timezone --timezone Europe/London --timezone Asia/Tokyo

  # Enable colorized table output:
   $ timeBuddy --color

Learn More:
  To submit feature requests, bugs, or to check for new versions, visit https://github.com/JakeTRogers/timeBuddy`,
	Args: func(cmd *cobra.Command, args []string) error {
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
		for k, v := range v.AllSettings() {
			l.Debug().Str(k, fmt.Sprintf("%v", v)).Msg("viper:")
		}

		// write preferences to config file
		v.Set("color", colorEnabled)
		v.Set("timezone", timezones)
		v.Set("twelve-hour", twelveHourEnabled)
		if err := v.WriteConfig(); err != nil {
			l.Error().Str("viper", err.Error()).Send()
		}

		// loop over the timezones and get the details for each
		var zones timezoneDetails
		for _, z := range timezones {
			zones = append(zones, getZoneInfo(z, date))
		}

		printTimeTable(zones, colorEnabled)
	},
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
	rootCmd.Flags().BoolVarP(&twelveHourEnabled, "twelve-hour", "t", false, "use 12-hour time format instead of 24-hour. If previously enabled, use --twelve-hour=false to disable it.")
	rootCmd.PersistentFlags().CountP("verbose", "v", "``increase logging verbosity, 1=warn, 2=info, 3=debug, 4=trace")
	rootCmd.Flags().BoolP("exclude-local", "x", false, "disable default behavior of including local timezone in output")
	rootCmd.Flags().StringArrayVarP(&timezones, "timezone", "z", []string{}, "``timezone to use for time conversion. Accepts timezone name, like America/New_York. Can be used multiple times.")
	err := rootCmd.RegisterFlagCompletionFunc("timezone", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return timezonesAll, cobra.ShellCompDirectiveDefault
	})
	if err != nil {
		l.Error().Err(err).Send()
	}
}
