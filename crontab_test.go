package cronexplain

import (
	"strings"
	"testing"
)

func TestParseCrontab(t *testing.T) {
	in := `# comment
   # indented comment

MAILTO = "ops@example.com"
*/5 * * * *   /bin/sync  --fast   now
CRON_TZ = 'Asia/Tokyo'
@daily	echo "a  b"
TZ=Europe/Berlin
0 9 * * mon /bin/report # not a comment
CRON_TZ=
30 2 * * * /bin/backup
`
	entries, err := ParseCrontab(strings.NewReader(in), false)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		line          int
		expr, cmd, tz string
	}{
		{5, "*/5 * * * *", "/bin/sync  --fast   now", ""},
		{7, "@daily", `echo "a  b"`, "Asia/Tokyo"},
		{9, "0 9 * * mon", "/bin/report # not a comment", "Asia/Tokyo"}, // TZ does not change the schedule
		{11, "30 2 * * *", "/bin/backup", ""},
	}
	if len(entries) != len(want) {
		t.Fatalf("got %d entries: %+v", len(entries), entries)
	}
	for i, w := range want {
		e := entries[i]
		if e.Err != nil || e.Schedule == nil {
			t.Fatalf("line %d: %v", e.Line, e.Err)
		}
		if e.Line != w.line || e.Expr != w.expr || e.Command != w.cmd || e.TZ != w.tz || e.User != "" {
			t.Errorf("entry %d = %+v, want %+v", i, e, w)
		}
	}
}

func TestParseCrontabSystem(t *testing.T) {
	in := "0 3 * * * backup /bin/dump --all\n@hourly root /bin/x\n"
	entries, err := ParseCrontab(strings.NewReader(in), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].User != "backup" || entries[0].Command != "/bin/dump --all" ||
		entries[1].Expr != "@hourly" || entries[1].User != "root" || entries[1].Command != "/bin/x" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestParseCrontabErrors(t *testing.T) {
	cases := []struct {
		system     bool
		line, want string
	}{
		{false, "0 9 * *", "line 1: expected a schedule and a command"},
		{false, "0 9 * * *", "line 1: expected a schedule and a command"},
		{false, "@daily", "line 1: expected a schedule and a command"},
		{false, "0 30 2 * * * /bin/x", `line 1: "*" after the schedule looks like a time field`},
		{false, "61 * * * * /bin/x", "line 1: cron expression"},
		{false, "@nightly /bin/x", "unknown macro @nightly"},
		{true, "0 5 * * 1-5 /bin/x", "is the user column missing?"},
		{true, "0 5 * * 1-5 /bin/x --flag", `user "/bin/x" looks like a command`},
		{true, "0 30 2 * * * root /bin/x", `"*" after the schedule looks like a time field`},
	}
	for _, c := range cases {
		entries, err := ParseCrontab(strings.NewReader(c.line+"\n"), c.system)
		if err != nil || len(entries) != 1 {
			t.Fatalf("%q: %v %+v", c.line, err, entries)
		}
		e := entries[0]
		if e.Err == nil || !strings.Contains(e.Err.Error(), c.want) {
			t.Errorf("%q: err = %v, want %q", c.line, e.Err, c.want)
		}
		if e.Schedule != nil || e.Text != c.line {
			t.Errorf("%q: schedule %v, text %q", c.line, e.Schedule, e.Text)
		}
	}
}
