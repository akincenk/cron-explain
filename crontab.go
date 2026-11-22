package cronexplain

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// CrontabEntry is one job line of a crontab file.
type CrontabEntry struct {
	Line    int    // 1-based line number in the file
	Text    string // the line as written, without leading and trailing blanks
	Expr    string // the schedule part, such as "*/5 * * * *" or "@daily"
	User    string // user column; only set for system crontabs
	Command string
	// TZ is the value of CRON_TZ in effect for this line, or "" when the
	// file has not set it. Like cronie, only CRON_TZ changes when a job
	// runs; TZ is passed to the command and is not used for the schedule.
	TZ       string
	Schedule *Schedule // nil when Err is set
	Err      error
}

// timeLike matches a token that can only be a cron time field.
var timeLike = regexp.MustCompile(`^[0-9*][0-9*,/-]*$`)

// envLine matches "name = value"; spaces around "=" are optional.
var envLine = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$`)

// ParseCrontab reads a crontab file and returns its job lines in order.
// Blank lines and lines whose first non-blank character is "#" are
// skipped, and "name = value" lines set variables. With system set, each
// job line has a user column after the schedule, as in /etc/crontab and
// /etc/cron.d. A job line that cannot be read is returned with Err set;
// the returned error is only for problems reading r.
func ParseCrontab(r io.Reader, system bool) ([]CrontabEntry, error) {
	var entries []CrontabEntry
	tz := ""
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if m := envLine.FindStringSubmatch(line); m != nil {
			if m[1] == "CRON_TZ" {
				tz = unquote(m[2])
			}
			continue
		}
		entries = append(entries, parseJobLine(n, line, tz, system))
	}
	return entries, sc.Err()
}

func parseJobLine(n int, line, tz string, system bool) CrontabEntry {
	e := CrontabEntry{Line: n, Text: line, TZ: tz}
	timeFields := 5
	if strings.HasPrefix(line, "@") {
		timeFields = 1
	}
	want := timeFields + 1 // and a command
	if system {
		want++
	}
	tokens, rest := splitFields(line, want-1)
	if len(tokens) < want-1 || rest == "" {
		e.Expr = strings.Join(tokens, " ")
		what := "schedule and a command"
		if system {
			what = "schedule, a user and a command"
		}
		e.Err = fmt.Errorf("line %d: expected a %s", n, what)
		if system && len(tokens) > timeFields && strings.HasPrefix(tokens[len(tokens)-1], "/") {
			e.Err = fmt.Errorf("line %d: expected a %s; is the user column missing?", n, what)
		}
		return e
	}
	e.Expr = strings.Join(tokens[:timeFields], " ")
	if system {
		e.User = tokens[timeFields]
	}
	e.Command = rest
	// A seconds field (six fields) shifts everything one column right.
	next := e.Command
	if system {
		next = e.User
	}
	if system && strings.HasPrefix(e.User, "/") {
		e.Err = fmt.Errorf("line %d: user %q looks like a command; is the user column missing?", n, e.User)
		return e
	}
	if first, _ := splitFields(next, 1); timeLike.MatchString(first[0]) {
		e.Err = fmt.Errorf("line %d: %q after the schedule looks like a time field; crontab files take 5 time fields (no seconds)", n, first[0])
		return e
	}
	s, err := Parse(e.Expr)
	if err != nil {
		e.Err = fmt.Errorf("line %d: %w", n, err)
		return e
	}
	e.Schedule = s
	return e
}

// splitFields returns the first k blank-separated fields of line and the
// rest of the line with its inner spacing kept.
func splitFields(line string, k int) ([]string, string) {
	var tokens []string
	rest := strings.TrimSpace(line)
	for len(tokens) < k && rest != "" {
		i := strings.IndexAny(rest, " \t")
		if i < 0 {
			tokens = append(tokens, rest)
			return tokens, ""
		}
		tokens = append(tokens, rest[:i])
		rest = strings.TrimLeft(rest[i:], " \t")
	}
	return tokens, rest
}

// unquote removes one pair of matching single or double quotes.
func unquote(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1]
	}
	return v
}
