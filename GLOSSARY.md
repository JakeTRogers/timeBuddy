# timeBuddy

A command-line tool that lays several timezones side by side, hour by hour, so you can find times that work across them.

## Timezones

**Timezone**:
A named IANA region whose clock follows one set of offset and daylight saving rules, such as `America/New_York`.
_Avoid_: Time zone, zone, tz

**Area**:
The first part of a timezone name, grouping its timezones by continent or ocean, such as `America` in `America/New_York`.
_Avoid_: Region, continent

**Location**:
The part of a timezone name after its area, usually a representative city, such as `New_York` in `America/New_York`.
_Avoid_: City, place

**UTC offset**:
How far a timezone's clock is ahead of or behind UTC at a given moment, such as +11 or +5:30. A timezone can have two UTC offsets on the same date when daylight saving time starts or ends.
_Avoid_: Offset (unqualified), GMT offset

**Selected timezones**:
The ordered list of timezones a user has chosen to see, remembered from one run to the next.
_Avoid_: Saved timezones, configured timezones, last used timezones

**Local timezone**:
The timezone of the machine timeBuddy runs on. Unless excluded, it is shown automatically: as the first row when it is not one of the selected timezones, and in its selected position when it is. Showing it automatically does not make it a selected timezone.
_Avoid_: Home timezone, system timezone, default timezone

## The time table

**Time table**:
The grid timeBuddy shows: one row per timezone and one column per hour of the display date, so each column is a single moment read off every timezone's clock.
_Avoid_: Grid, chart, output

**Display date**:
The UTC calendar day a time table covers, from 00:00 to 23:00 UTC. Each row shows that same span on its own clock, so a row can start on the previous or end on the next local day.
_Avoid_: Local date, day

**Hour column**:
One column of the time table: a single UTC hour of the display date, shown in each row as that timezone's local time.
_Avoid_: Slot, cell

**Highlighted hour**:
The single hour column the time table emphasizes: the hour chosen with `--highlight`, or, when none is chosen and the display date is today, the current hour. A table for another date with no chosen hour has no highlighted hour. A chosen hour is stated as a local hour at a UTC offset, and must match the UTC offset a displayed timezone has at that hour.
_Avoid_: Index column, current-hour column, now column
