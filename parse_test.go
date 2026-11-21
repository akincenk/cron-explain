package cronexplain

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseValues(t *testing.T) {
	tests := []struct {
		expr  string
		field Field
		want  []int
	}{
		{"*/15 * * * *", Minute, []int{0, 15, 30, 45}},
		{"5/20 * * * *", Minute, []int{5, 25, 45}},
		{"0 9-17/4 * * *", Hour, []int{9, 13, 17}},
		{"0 0 1,15,31 * *", DayOfMonth, []int{1, 15, 31}},
		{"0 0 * jan-mar,DEC *", Month, []int{1, 2, 3, 12}},
		{"0 0 * * MON-FRI", DayOfWeek, []int{1, 2, 3, 4, 5}},
		{"0 0 * * 5-7", DayOfWeek, []int{0, 5, 6}},
		{"0 0 * * 7", DayOfWeek, []int{0}},
		{"0 0 * * *", DayOfWeek, []int{0, 1, 2, 3, 4, 5, 6}},
		{"30 */10 * * * *", Second, []int{30}},
		{"30 */10 * * * *", Minute, []int{0, 10, 20, 30, 40, 50}},
		{"@hourly", Minute, []int{0}},
		{"@weekly", DayOfWeek, []int{0}},
	}
	for _, tt := range tests {
		s, err := Parse(tt.expr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.expr, err)
		}
		if got := s.values(tt.field); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Parse(%q) %s = %v, want %v", tt.expr, tt.field, got, tt.want)
		}
	}
}

func TestParseSecondsField(t *testing.T) {
	five, _ := Parse("* * * * *")
	six, _ := Parse("0 * * * * *")
	if five.HasSeconds() || !six.HasSeconds() {
		t.Fatalf("HasSeconds: five=%v six=%v", five.HasSeconds(), six.HasSeconds())
	}
	if got := five.values(Second); !reflect.DeepEqual(got, []int{0}) {
		t.Errorf("five-field seconds = %v, want [0]", got)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		expr, field, msg string
	}{
		{"* * * *", "", "expected 5 fields"},
		{"* * * * * * *", "", "got 7"},
		{"60 * * * *", "minute", "60 is out of range 0-59"},
		{"0 24 * * *", "hour", "24 is out of range 0-23"},
		{"0 0 0 * *", "day of month", "0 is out of range 1-31"},
		{"0 0 * 13 *", "month", "13 is out of range 1-12"},
		{"0 0 * * 8", "day of week", "8 is out of range 0-7"},
		{"0 0 * * FUN", "day of week", `"FUN" is not a number`},
		{"0 17-9 * * *", "hour", "starts after it ends"},
		{"*/0 * * * *", "minute", "at least 1"},
		{"*/61 * * * *", "minute", "larger than the field"},
		{"1,,2 * * * *", "minute", "empty item"},
		{"0 0 1- * *", "day of month", "missing value"},
		{"@every 5m", "", "unknown macro"},
	}
	for _, tt := range tests {
		_, err := Parse(tt.expr)
		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Fatalf("Parse(%q) error = %v, want *ParseError", tt.expr, err)
		}
		if pe.Field != tt.field || !strings.Contains(pe.Msg, tt.msg) {
			t.Errorf("Parse(%q) = field %q msg %q, want field %q msg containing %q", tt.expr, pe.Field, pe.Msg, tt.field, tt.msg)
		}
	}
}

func TestReboot(t *testing.T) {
	s, err := Parse("@REBOOT")
	if err != nil || !s.Reboot() {
		t.Fatalf("Parse(@REBOOT) = %v, %v", s, err)
	}
	if got := s.Explain(); got != "Once, when the cron daemon starts (usually at boot)." {
		t.Errorf("Explain = %q", got)
	}
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if len(s.Next(from, time.UTC, 3)) != 0 || len(s.Check(from, time.UTC)) != 0 {
		t.Error("@reboot should have no runs and no warnings")
	}
}

func TestRestrictedFollowsVixieCron(t *testing.T) {
	s, _ := Parse("0 0 */2 * 1")
	if s.restricted(DayOfMonth) {
		t.Error(`"*/2" in day of month should not count as restricted`)
	}
	if !s.restricted(DayOfWeek) {
		t.Error(`"1" in day of week should count as restricted`)
	}
}
