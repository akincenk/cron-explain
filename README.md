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

### Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `-tz` | this computer's zone | IANA time zone, such as `Europe/Berlin` |
| `-n` | 5 | how many next runs to list (0 to 1000) |
| `-from` | now | list runs after this time: RFC 3339 or `YYYY-MM-DD HH:MM` in the `-tz` zone |
| `-json` | off | print JSON (`expression`, `description`, `time_zone`, `runs`, `warnings`) |

Exit codes: 0 success, 1 invalid expression, 2 invalid flags.

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
```

## Example

[examples/shop-server.crontab](examples/shop-server.crontab) is a made-up crontab with several of the mistakes above. It was used to try the tool by hand.

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
