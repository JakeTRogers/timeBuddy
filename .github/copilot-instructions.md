# timeBuddy Copilot Instructions

## Project Overview

timeBuddy is a Go CLI tool (similar to World Time Buddy) that displays current time across multiple timezones in a tabular format. Uses Cobra for CLI, Viper for configuration persistence, and go-pretty for table rendering.

## Architecture

```
main.go           → Entry point, calls cmd.Execute()
cmd/
  root.go         → Main command, timezone processing, table rendering, live mode
  list.go         → Lists available timezones (timezonesAll slice)
  wizard.go       → Interactive TUI for timezone selection (Bubbletea/Lipgloss)
logger/
  logger.go       → Zerolog wrapper with verbosity levels
```

### Key Data Flows

1. **Config Loading**: `initializeConfig()` → Viper reads `~/.config/.timeBuddy.yaml` → binds flags via `bindFlags()`
2. **Timezone Processing**: `processTimezones()` → `getZoneInfo()` per zone → `timezoneDetail` structs (returns errors)
3. **Table Rendering**: `printTimeTable()` uses go-pretty with conditional styling (color/no-color)

## Development Commands

```bash
# Run tests (standard Go testing)
go test ./...

# Run with coverage
go test ./... -cover

# Run golangci-lint
golangci-lint run ./...

# Run with verbose logging (1=warn, 2=info, 3=debug, 4=trace)
go run . -vvv

# Build binary
go build -o timeBuddy .
```

## Code Patterns

### Error Handling (go.instructions.md compliant)

Helper functions return errors; CLI boundary handles logging/exit:

```go
// Helper function returns wrapped error
func getZoneInfo(timezone, date string) (timezoneDetail, error) {
    loc, err := time.LoadLocation(timezone)
    if err != nil {
        return timezoneDetail{}, fmt.Errorf("invalid timezone %q: %w", timezone, err)
    }
    // ...
}

// CLI boundary (RunE) handles the error
RunE: func(cmd *cobra.Command, args []string) error {
    zones, err := processTimezones()
    if err != nil {
        return err  // Cobra displays error to user
    }
    // ...
}
```

### CLI Commands use RunE

All commands use `RunE` (not `Run`) for proper error propagation:

```go
var myCmd = &cobra.Command{
    Use: "mycommand",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Return errors instead of calling log.Fatal()
        return nil
    },
}
```

### CLI Flags with Persistent Config

Flags auto-save to config via `saveUserPreferences()`. To add a new persistent flag:

```go
// In init():
rootCmd.Flags().BoolVarP(&myFlag, "my-flag", "m", false, "description")

// In saveUserPreferences():
v.Set("my-flag", myFlag)
```

### Timezone Validation

Validate timezones through `time.LoadLocation()` and return errors:

```go
loc, err := time.LoadLocation(timezone)
if err != nil {
    return timezoneDetail{}, fmt.Errorf("invalid timezone %q: %w", timezone, err)
}
```

### Table-Driven Tests

Tests follow `Test_functionName_scenario` naming convention:

```go
func Test_parseOffset(t *testing.T) {
    tests := []struct {
        name           string
        input          string
        expectedHour   int
        expectError    bool
    }{...}

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}
```

### Logging Convention

Use the package-level logger `log` from `logger.GetLogger()`. Don't log and return errors (choose one):

```go
// In helpers: return errors, don't log
func processData() error {
    if err != nil {
        return fmt.Errorf("processing failed: %w", err)
    }
}

// At CLI boundary: log if needed
log.Debug().Str("key", value).Msg("message")
log.Error().Err(err).Msg("non-fatal context")  // Only for non-fatal side effects
```

## Testing Considerations

- `time/tzdata` is embedded (import `_ "time/tzdata"`) - tests work without system timezone data
- Test fixtures in `cmd/root_test.go` provide `testZones` and `testTime` for consistent testing
- Test names follow `Test_functionName_scenario` convention
- Helper functions that return errors have corresponding `_errors` test functions
- The wizard uses Bubbletea - integration tests may need to mock `tea.Program`

## Config File Locations

- **Linux/macOS**: `~/.config/.timeBuddy.yaml`
- **Windows**: `%APPDATA%\.timeBuddy.yaml`

Config keys: `color`, `timezone` (array), `twelve-hour`
