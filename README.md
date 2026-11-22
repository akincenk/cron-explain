# cron-explain

[![CI](https://github.com/akincenk/cron-explain/actions/workflows/ci.yml/badge.svg)](https://github.com/akincenk/cron-explain/actions/workflows/ci.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/akincenk/cron-explain/badge)](https://scorecard.dev/viewer/?uri=github.com/akincenk/cron-explain)

A Go library and command-line tool that reads a cron expression and tells you:

- what it means, in one plain English sentence,
- when it runs next, in a time zone you choose, including what happens on the days clocks change for daylight saving time,
- which parts are likely mistakes.

```console
$ cron-explain -tz America/Toronto -from '2026-10-31 00:00' -n 3 '30 1 * * *'
30 1 * * *
At 01:30.

Next runs (America/Toronto):
  2026-10-31 01:30:00 EDT (-04:00) Sat
  2026-11-01 01:30:00 EDT (-04:00) runs once; this time happens again at 01:30:00 EST (-05:00) when clocks move back
  2026-11-02 01:30:00 EST (-05:00) Mon

Warnings:
  [dst-repeat] On 2026-11-01 clocks in America/Toronto move back: 01:30:00 happens twice; this tool counts one run each, at the first one (EDT).
```

## Who it is for

People who write or review crontab lines, CI schedules and job scheduler settings and want to check them before they go live: developers, system administrators, and anyone who has had a nightly job silently skip a night in March.

## What it checks

| Code | Meaning |
| --- | --- |
| `never-runs` | No date matches, for example `0 0 30 2 *` (February 30). |
| `day-or-weekday` | Day of month and day of week are both set, so cron runs when either matches. `0 6 1 * 1` runs on the 1st and on every Monday. |
| `star-step-day` | One day field is `*/n` and the other is set. Vixie cron and cronie require both to match; robfig/cron (Go) runs when either matches. |
| `uneven-step` | A step that does not divide the field, such as `*/7` in minutes: runs at :56 and again at :00. |
| `short-month` | Day 29 to 31 is missing in some of the selected months. |
| `leap-day` | February 29 exists only in leap years. |
| `dst-skip` | A run time falls in the hour skipped when clocks move forward (within the next year). |
| `dst-repeat` | A run time falls in the hour that happens twice when clocks move back (within the next year). |

## Supported syntax

- Five fields: minute, hour, day of month, month, day of week.
- Six fields: a seconds field in front.
- `*`, single values, ranges `a-b`, lists `a,b`, steps `*/n`, `a-b/n` and `a/n` (same as `a-max/n`).
- Month names `JAN`-`DEC` and weekday names `SUN`-`SAT`, any case. `0` and `7` both mean Sunday.
- Macros `@yearly`, `@annually`, `@monthly`, `@weekly`, `@daily`, `@midnight`, `@hourly`, `@reboot`.

Quartz syntax (`?`, `L`, `W`, `#`, a year field) is planned, see [ROADMAP.md](ROADMAP.md).

### Day of month and day of week

Like Vixie cron and cronie, a day field that starts with `*` (including `*/2`) counts as "not set". When both day fields are set, a day matches if either one matches. Otherwise both have to match.

### Daylight saving time

cron-explain works with wall-clock time in the chosen zone:

- A run time that does not exist because clocks move forward is listed as `skipped` and is not counted as a run.
- A run time that happens twice because clocks move back runs once, at the first instant. The output shows both instants.

Real cron daemons differ here. cronie's manual says that for jobs at a specific time, or with a granularity greater than one hour, it runs jobs from a skipped interval right away and avoids running a job twice in a repeated one, while jobs that run more often are scheduled normally. cron-explain shows the plain wall-clock answer and warns you about the affected days; matching a specific daemon is on the roadmap.

Time zone rules come from the system's time zone database, with Go's built-in copy as a fallback. Rules do change: British Columbia, for example, stays on UTC-7 all year from November 2026, so older databases give different answers for `America/Vancouver`.

## Install and run

You need Go 1.26 or newer.

```console
go install github.com/akincenk/cron-explain/cmd/cron-explain@latest
```

Or from a clone:

```console
git clone https://github.com/akincenk/cron-explain
cd cron-explain
go build ./cmd/cron-explain
./cron-explain '*/15 9-17 * * 1-5'
```

Quote the expression so your shell does not expand `*`.

### Crontab files

`-file` checks every job line of a crontab and prints one report per line, with its line number, followed by a count of lines with warnings and errors. `-file -` reads standard input, so `crontab -l | cron-explain -n 1 -file -` checks your own crontab.

```console
cron-explain -n 1 -file examples/shop-server.crontab
cron-explain -n 1 -system -file /etc/cron.d/backup
```

- Blank lines and lines starting with `#` are skipped. A `#` later in a job line is part of the command, as in cron.
- `NAME=value` lines are variables. `CRON_TZ` sets the time zone for the job lines after it and overrides `-tz`; `CRON_TZ=` goes back to `-tz`. `TZ` is only passed to commands and does not change the schedule, which is how cronie handles it.
- `-system` reads the format of `/etc/crontab` and `/etc/cron.d`, where a user column follows the schedule.
- Crontab files have five time fields. A line with a seconds field shifts the columns, so a time-like value where the command (or user) should be is reported as an error, as is a user column that looks like a command.
- `-from` is read in the `-tz` zone even when the file sets `CRON_TZ`; use RFC 3339 with an offset to be exact.

The exit code is 1 when any line has an error; the other lines are still reported. With `-json` the output is a list of objects with `line`, `user`, `command`, the fields of a single report, and `error` for lines that could not be read.

### Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `-tz` | this computer's zone | IANA time zone, such as `Europe/Berlin` |
| `-n` | 5 | how many next runs to list (0 to 1000) |
| `-from` | now | list runs after this time: RFC 3339 or `YYYY-MM-DD HH:MM` in the `-tz` zone |
| `-json` | off | print JSON (`expression`, `description`, `time_zone`, `runs`, `warnings`) |
| `-file` | none | check every job line of a crontab file, `-` for standard input |
| `-system` | off | with `-file`: lines have a user column (`/etc/crontab`, `/etc/cron.d`) |

Exit codes: 0 success, 1 invalid expression (or any invalid line with `-file`), 2 invalid flags or unreadable file.

## Library

```go
s, err := cronexplain.Parse("30 2 * * *")
if err != nil {
	log.Fatal(err) // *cronexplain.ParseError names the field and the problem
}
fmt.Println(s.Explain()) // At 02:30.

loc, _ := time.LoadLocation("America/Toronto")
for _, o := range s.Next(time.Now(), loc, 5) {
	fmt.Println(o.Describe())
}
for _, w := range s.Check(time.Now(), loc) {
	fmt.Println(w.Code, w.Message)
}

f, _ := os.Open("crontab")
entries, err := cronexplain.ParseCrontab(f, false) // true for /etc/crontab format
for _, e := range entries {
	if e.Err != nil {
		fmt.Println(e.Err) // line number and problem
		continue
	}
	fmt.Println(e.Line, e.Schedule.Explain(), e.TZ) // TZ is CRON_TZ in effect, or ""
}
```

## Example

[examples/shop-server.crontab](examples/shop-server.crontab) is a made-up crontab with several of the mistakes above, and [examples/shop-cron.d](examples/shop-cron.d) a made-up `/etc/cron.d` file with a missing user column, a seconds field and a misspelled `CRON_TZ`. Both were used to try the tool by hand:

```console
cron-explain -n 1 -file examples/shop-server.crontab
cron-explain -n 1 -system -file examples/shop-cron.d
```

## Development

```console
go test -race ./...
go vet ./...
gofmt -l .
```

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Similar projects

- [lnquy/cron](https://github.com/lnquy/cron) turns cron expressions into descriptions in many languages. It does not list run times.
- [adhocore/gronx](https://github.com/adhocore/gronx) and [robfig/cron](https://github.com/robfig/cron) parse expressions and compute next run times for scheduling jobs.
- [crontab.guru](https://crontab.guru) explains expressions in the browser.

cron-explain puts the description, the run list with daylight saving details, and the warnings in one offline command.

## License

MIT, see [LICENSE](LICENSE).
