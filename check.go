package cronexplain

import (
	"fmt"
	"time"
)

// Warning is a likely mistake or surprise in a schedule.
type Warning struct {
	// Code is a short stable name, for example "never-runs" or "dst-skip".
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Check looks for common mistakes: dates that never exist, the day of
// month / day of week OR rule, steps that do not divide their field, days
// missing in short months, and run times that daylight saving changes skip
// or repeat in loc during the year after from.
func (s *Schedule) Check(from time.Time, loc *time.Location) []Warning {
	if s.neverRuns(from) {
		return []Warning{{"never-runs", "This schedule never runs: no date matches every field (for example February 30, or day 31 in a month that has 30 days)."}}
	}
	var w []Warning
	domR, dowR := s.restricted(DayOfMonth), s.restricted(DayOfWeek)
	switch {
	case domR && dowR:
		w = append(w, Warning{"day-or-weekday", "Day of month and day of week are both set, so cron runs on days that match either one, not only days that match both."})
	case !s.isAll(DayOfMonth) && dowR, !s.isAll(DayOfWeek) && domR:
		w = append(w, Warning{"star-step-day", "One day field starts with \"*/\" and the other is set. Vixie cron and cronie require both to match (this tool does the same); some other schedulers, such as robfig/cron in Go, run when either matches."})
	}
	for _, f := range []Field{Second, Minute, Hour} {
		size := specs[f].max + 1
		for _, p := range s.parts[f] {
			if !p.star || p.step == 1 || size%p.step == 0 {
				continue
			}
			last := (size - 1) / p.step * p.step
			w = append(w, Warning{"uneven-step", fmt.Sprintf(
				"*/%d in the %s field restarts at 0: it runs at %d, %d, ... %d and then at 0, so the last gap is %d %ss, not %d.",
				p.step, f, 0, p.step, last, size-last, f, p.step)})
		}
	}
	if domR && !dowR {
		w = append(w, s.shortMonthWarnings()...)
	}
	return append(w, s.dstWarnings(from, loc)...)
}

func (s *Schedule) neverRuns(from time.Time) bool {
	day := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	for end := day.AddDate(searchYears, 0, 0); day.Before(end); day = day.AddDate(0, 0, 1) {
		if s.matchesDay(day) {
			return false
		}
	}
	return true
}

func (s *Schedule) shortMonthWarnings() []Warning {
	var w []Warning
	months := s.values(Month)
	for _, d := range s.values(DayOfMonth) {
		if d < 29 {
			continue
		}
		var missing []string
		leapOnly := false
		for _, m := range months {
			switch days := daysIn(m); {
			case m == 2 && d == 29:
				leapOnly = true
			case d > days:
				missing = append(missing, monthName(m))
			}
		}
		if len(missing) > 0 {
			w = append(w, Warning{"short-month", fmt.Sprintf("Day %d does not exist in %s, so there is no run on that day in those months.", d, joinAnd(missing))})
		}
		if leapOnly {
			w = append(w, Warning{"leap-day", "Day 29 of February exists only in leap years."})
		}
	}
	return w
}

// daysIn returns the most days a month can have (29 for February).
func daysIn(m int) int {
	return time.Date(2028, time.Month(m)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// dstWarnings reports run times on daylight saving change days during the
// year after from that do not exist or happen twice.
func (s *Schedule) dstWarnings(from time.Time, loc *time.Location) []Warning {
	var w []Warning
	seen := map[time.Time]bool{}
	t := from.In(loc)
	_, prev := t.Zone()
	for end := t.AddDate(1, 0, 0); t.Before(end); t = t.Add(time.Hour) {
		_, off := t.Zone()
		if off == prev {
			continue
		}
		prev = off
		day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		if seen[day] || !s.matchesDay(day) {
			continue
		}
		seen[day] = true
		var skipped, repeated []Occurrence
		for _, o := range s.onDay(day, loc) {
			switch o.Kind {
			case Skipped:
				skipped = append(skipped, o)
			case Repeated:
				repeated = append(repeated, o)
			}
		}
		date := day.Format(time.DateOnly)
		if len(skipped) > 0 {
			w = append(w, Warning{"dst-skip", fmt.Sprintf("On %s clocks in %s move forward: %s %s that day, so there is no run.",
				date, loc, span(skipped), verb(skipped, "does not exist", "do not exist"))})
		}
		if len(repeated) > 0 {
			w = append(w, Warning{"dst-repeat", fmt.Sprintf("On %s clocks in %s move back: %s %s twice; this tool counts one run each, at the first one (%s).",
				date, loc, span(repeated), verb(repeated, "happens", "happen"), repeated[0].Time.Format("MST"))})
		}
	}
	return w
}

func span(occ []Occurrence) string {
	first := occ[0].Wall.Format(time.TimeOnly)
	if len(occ) == 1 {
		return first
	}
	return fmt.Sprintf("%d run times from %s to %s", len(occ), first, occ[len(occ)-1].Wall.Format(time.TimeOnly))
}

func verb(occ []Occurrence, one, many string) string {
	if len(occ) == 1 {
		return one
	}
	return many
}
