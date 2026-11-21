package cronexplain

import (
	"strings"
	"testing"
	"time"
)

func codes(w []Warning) string {
	var c []string
	for _, x := range w {
		c = append(c, x.Code)
	}
	return strings.Join(c, ",")
}

func TestCheckCodes(t *testing.T) {
	from := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	tests := []struct{ expr, want string }{
		{"0 9 * * 1-5", ""},
		{"*/15 * * * *", ""},
		{"0 0 30 2 *", "never-runs"},
		{"0 0 31 4,6 *", "never-runs"},
		{"0 0 1 * 1", "day-or-weekday"},
		{"0 0 */2 * 1", "star-step-day"},
		{"*/7 * * * *", "uneven-step"},
		{"0 */5 * * *", "uneven-step"},
		{"*/25 * * * * *", "uneven-step"},
		{"0 0 31 * *", "short-month"},
		{"0 0 29 * *", "leap-day"},
		{"0 0 30 1-3 *", "short-month"},
	}
	for _, tt := range tests {
		if got := codes(mustParse(t, tt.expr).Check(from, time.UTC)); got != tt.want {
			t.Errorf("Check(%q) = %q, want %q", tt.expr, got, tt.want)
		}
	}
}

func TestCheckMessages(t *testing.T) {
	from := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	w := mustParse(t, "*/7 * * * *").Check(from, time.UTC)
	want := "*/7 in the minute field restarts at 0: it runs at 0, 7, ... 56 and then at 0, so the last gap is 4 minutes, not 7."
	if len(w) != 1 || w[0].Message != want {
		t.Errorf("got %+v\nwant %q", w, want)
	}
	w = mustParse(t, "0 0 31 * *").Check(from, time.UTC)
	want = "Day 31 does not exist in February, April, June, September and November, so there is no run on that day in those months."
	if len(w) != 1 || w[0].Message != want {
		t.Errorf("got %+v\nwant %q", w, want)
	}
}

func TestCheckDST(t *testing.T) {
	tor := mustLoad(t, "America/Toronto")
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, tor)
	w := mustParse(t, "30 2 * * *").Check(from, tor)
	if codes(w) != "dst-skip" {
		t.Fatalf("02:30 daily from June 2026: got %+v", w)
	}
	if !strings.Contains(w[0].Message, "On 2027-03-14 clocks in America/Toronto move forward: 02:30:00 does not exist") {
		t.Errorf("message = %q", w[0].Message)
	}

	w = mustParse(t, "*/15 1 * * *").Check(from, tor)
	if codes(w) != "dst-repeat" || !strings.Contains(w[0].Message, "4 run times from 01:00:00 to 01:45:00 happen twice") {
		t.Errorf("01:xx on fall-back day: got %+v", w)
	}

	// Days that do not match the schedule are not reported.
	if w := mustParse(t, "30 2 * * 1-5").Check(from, tor); len(w) != 0 {
		t.Errorf("weekday-only schedule should miss the Sunday changes, got %+v", w)
	}
	// Times away from the change are fine.
	if w := mustParse(t, "0 9 * * *").Check(from, tor); len(w) != 0 {
		t.Errorf("09:00 is never skipped, got %+v", w)
	}
}
