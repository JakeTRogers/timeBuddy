# TimeBuddy Copilot Instructions

We are building a CLI tool named "TimeBuddy" in Go 1.23 that displays current or specified times across multiple timezones.

We use:

- github.com/spf13/cobra for the CLI commands.
- github.com/spf13/viper for YAML-based configuration (timezones, color, time format).
- github.com/rs/zerolog for structured logging.
- github.com/jedib0t/go-pretty/v6 for tabular output.

The tool is aware of daylight savings time (DST) and can accept a specific date to show localized times.

Please keep these dependencies in mind when suggesting or generating code.
