// Package cronexplain parses cron expressions, describes them in plain
// English, lists their next run times in a time zone, and warns about
// common mistakes.
package cronexplain

import (
	"fmt"
	"strconv"
	"strings"
)

// Field identifies one position of a cron expression.
type Field int

// Fields in the order they appear in a six-field expression.
const (
	Second Field = iota
	Minute
	Hour
	DayOfMonth
	Month
	DayOfWeek
)

type fieldSpec struct {
	name     string
	min, max int
	names    map[string]int
}

var monthNames = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
	"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

var dayNames = map[string]int{
	"SUN": 0, "MON": 1, "TUE": 2, "WED": 3, "THU": 4, "FRI": 5, "SAT": 6,
}

var specs = [...]fieldSpec{
	Second:     {"second", 0, 59, nil},
	Minute:     {"minute", 0, 59, nil},
	Hour:       {"hour", 0, 23, nil},
	DayOfMonth: {"day of month", 1, 31, nil},
	Month:      {"month", 1, 12, monthNames},
	DayOfWeek:  {"day of week", 0, 7, dayNames},
}

func (f Field) String() string { return specs[f].name }

// part is one comma-separated item of a field: a single value, a range,
// or a stepped range. A star is a range over the whole field with star set.
type part struct {
	lo, hi, step int
	star         bool
}

func (p part) single() bool { return !p.star && p.lo == p.hi }

// Schedule is a parsed cron expression.
type Schedule struct {
	expr       string
	hasSeconds bool
	parts      [6][]part
	bits       [6]uint64
}

// ParseError reports a problem with one field of an expression.
type ParseError struct {
	Expr  string
	Field string // empty when the problem is not tied to one field
	Msg   string
}

func (e *ParseError) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("cron expression %q: %s", e.Expr, e.Msg)
	}
	return fmt.Sprintf("cron expression %q: %s field: %s", e.Expr, e.Field, e.Msg)
}

var macros = map[string]string{
	"@yearly":   "0 0 1 1 *",
	"@annually": "0 0 1 1 *",
	"@monthly":  "0 0 1 * *",
	"@weekly":   "0 0 * * 0",
	"@daily":    "0 0 * * *",
	"@midnight": "0 0 * * *",
	"@hourly":   "0 * * * *",
}

// Parse reads a standard five-field expression (minute hour day-of-month
// month day-of-week), a six-field expression with seconds in front, or one
// of the macros @yearly, @annually, @monthly, @weekly, @daily, @midnight and
// @hourly. Month and weekday names (JAN, MON) are accepted in any case, and
// both 0 and 7 mean Sunday.
func Parse(expr string) (*Schedule, error) {
	s := &Schedule{expr: strings.TrimSpace(expr)}
	text := s.expr
	if strings.HasPrefix(text, "@") {
		m, ok := macros[strings.ToLower(text)]
		if !ok {
			return nil, &ParseError{Expr: expr, Msg: fmt.Sprintf("unknown macro %s (known: @yearly, @annually, @monthly, @weekly, @daily, @midnight, @hourly)", text)}
		}
		text = m
	}
	fields := strings.Fields(text)
	switch len(fields) {
	case 5:
		fields = append([]string{"0"}, fields...)
	case 6:
		s.hasSeconds = true
	default:
		return nil, &ParseError{Expr: expr, Msg: fmt.Sprintf("expected 5 fields (or 6 with seconds), got %d", len(fields))}
	}
	for i, text := range fields {
		f := Field(i)
		parts, err := parseField(text, specs[f])
		if err != nil {
			return nil, &ParseError{Expr: expr, Field: f.String(), Msg: err.Error()}
		}
		s.parts[f] = parts
		for _, p := range parts {
			for v := p.lo; v <= p.hi; v += p.step {
				if f == DayOfWeek && v == 7 {
					s.bits[f] |= 1 // 7 is Sunday, like 0
					break
				}
				s.bits[f] |= 1 << uint(v)
			}
		}
	}
	return s, nil
}

func parseField(text string, spec fieldSpec) ([]part, error) {
	var parts []part
	for _, item := range strings.Split(text, ",") {
		if item == "" {
			return nil, fmt.Errorf("empty item in %q", text)
		}
		p, err := parseItem(item, spec)
		if err != nil {
			return nil, err
		}
		parts = append(parts, p)
	}
	return parts, nil
}

func parseItem(item string, spec fieldSpec) (part, error) {
	rangeText, stepText, hasStep := strings.Cut(item, "/")
	p := part{step: 1}
	switch {
	case rangeText == "*":
		p.lo, p.hi, p.star = spec.min, spec.max, true
		if spec.name == "day of week" {
			p.hi = 6
		}
	default:
		loText, hiText, isRange := strings.Cut(rangeText, "-")
		lo, err := parseValue(loText, spec)
		if err != nil {
			return p, err
		}
		p.lo, p.hi = lo, lo
		if isRange {
			hi, err := parseValue(hiText, spec)
			if err != nil {
				return p, err
			}
			if hi < lo {
				return p, fmt.Errorf("range %s starts after it ends", rangeText)
			}
			p.hi = hi
		} else if hasStep {
			// "5/15" means "5-max/15", as in most cron implementations.
			p.hi = spec.max
			if spec.name == "day of week" {
				p.hi = 6
			}
		}
	}
	if hasStep {
		step, err := strconv.Atoi(stepText)
		if err != nil || step < 1 {
			return p, fmt.Errorf("step %q must be a whole number of at least 1", stepText)
		}
		if step > spec.max-spec.min+1 {
			return p, fmt.Errorf("step %d is larger than the field (%d-%d)", step, spec.min, spec.max)
		}
		p.step = step
		if !p.star && p.lo == p.hi {
			return p, fmt.Errorf("step on a single value %s has no effect; use a range like %d-%d/%d", item, p.lo, spec.max, step)
		}
	}
	return p, nil
}

func parseValue(text string, spec fieldSpec) (int, error) {
	if text == "" {
		return 0, fmt.Errorf("missing value")
	}
	if v, ok := spec.names[strings.ToUpper(text)]; ok {
		return v, nil
	}
	v, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("%q is not a number or a known name", text)
	}
	if v < spec.min || v > spec.max {
		return 0, fmt.Errorf("%d is out of range %d-%d", v, spec.min, spec.max)
	}
	return v, nil
}

// String returns the expression as it was given (trimmed).
func (s *Schedule) String() string { return s.expr }

// HasSeconds reports whether the expression had a seconds field.
func (s *Schedule) HasSeconds() bool { return s.hasSeconds }

func (s *Schedule) has(f Field, v int) bool { return s.bits[f]&(1<<uint(v)) != 0 }

// restricted reports whether a day field counts as limiting the days. When
// both day fields do, cron runs on days matching either one (OR). Like
// Vixie cron and cronie, a field that starts with "*" (including "*/2")
// does not count as limiting.
func (s *Schedule) restricted(f Field) bool {
	return !s.parts[f][0].star
}

func (s *Schedule) values(f Field) []int {
	var out []int
	for v := specs[f].min; v <= specs[f].max; v++ {
		if s.has(f, v) {
			out = append(out, v)
		}
	}
	return out
}
