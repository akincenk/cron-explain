package cronexplain

import (
	"testing"
	"time"
)

func mustLoad(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func mustParse(t *testing.T, expr string) *Schedule {
	t.Helper()
	s, err := Parse(expr)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func formatRuns(occ []Occurrence) []string {
	var out []string
	for _, o := range occ {
		if o.Kind == Skipped {
			out = append(out, "skip "+o.Wall.Format(time.DateTime))
			continue
		}
		out = append(out, o.Time.Format(time.RFC3339))
	}
	return out
}

func checkRuns(t *testing.T, expr string, from time.Time, loc *time.Location, n int, want []string) {
	t.Helper()
	got := formatRuns(mustParse(t, expr).Next(from, loc, n))
	if len(got) != len(want) {
		t.Fatalf("%s: got %d entries %v, want %v", expr, len(got), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s entry %d = %s, want %s", expr, i, got[i], want[i])
		}
	}
}

func TestNextBasic(t *testing.T) {
	utc := time.UTC
	from := time.Date(2026, 11, 20, 20, 7, 30, 0, utc)
	checkRuns(t, "*/15 * * * *", from, utc, 3, []string{
		"2026-11-20T20:15:00Z", "2026-11-20T20:30:00Z", "2026-11-20T20:45:00Z",
	})
	checkRuns(t, "0 9 * * MON-FRI", from, utc, 3, []string{
		"2026-11-23T09:00:00Z", "2026-11-24T09:00:00Z", "2026-11-25T09:00:00Z",
	})
	checkRuns(t, "15,45 * * * * *", from, utc, 2, []string{
		"2026-11-20T20:07:45Z", "2026-11-20T20:08:15Z",
	})
}

func TestNextIsStrictlyAfterFrom(t *testing.T) {
	from := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	checkRuns(t, "0 12 * * *", from, time.UTC, 1, []string{"2026-01-02T12:00:00Z"})
}

func TestNextDayOfMonthOrWeekday(t *testing.T) {
	// Both day fields restricted: runs on the 13th OR on Fridays.
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	checkRuns(t, "0 0 13 * 5", from, time.UTC, 3, []string{
		"2026-11-06T00:00:00Z", "2026-11-13T00:00:00Z", "2026-11-20T00:00:00Z",
	})
	// "*/2" starts with a star, so the fields are joined with AND, as in
	// Vixie cron: odd days of the month that are also Mondays.
	checkRuns(t, "0 0 */2 * 1", from, time.UTC, 2, []string{
		"2026-11-09T00:00:00Z", "2026-11-23T00:00:00Z",
	})
}

func TestNextLeapDay(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	checkRuns(t, "0 0 29 2 *", from, time.UTC, 2, []string{
		"2028-02-29T00:00:00Z", "2032-02-29T00:00:00Z",
	})
}

func TestNextNeverRuns(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if got := mustParse(t, "0 0 30 2 *").Next(from, time.UTC, 1); len(got) != 0 {
		t.Fatalf("February 30 should never run, got %v", formatRuns(got))
	}
}

func TestNextSpringForwardSkipsMissingTime(t *testing.T) {
	// Vancouver moved from 02:00 PST to 03:00 PDT on 2026-03-08.
	van := mustLoad(t, "America/Vancouver")
	from := time.Date(2026, 3, 7, 12, 0, 0, 0, van)
	checkRuns(t, "30 2 * * *", from, van, 2, []string{
		"skip 2026-03-08 02:30:00",
		"2026-03-09T02:30:00-07:00",
		"2026-03-10T02:30:00-07:00",
	})
}

func TestNextFallBackRunsOnce(t *testing.T) {
	// Toronto moved from 02:00 EDT back to 01:00 EST on 2026-11-01.
	// (Vancouver did not: British Columbia stays on UTC-7 from 2026.)
	tor := mustLoad(t, "America/Toronto")
	from := time.Date(2026, 10, 31, 12, 0, 0, 0, tor)
	occ := mustParse(t, "30 1 * * *").Next(from, tor, 2)
	if len(occ) != 2 {
		t.Fatalf("got %d runs", len(occ))
	}
	first := occ[0]
	if first.Kind != Repeated {
		t.Fatalf("kind = %v, want repeated", first.Kind)
	}
	if got := first.Time.Format(time.RFC3339); got != "2026-11-01T01:30:00-04:00" {
		t.Errorf("first instant = %s", got)
	}
	if got := first.Second.Format(time.RFC3339); got != "2026-11-01T01:30:00-05:00" {
		t.Errorf("second instant = %s", got)
	}
	if got := occ[1].Time.Format(time.RFC3339); got != "2026-11-02T01:30:00-05:00" {
		t.Errorf("next day = %s", got)
	}
}

func TestNextEveryMinuteAcrossFallBack(t *testing.T) {
	// Every 30 minutes through the repeated hour: 01:00 and 01:30 run once
	// each, at their first instant, then 02:00 PST follows.
	tor := mustLoad(t, "America/Toronto")
	from := time.Date(2026, 11, 1, 0, 45, 0, 0, tor)
	checkRuns(t, "*/30 * * * *", from, tor, 3, []string{
		"2026-11-01T01:00:00-04:00",
		"2026-11-01T01:30:00-04:00",
		"2026-11-01T02:00:00-05:00",
	})
}

func TestNextHalfHourZone(t *testing.T) {
	// Lord Howe Island moves its clocks by 30 minutes.
	lh := mustLoad(t, "Australia/Lord_Howe")
	from := time.Date(2026, 10, 3, 12, 0, 0, 0, lh)
	occ := mustParse(t, "15 2 * * *").Next(from, lh, 1)
	if len(occ) != 2 || occ[0].Kind != Skipped {
		t.Fatalf("want a skipped 02:15 on 2026-10-04 then a run, got %v", formatRuns(occ))
	}
}
