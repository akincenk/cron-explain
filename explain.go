package cronexplain

import (
	"fmt"
	"strings"
	"time"
)

// maxListedTimes is the most clock times Explain lists one by one before it
// describes the fields separately.
const maxListedTimes = 12

// Explain describes the schedule in one English sentence, for example
// "At 09:00, Monday through Friday." for "0 9 * * 1-5".
func (s *Schedule) Explain() string {
	if s.reboot {
		return "Once, when the cron daemon starts (usually at boot)."
	}
	day, month := s.dayPhrase(), s.monthPhrase()
	if d, m := s.values(DayOfMonth), s.values(Month); len(d) == 1 && len(m) == 1 && s.isAll(DayOfWeek) {
		day, month = fmt.Sprintf("on %s %d", monthName(m[0]), d[0]), "" // "on January 1"
	}
	var out []string
	for _, p := range []string{s.timePhrase(), day, month} {
		if p != "" {
			out = append(out, p)
		}
	}
	text := strings.Join(out, ", ")
	return strings.ToUpper(text[:1]) + text[1:] + "."
}

func (s *Schedule) isAll(f Field) bool {
	p := s.parts[f]
	return len(p) == 1 && p[0].star && p[0].step == 1
}

func (s *Schedule) allSingle(f Field) bool {
	for _, p := range s.parts[f] {
		if !p.single() {
			return false
		}
	}
	return true
}

func (s *Schedule) timePhrase() string {
	if s.allSingle(Second) && s.allSingle(Minute) && s.allSingle(Hour) {
		secs, mins, hours := s.values(Second), s.values(Minute), s.values(Hour)
		if len(secs)*len(mins)*len(hours) <= maxListedTimes {
			showSec := len(secs) > 1 || secs[0] != 0
			var times []string
			for _, h := range hours {
				for _, m := range mins {
					for _, sec := range secs {
						t := fmt.Sprintf("%02d:%02d", h, m)
						if showSec {
							t += fmt.Sprintf(":%02d", sec)
						}
						times = append(times, t)
					}
				}
			}
			return "at " + joinAnd(times)
		}
	}

	fields := []Field{Minute, Hour}
	if !(s.allSingle(Second) && len(s.values(Second)) == 1 && s.values(Second)[0] == 0) {
		fields = []Field{Second, Minute, Hour}
	}
	text := ""
	finerSingle := false
	for i, f := range fields {
		var phrase string
		switch {
		case s.isAll(f) && i == 0:
			phrase = "every " + specs[f].name
		case s.isAll(f) && finerSingle:
			text += " of every " + specs[f].name
			finerSingle = false
			continue
		case s.isAll(f):
			continue
		default:
			phrase = s.fieldPhrase(f)
		}
		if text != "" {
			text += ", "
		}
		text += phrase
		finerSingle = s.allSingle(f)
	}
	return text
}

// fieldPhrase describes a second, minute or hour field that is not "*".
func (s *Schedule) fieldPhrase(f Field) string {
	unit := specs[f].name
	var singles []string
	var others []string
	for _, p := range s.parts[f] {
		switch {
		case p.single() && f == Hour:
			singles = append(singles, fmt.Sprintf("%02d", p.lo))
		case p.single():
			singles = append(singles, fmt.Sprint(p.lo))
		case p.star:
			others = append(others, fmt.Sprintf("every %d %ss", p.step, unit))
		case f == Hour && p.step == 1:
			others = append(others, fmt.Sprintf("from %02d:00 through %02d:59", p.lo, p.hi))
		case f == Hour:
			others = append(others, fmt.Sprintf("every %d hours from %02d:00 through %02d:59", p.step, p.lo, p.hi))
		case p.step == 1:
			others = append(others, fmt.Sprintf("every %s from %d through %d", unit, p.lo, p.hi))
		default:
			others = append(others, fmt.Sprintf("every %d %ss from %d through %d", p.step, unit, p.lo, p.hi))
		}
	}
	var all []string
	if f == Hour && len(singles) == 1 && len(others) == 0 {
		return fmt.Sprintf("from %s:00 through %s:59", singles[0], singles[0])
	}
	if len(singles) > 0 {
		lead := "at " + unit
		if f == Hour {
			lead = "during hour"
		}
		if len(singles) > 1 {
			lead += "s"
		}
		all = append(all, lead+" "+joinAnd(singles))
	}
	return joinAnd(append(all, others...))
}

func (s *Schedule) dayPhrase() string {
	domAll, dowAll := s.isAll(DayOfMonth), s.isAll(DayOfWeek)
	if domAll && dowAll {
		return ""
	}
	var dom, dow string
	if !domAll {
		dom = s.domPhrase()
	}
	if !dowAll {
		dow = "on " + joinAnd(s.namedParts(DayOfWeek, weekdayName))
	}
	switch {
	case dom == "":
		return dow
	case dow == "":
		return dom
	case s.restricted(DayOfMonth) && s.restricted(DayOfWeek):
		return dom + " or " + dow + " (either one is enough)"
	}
	return dom + ", but only " + dow
}

func (s *Schedule) domPhrase() string {
	var singles, others []string
	for _, p := range s.parts[DayOfMonth] {
		switch {
		case p.single():
			singles = append(singles, fmt.Sprint(p.lo))
		case p.star:
			others = append(others, fmt.Sprintf("every %s day of the month", ordinal(p.step)))
		case p.step == 1:
			others = append(others, fmt.Sprintf("on days %d through %d of the month", p.lo, p.hi))
		default:
			others = append(others, fmt.Sprintf("every %s day from day %d through %d of the month", ordinal(p.step), p.lo, p.hi))
		}
	}
	var all []string
	if len(singles) == 1 {
		all = append(all, "on day "+singles[0]+" of the month")
	} else if len(singles) > 1 {
		all = append(all, "on days "+joinAnd(singles)+" of the month")
	}
	return joinAnd(append(all, others...))
}

func (s *Schedule) monthPhrase() string {
	if s.isAll(Month) {
		return ""
	}
	return "in " + joinAnd(s.namedParts(Month, monthName))
}

// namedParts renders month or weekday parts as names. Ranges read
// "Monday through Friday"; stepped parts are spelled out value by value.
func (s *Schedule) namedParts(f Field, name func(int) string) []string {
	var out []string
	for _, p := range s.parts[f] {
		switch {
		case p.single():
			out = append(out, name(p.lo))
		case p.step == 1 && !p.star:
			out = append(out, name(p.lo)+" through "+name(p.hi))
		default:
			for v := p.lo; v <= p.hi; v += p.step {
				out = append(out, name(v))
			}
		}
	}
	return out
}

func weekdayName(v int) string { return time.Weekday(v % 7).String() }
func monthName(v int) string   { return time.Month(v).String() }

func ordinal(n int) string {
	suffix := "th"
	switch {
	case n%100 >= 11 && n%100 <= 13:
	case n%10 == 1:
		suffix = "st"
	case n%10 == 2:
		suffix = "nd"
	case n%10 == 3:
		suffix = "rd"
	}
	return fmt.Sprintf("%d%s", n, suffix)
}

func joinAnd(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}
