package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func runCLI(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := run(args, strings.NewReader(""), &out, &errOut, now)
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

func TestReboot(t *testing.T) {
	code, out, _ := runCLI("-tz", "UTC", "@reboot")
	if code != 0 || !strings.Contains(out, "none: @reboot is not tied to a time") {
		t.Errorf("exit %d, output:\n%s", code, out)
	}
}

func TestNeverRuns(t *testing.T) {
	code, out, _ := runCLI("-tz", "UTC", "0 0 30 2 *")
	if code != 0 || !strings.Contains(out, "none in the next 30 years") || !strings.Contains(out, "[never-runs]") {
		t.Errorf("exit %d, output:\n%s", code, out)
	}
}

func writeFile(t *testing.T, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "crontab")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFileText(t *testing.T) {
	p := writeFile(t, "# shop\n15 1 * * * /bin/rotate\nCRON_TZ=America/Toronto\n30 2 * * * /bin/backup\n61 * * * * /bin/bad\n")
	code, out, _ := runCLI("-tz", "UTC", "-n", "1", "-from", "2026-11-20 12:00", "-file", p)
	if code != 1 {
		t.Errorf("exit %d, want 1 because line 5 is bad", code)
	}
	for _, want := range []string{
		"Line 2: 15 1 * * *  /bin/rotate\nAt 01:15.\n\nNext runs (UTC):\n  2026-11-21 01:15:00 UTC (+00:00) Sat\n",
		"Line 4: 30 2 * * *  /bin/backup\nAt 02:30.\n\nNext runs (America/Toronto):\n  2026-11-21 02:30:00 EST (-05:00) Sat\n",
		"[dst-skip]",
		"Line 5: 61 * * * * /bin/bad\nError: cron expression",
		"\n3 job lines: 1 with warnings, 1 with errors.\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestFileJSONFromStdin(t *testing.T) {
	var out, errOut bytes.Buffer
	in := strings.NewReader("@reboot root /bin/start\nCRON_TZ=Mars/Base\n0 3 * * * backup /bin/dump\n")
	code := run([]string{"-json", "-system", "-n", "1", "-file", "-"}, in, &out, &errOut, now)
	if code != 1 {
		t.Errorf("exit %d, want 1 for the unknown zone", code)
	}
	var got []struct {
		Line                      int
		User, Command, Expression string
		Error                     string
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(got) != 2 {
		t.Fatalf("got %d entries\n%s", len(got), out.String())
	}
	if got[0].Line != 1 || got[0].User != "root" || got[0].Command != "/bin/start" || got[0].Error != "" || got[0].Expression != "@reboot" {
		t.Errorf("first = %+v\n%s", got[0], out.String())
	}
	if got[1].Line != 3 || got[1].User != "backup" || !strings.Contains(got[1].Error, `unknown time zone "Mars/Base"`) || got[1].Expression != "" {
		t.Errorf("second = %+v", got[1])
	}
}

func TestFileUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"-file", "x", "0 * * * *"},
		{"-system", "0 * * * *"},
		{"-file", filepath.Join(t.TempDir(), "missing")},
	} {
		if code, _, _ := runCLI(args...); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}
