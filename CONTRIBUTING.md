# Contributing

Thanks for taking a look. Issues and pull requests are welcome.

## Before you start

- For anything bigger than a small fix, open an issue first so we can agree on the approach.
- Planned work is in [ROADMAP.md](ROADMAP.md); each item has a GitHub issue.

## Working on the code

You need Go 1.26 or newer. There are no other dependencies.

```console
go test -race ./...
go vet ./...
gofmt -l .        # must print nothing
go run ./cmd/cron-explain -tz UTC '*/15 * * * *'
```

Layout:

- `parse.go`: fields, ranges, steps, names and macros.
- `explain.go`: the English description.
- `next.go`: next run times and daylight saving handling.
- `check.go`: warnings.
- `cmd/cron-explain`: the command-line tool.

## Tests

- Every behaviour change needs a test. Bug fixes start with a test that fails on the old code.
- Time zone tests use fixed dates and zones whose rules for those dates are stable across tz database versions (for example America/Toronto in 2026). Do not depend on the current date.
- Keep expected English text in the tests exact, so wording changes are visible in review.

## Pull requests

- One topic per pull request, with a short description of what changed and why.
- CI must be green: tests on Linux and macOS, gofmt, vet, gitleaks and zizmor.
- Do not commit secrets or real personal data. Example files use made-up names and paths.

## License

By contributing you agree that your work is released under the MIT license of this project.
