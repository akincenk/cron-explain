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

Explains a cron expression and lists its next run times.
EXPRESSION is 5 fields (minute hour day-of-month month day-of-week),
6 fields with seconds first, or a macro such as @daily. Quote it so the
shell does not expand "*".

Examples:
  cron-explain '*/15 9-17 * * 1-5'
  cron-explain -tz America/Toronto -n 3 '30 2 * * *'
  cron-explain -json '@weekly'

Flags:
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, time.Now()))
}

func run(args []string, stdout, stderr io.Writer, now time.Time) int {
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
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() == 0 {
		fs.Usage()
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

	// Joining lets an unquoted expression with no "*" still work: 0 9 1 1 1
	s, err := cronexplain.Parse(strings.Join(fs.Args(), " "))
	if err != nil {
		fmt.Fprintf(stderr, "cron-explain: %v\n", err)
		return 1
	}
	r := report{
		Expression:  s.String(),
		Description: s.Explain(),
		TimeZone:    zoneName(loc),
		Runs:        []runEntry{},
		Warnings:    s.Check(from, loc),
	}
	for _, o := range s.Next(from, loc, *n) {
		r.Runs = append(r.Runs, toRun(o))
	}
	if r.Warnings == nil {
		r.Warnings = []cronexplain.Warning{}
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(r); err != nil {
			fmt.Fprintf(stderr, "cron-explain: %v\n", err)
			return 1
		}
		return 0
	}
	printText(stdout, r, s, *n)
	return 0
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

func printText(w io.Writer, r report, s *cronexplain.Schedule, n int) {
	fmt.Fprintln(w, r.Expression)
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
