# timeBuddy

CLI based version of World Time Buddy

## Description

TimeBuddy is a CLI tool, implemented in Go, that mimics the functionality of World Time Buddy. It accepts a list of timezone and displays the current time in each of these timezones in a tabular format. The program is aware of daylight savings time and adjusts the time for each timezone accordingly. A specific date can also be provided to view the time in each timezone for that date.

The last used timezones, color, and time format preferences are saved in a YAML formatted configuration file. This feature ensures that you need to
specify your preferred time zones only once. The order in which you specify the time zones is retained and reflected in the table output.

- Windows: `$HOME/AppData/Roaming/.timeBuddy.yaml`
- Linux/macOS: `~/.config/.timeBuddy.yaml`

If the configuration file does not exist, it will be created. The configuration file follows the format:

```yaml
color: true
timezone:
    - Local
    - America/New_York
    - Europe/Vilnius
    - Australia/Adelaide
twelve-hour: false
```

## Screenshots

![timeBuddy No Color & No Config](screenshots/timeBuddy-no-color-no-config.png)
![timeBuddy No Color w/ Timezones](screenshots/timeBuddy-no-color-w-timezones.png)
![timeBuddy Color](screenshots/timeBuddy-color-w-timezones.png)
![timeBuddy Color w/ 12hr](screenshots/timeBuddy-color-w-timezones-12hr.png)
![timeBuddy Color w/ DST Date](screenshots/timeBuddy-color-w-date-dst.png)

## Installation

1. Download the binary for your preferred platform from the [releases](https://github.com/JakeTRogers/timeBuddy/releases) page
2. Extract the archive. It can contains the markdown generated documentation and the binary
3. Copy the binary to a directory in your `$PATH`. i.e. `/usr/local/bin`

## Usage

```text
Usage:
  timeBuddy [flags]
  timeBuddy [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  list        List time zones

Flags:
  -c, --color           enable colorized table output. If previously enabled, use --color=false to disable it,
  -d, --date            date to use for time conversion. Expects YYYY-MM-DD format. Defaults to current date/time. (default "2024-01-02")
  -x, --exclude-local   disable default behavior of including local timezone in output
  -h, --help            help for timeBuddy
  -z, --timezone        timezone to use for time conversion. Accepts timezone name, like America/New_York. Can be used multiple times.
  -t, --twelve-hour     use 12-hour time format instead of 24-hour. If previously enabled, use --twelve-hour=false to disable it.
  -v, --verbose         increase logging verbosity, 1=warn, 2=info, 3=debug, 4=trace
      --version         version for timeBuddy

Use "timeBuddy [command] --help" for more information about a command.
```
