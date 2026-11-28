# The display date is a UTC calendar day

A time table covers the 24 UTC hours of its display date, not the viewer's local day or each timezone's own day. A fixed UTC span means every hour column is one moment read off every row's clock, so rows always line up, and the table looks the same whichever machine renders it. The cost is that a row can start on the previous local day or end on the next one; when a timezone changes its UTC offset partway through that span, its offset is read per hour column rather than once per row.

## Considered Options

- **The viewer's local day**: rows still line up, but the same `--date` gives a different table on machines in different timezones.
- **Each timezone's own local day**: every row would run midnight to midnight, but its columns would no longer be shared moments, which defeats the side-by-side comparison.
