# Roadmap

Planned work, about 30 hours in total. Each item has a GitHub issue with the `roadmap` label. Estimates are rough.

## Input

1. **Read a crontab file** (3 h). `cron-explain -file crontab` checks every line: skips comments and blank lines, understands `VAR=value` lines and applies `CRON_TZ`/`TZ`, supports the system crontab format with a user column (`/etc/crontab`, `/etc/cron.d/*`), and prints a report per line with its line number. Today each expression has to be cut out by hand.
2. **Quartz syntax** (4 h). `-dialect quartz`: `?`, `L` (last day, last weekday), `W` (nearest weekday), `#` (nth weekday of the month), an optional year field, and Quartz day-of-week numbering (1 = Sunday).
3. **Dialect differences** (2 h). `-dialect robfig` for Go's robfig/cron and Kubernetes CronJob rules (OR day rule when one field is `*/n`, supported macros), with a table in the README of where dialects disagree.

## Output

4. **Match a daemon's daylight saving behaviour** (4 h). `-dst cronie` lists runs the way cronie actually runs them: jobs from a skipped interval right after the change, no second run in a repeated interval for fixed-time jobs, normal scheduling for jobs that run more often than hourly. Tests per rule from cronie's manual.
5. **List the values of uneven steps in the description** (1 h). `0 */7 * * *` reads "every 7 hours" today, which is wrong after 21:00; say "at minute 0 past hours 00, 07, 14 and 21".
6. **12-hour clock** (1 h). `-12h` prints "9:30 AM" style times in descriptions and run lists.
7. **Previous runs** (2 h). `-prev N` lists the last N runs before `-from`, to answer "should this have run last night?".
8. **Clearer English for common patterns** (3 h). "every 2 hours, on the hour", "at minute 0 past every hour from 09:00 through 17:00", "on the first Monday" style phrasing, backed by a golden-file test with at least 100 expressions.
9. **Time zone database information** (1 h). Show which tz database version was used (system or Go's built-in copy) and add a flag to force the built-in one, since rule changes (such as British Columbia's in 2026) change answers.

## Use in CI

10. **`-strict` exit code** (2 h). Exit non-zero when warnings are found, with `-ignore code,code` to allow known ones. Document a GitHub Actions step and a pre-commit hook that check crontab files in a repository.

## Quality

11. **Fuzz and cross-check tests** (2 h). A `go test -fuzz` target for `Parse`, and a test that compares `Next` with a slow minute-by-minute search over random expressions and zones.
12. **Faster search for long gaps** (2 h). `Next` builds every time of a matching day; skip non-matching months and hours directly and add benchmarks for second-level schedules and rare dates like February 29 on a Monday.
13. **Package examples** (1 h). `Example` functions for `Parse`, `Explain`, `Next` and `Check` so they show on pkg.go.dev.

## Releases

14. **Release binaries** (2 h). A tag-triggered workflow that builds macOS, Linux and Windows binaries with SHA-256 checksums and attaches them to the GitHub release, with pinned actions and no extra services.
