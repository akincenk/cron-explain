package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func runCLI(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut, now)
	return code, out.String(), errOut.String()
}

func TestTextOutput(t *testing.T) {
	code, out, _ := runCLI("-tz", "America/Toronto", "-from", "2026-10-31 00:00", "-n", "3", "30 1 * * *")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{
		"At 01:30.",
		"Next runs (America/Toronto):",
		"2026-10-31 01:30:00 EDT (-04:00) Sat",
		"2026-11-01 01:30:00 EDT (-04:00) runs once; this time happens again at 01:30:00 EST (-05:00)",
		"2026-11-02 01:30:00 EST (-05:00) Mon",
		"[dst-repeat] On 2026-11-01",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestUnquotedFieldsAreJoined(t *testing.T) {
	code, out, _ := runCLI("-tz", "UTC", "-n", "1", "0", "9", "1", "1", "1")
	if code != 0 || !strings.Contains(out, "0 9 1 1 1\n") {
		t.Fatalf("exit %d, output:\n%s", code, out)
	}
}

func TestJSONOutput(t *testing.T) {
	code, out, _ := runCLI("-json", "-tz", "America/Vancouver", "-from", "2026-03-07T12:00:00-08:00", "-n", "1", "30 2 * * *")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	var r report
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if r.TimeZone != "America/Vancouver" || r.Description != "At 02:30." {
		t.Errorf("header = %+v", r)
	}
	if len(r.Runs) != 2 || r.Runs[0].Kind != "skipped" || r.Runs[0].Time != "" || r.Runs[0].Wall != "2026-03-08T02:30:00" {
		t.Fatalf("runs = %+v", r.Runs)
	}
	if r.Runs[1].Time != "2026-03-09T02:30:00-07:00" {
		t.Errorf("second entry = %+v", r.Runs[1])
	}
}

func TestJSONEmptyListsAreArrays(t *testing.T) {
	_, out, _ := runCLI("-json", "-tz", "UTC", "-n", "0", "@daily")
	if !strings.Contains(out, `"runs": []`) || !strings.Contains(out, `"warnings": []`) {
		t.Errorf("want empty arrays, got:\n%s", out)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		args []string
		code int
		msg  string
	}{
		{[]string{}, 2, "Usage:"},
		{[]string{"61 * * * *"}, 1, "minute field: 61 is out of range 0-59"},
		{[]string{"-tz", "Mars/Base", "@daily"}, 2, `unknown time zone "Mars/Base"`},
		{[]string{"-n", "-1", "@daily"}, 2, "-n must be between 0 and 1000"},
		{[]string{"-from", "yesterday", "@daily"}, 2, `"yesterday" is not RFC 3339`},
		{[]string{"-bogus"}, 2, "flag provided but not defined"},
	}
	for _, tt := range tests {
		code, _, errOut := runCLI(tt.args...)
		if code != tt.code || !strings.Contains(errOut, tt.msg) {
			t.Errorf("%v: exit %d, stderr %q; want exit %d and %q", tt.args, code, errOut, tt.code, tt.msg)
		}
	}
}

func TestNeverRuns(t *testing.T) {
	code, out, _ := runCLI("-tz", "UTC", "0 0 30 2 *")
	if code != 0 || !strings.Contains(out, "none in the next 30 years") || !strings.Contains(out, "[never-runs]") {
		t.Errorf("exit %d, output:\n%s", code, out)
	}
}
