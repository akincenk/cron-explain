package cronexplain

import (
	"fmt"
	"time"
)

// Kind tells how a matching wall-clock time maps to real time.
type Kind int

const (
	// Normal: the wall-clock time happens exactly once.
	Normal Kind = iota
	// Repeated: clocks were set back and the wall-clock time happens twice.
	// The schedule runs once, at the first of the two instants.
	Repeated
	// Skipped: clocks were set forward over this wall-clock time, so it does
	// not exist that day and the schedule does not run.
	Skipped
)

func (k Kind) String() string {
	switch k {
	case Repeated:
		return "repeated"
	case Skipped:
		return "skipped"
	}
	return "normal"
}

// Occurrence is one matching wall-clock time.
type Occurrence struct {
	// Wall is the matching local date and time, stored with time.UTC as its
	// location so that it can hold times that do not exist in the zone.
	Wall time.Time
	// Time is the instant the schedule runs. It is the zero time for Skipped.
	Time time.Time
	// Second is the later instant of a Repeated wall-clock time.
	Second time.Time
	Kind   Kind
}

// searchYears bounds the search for the next run. It covers leap days and
// the 28-year cycle in which every date falls on every weekday.
const searchYears = 30

// Next returns the next n runs strictly after from, in loc. Wall-clock times
// skipped by a daylight saving change are included as Skipped entries and do
// not count toward n. It returns fewer than n runs when the expression has no
// more matches within 30 years, for example "0 0 30 2 *".
func (s *Schedule) Next(from time.Time, loc *time.Location, n int) []Occurrence {
	var out []Occurrence
	if n <= 0 {
		return out
	}
	from = from.In(loc)
	fromWall := wallOf(from)
	day := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	end := day.AddDate(searchYears, 0, 0)
	runs := 0
	for ; day.Before(end); day = day.AddDate(0, 0, 1) {
		if !s.matchesDay(day) {
			continue
		}
		for _, o := range s.onDay(day, loc) {
			if !o.Wall.After(fromWall) && o.Kind == Skipped {
				continue
			}
			if o.Kind != Skipped && !o.Time.After(from) {
				continue
			}
			out = append(out, o)
			if o.Kind != Skipped {
				runs++
				if runs == n {
					return out
				}
			}
		}
	}
	return out
}

// matchesDay applies the cron day rule: when both day fields are restricted
// a day matches if either one does, otherwise both must match.
func (s *Schedule) matchesDay(day time.Time) bool {
	if !s.has(Month, int(day.Month())) {
		return false
	}
	dom := s.has(DayOfMonth, day.Day())
	dow := s.has(DayOfWeek, int(day.Weekday()))
	if s.restricted(DayOfMonth) && s.restricted(DayOfWeek) {
		return dom || dow
	}
	return dom && dow
}

// onDay lists every matching time on one local date, in order.
func (s *Schedule) onDay(day time.Time, loc *time.Location) []Occurrence {
	var out []Occurrence
	for _, h := range s.values(Hour) {
		for _, m := range s.values(Minute) {
			for _, sec := range s.values(Second) {
				wall := day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second)
				out = append(out, resolve(wall, loc))
			}
		}
	}
	return out
}

// resolve maps a wall-clock time to the instants that show it in loc. Go's
// time.Date picks one instant for missing or doubled times without saying
// so, so this tries the zone offsets in force a day and a half on either
// side and keeps the ones that give back the same wall-clock time.
func resolve(wall time.Time, loc *time.Location) Occurrence {
	o := Occurrence{Wall: wall}
	var found []time.Time
	for _, probe := range []time.Duration{-36 * time.Hour, 0, 36 * time.Hour} {
		_, offset := wall.Add(probe).In(loc).Zone()
		t := wall.Add(-time.Duration(offset) * time.Second).In(loc)
		if !wallOf(t).Equal(wall) {
			continue
		}
		dup := false
		for _, f := range found {
			dup = dup || f.Equal(t)
		}
		if !dup {
			found = append(found, t)
		}
	}
	switch len(found) {
	case 0:
		o.Kind = Skipped
	case 1:
		o.Time = found[0]
	default:
		a, b := found[0], found[1]
		if b.Before(a) {
			a, b = b, a
		}
		o.Kind, o.Time, o.Second = Repeated, a, b
	}
	return o
}

func wallOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
}

// Describe returns a one-line description of the occurrence, such as
// "2026-03-08 02:30:00 skipped: clocks move forward over this time".
func (o Occurrence) Describe() string {
	switch o.Kind {
	case Skipped:
		return fmt.Sprintf("%s skipped: clocks move forward over this time", o.Wall.Format(time.DateTime))
	case Repeated:
		return fmt.Sprintf("%s runs once; this time happens again at %s when clocks move back",
			o.Time.Format("2006-01-02 15:04:05 MST (-07:00)"), o.Second.Format("15:04:05 MST (-07:00)"))
	}
	return o.Time.Format("2006-01-02 15:04:05 MST (-07:00) Mon")
}
