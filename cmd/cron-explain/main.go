// Command cron-explain prints a plain-English description of a cron
// expression, its next run times in a time zone, and warnings.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	_ "time/tzdata" // fallback zone data for systems without it

	cronexplain "github.com/akincenk/cron-explain"
)

const usage = `Usage: cron-explain [flags] EXPRESSION
       cron-explain [flags] -file CRONTAB

Explains a cron expression and lists its next run times.
EXPRESSION is 5 fields (minute hour day-of-month month day-of-week),
6 fields with seconds first, or a macro such as @daily. Quote it so the
shell does not expand "*". With -file, every job line of a crontab is
checked; CRON_TZ lines in the file override -tz for the lines after them.

Examples:
  cron-explain '*/15 9-17 * * 1-5'
  cron-explain -tz America/Toronto -n 3 '30 2 * * *'
  cron-explain -json '@weekly'
  cron-explain -n 1 -file mycrontab
  cron-explain -system -file /etc/cron.d/backup

Flags:
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, time.Now()))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, now time.Time) int {
	fs := flag.NewFlagSet("cron-explain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, usage)
		fs.PrintDefaults()
	}
	tzName := fs.String("tz", "", "IANA time zone for run times, such as Europe/Berlin (default: this computer's zone)")
	n := fs.Int("n", 5, "how many next runs to list (0 to list none)")
	fromText := fs.String("from", "", `start listing after this time: RFC 3339 ("2026-03-08T00:00:00-08:00") or "2006-01-02 15:04" in the -tz zone (default: now)`)
	asJSON := fs.Bool("json", false, "print JSON instead of text")
	file := fs.String("file", "", `check every job line of this crontab file ("-" reads standard input)`)
	system := fs.Bool("system", false, "with -file: lines have a user column after the schedule, as in /etc/crontab and /etc/cron.d")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if (fs.NArg() == 0) == (*file == "") {
		fs.Usage()
		return 2
	}
	if *system && *file == "" {
		fmt.Fprintln(stderr, "cron-explain: -system needs -file")
		return 2
	}
	if *n < 0 || *n > 1000 {
		fmt.Fprintln(stderr, "cron-explain: -n must be between 0 and 1000")
		return 2
	}
	loc := time.Local
	if *tzName != "" {
		var err error
		if loc, err = time.LoadLocation(*tzName); err != nil {
			fmt.Fprintf(stderr, "cron-explain: unknown time zone %q (use an IANA name such as America/Vancouver)\n", *tzName)
			return 2
		}
	}
	from := now
	if *fromText != "" {
		var err error
		if from, err = parseFrom(*fromText, loc); err != nil {
			fmt.Fprintf(stderr, "cron-explain: -from: %v\n", err)
			return 2
		}
	}

	if *file != "" {
		return runFile(*file, *system, stdin, stdout, stderr, loc, from, *n, *asJSON)
	}

	// Joining lets an unquoted expression with no "*" still work: 0 9 1 1 1
	s, err := cronexplain.Parse(strings.Join(fs.Args(), " "))
	if err != nil {
		fmt.Fprintf(stderr, "cron-explain: %v\n", err)
		return 1
	}
	r := newReport(s, loc, from, *n)
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(r); err != nil {
			fmt.Fprintf(stderr, "cron-explain: %v\n", err)
			return 1
		}
		return 0
	}
	printText(stdout, r.Expression, *r, s, *n)
	return 0
}

func newReport(s *cronexplain.Schedule, loc *time.Location, from time.Time, n int) *report {
	r := &report{
		Expression:  s.String(),
		Description: s.Explain(),
		TimeZone:    zoneName(loc),
		Runs:        []runEntry{},
		Warnings:    s.Check(from, loc),
	}
	for _, o := range s.Next(from, loc, n) {
		r.Runs = append(r.Runs, toRun(o))
	}
	if r.Warnings == nil {
		r.Warnings = []cronexplain.Warning{}
	}
	return r
}

type fileEntry struct {
	Line    int    `json:"line"`
	User    string `json:"user,omitempty"`
	Command string `json:"command,omitempty"`
	*report
	Error string `json:"error,omitempty"`
}

// runFile checks every job line of a crontab. It exits 1 when any line
// has an error and still reports the other lines.
func runFile(path string, system bool, stdin io.Reader, stdout, stderr io.Writer, loc *time.Location, from time.Time, n int, asJSON bool) int {
	in := stdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(stderr, "cron-explain: %v\n", err)
			return 2
		}
		defer f.Close()
		in = f
	}
	entries, err := cronexplain.ParseCrontab(in, system)
	if err != nil {
		fmt.Fprintf(stderr, "cron-explain: reading %s: %v\n", path, err)
		return 2
	}
	zones := map[string]*time.Location{"": loc}
	out := []fileEntry{}
	code := 0
	for i, e := range entries {
		fe := fileEntry{Line: e.Line, User: e.User, Command: e.Command}
		z, ok := zones[e.TZ]
		if !ok {
			if z, err = time.LoadLocation(e.TZ); err != nil {
				z = nil
			}
			zones[e.TZ] = z
		}
		switch {
		case e.Err != nil:
			fe.Error = e.Err.Error()
		case z == nil:
			fe.Error = fmt.Sprintf("line %d: unknown time zone %q in CRON_TZ", e.Line, e.TZ)
		default:
			fe.report = newReport(e.Schedule, z, from, n)
		}
		if fe.Error != "" {
			code = 1
		}
		out = append(out, fe)
		if asJSON {
			continue
		}
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		title := fmt.Sprintf("Line %d: %s", e.Line, e.Expr)
		if e.User != "" {
			title += "  (user " + e.User + ")"
		}
		if e.Command != "" {
			title += "  " + e.Command
		}
		if fe.Error != "" {
			fmt.Fprintf(stdout, "Line %d: %s\n", e.Line, e.Text)
			fmt.Fprintln(stdout, "Error: "+strings.TrimPrefix(fe.Error, fmt.Sprintf("line %d: ", e.Line)))
			continue
		}
		printText(stdout, title, *fe.report, e.Schedule, n)
	}
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(stderr, "cron-explain: %v\n", err)
			return 1
		}
	} else {
		fmt.Fprintln(stdout, "\n"+summary(out))
	}
	return code
}

// zoneName returns the IANA name of loc. For time.Local Go only says
// "Local", so it looks at $TZ and the /etc/localtime link instead.
func zoneName(loc *time.Location) string {
	if loc != time.Local {
		return loc.String()
	}
	if tz := os.Getenv("TZ"); tz != "" {
		return strings.TrimPrefix(tz, ":")
	}
	if target, err := os.Readlink("/etc/localtime"); err == nil {
		if _, name, ok := strings.Cut(target, "zoneinfo/"); ok {
			return name
		}
	}
	return "Local"
}

func parseFrom(text string, loc *time.Location) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, text); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", text, loc); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("%q is not RFC 3339 or \"YYYY-MM-DD HH:MM\"", text)
}

type report struct {
	Expression  string                `json:"expression"`
	Description string                `json:"description"`
	TimeZone    string                `json:"time_zone"`
	Runs        []runEntry            `json:"runs"`
	Warnings    []cronexplain.Warning `json:"warnings"`
}

type runEntry struct {
	Kind string `json:"kind"`
	// Wall is the local date and time that matched, without an offset.
	Wall string `json:"wall"`
	// Time is when it runs; empty for "skipped".
	Time string `json:"time,omitempty"`
	// RepeatsAt is the second instant of a "repeated" wall-clock time.
	RepeatsAt string `json:"repeats_at,omitempty"`
	text      string
}

func toRun(o cronexplain.Occurrence) runEntry {
	r := runEntry{Kind: o.Kind.String(), Wall: o.Wall.Format("2006-01-02T15:04:05"), text: o.Describe()}
	if o.Kind != cronexplain.Skipped {
		r.Time = o.Time.Format(time.RFC3339)
	}
	if o.Kind == cronexplain.Repeated {
		r.RepeatsAt = o.Second.Format(time.RFC3339)
	}
	return r
}

func summary(out []fileEntry) string {
	warned, failed := 0, 0
	for _, e := range out {
		switch {
		case e.Error != "":
			failed++
		case len(e.Warnings) > 0:
			warned++
		}
	}
	lines := "job lines"
	if len(out) == 1 {
		lines = "job line"
	}
	return fmt.Sprintf("%d %s: %d with warnings, %d with errors.", len(out), lines, warned, failed)
}

func printText(w io.Writer, title string, r report, s *cronexplain.Schedule, n int) {
	fmt.Fprintln(w, title)
	fmt.Fprintln(w, r.Description)
	if n > 0 {
		fmt.Fprintf(w, "\nNext runs (%s):\n", r.TimeZone)
		if s.Reboot() {
			fmt.Fprintln(w, "  none: @reboot is not tied to a time")
		} else if len(r.Runs) == 0 {
			fmt.Fprintln(w, "  none in the next 30 years")
		}
		for _, x := range r.Runs {
			fmt.Fprintln(w, "  "+x.text)
		}
	}
	if len(r.Warnings) > 0 {
		fmt.Fprintln(w, "\nWarnings:")
		for _, x := range r.Warnings {
			fmt.Fprintf(w, "  [%s] %s\n", x.Code, x.Message)
		}
	}
}
