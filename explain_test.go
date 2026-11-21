package cronexplain

import "testing"

func TestExplain(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"* * * * *", "Every minute."},
		{"*/5 * * * *", "Every 5 minutes."},
		{"15 * * * *", "At minute 15 of every hour."},
		{"0 9 * * *", "At 09:00."},
		{"0 9,17 * * *", "At 09:00 and 17:00."},
		{"30 9 * * 1-5", "At 09:30, on Monday through Friday."},
		{"15 9-17 * * MON-FRI", "At minute 15, from 09:00 through 17:59, on Monday through Friday."},
		{"*/10 9 * * *", "Every 10 minutes, from 09:00 through 09:59."},
		{"*/15 9,17 * * *", "Every 15 minutes, during hours 09 and 17."},
		{"23 0-20/2 * * *", "At minute 23, every 2 hours from 00:00 through 20:59."},
		{"0,30-35 * * * *", "At minute 0 and every minute from 30 through 35."},
		{"0 0 1,15 * *", "At 00:00, on days 1 and 15 of the month."},
		{"0 0 13 * 5", "At 00:00, on day 13 of the month or on Friday (either one is enough)."},
		{"0 0 */2 * 1", "At 00:00, every 2nd day of the month, but only on Monday."},
		{"0 12 * 1-3 *", "At 12:00, in January through March."},
		{"0 0 1 */3 *", "At 00:00, on day 1 of the month, in January, April, July and October."},
		{"0 0 * * 5-7", "At 00:00, on Friday through Sunday."},
		{"@yearly", "At 00:00, on January 1."},
		{"@weekly", "At 00:00, on Sunday."},
		{"*/30 * * * * *", "Every 30 seconds."},
		{"30 */5 * * * *", "At second 30, every 5 minutes."},
		{"0 0 * * * *", "At minute 0 of every hour."},
		{"15 30 9 * * *", "At 09:30:15."},
	}
	for _, tt := range tests {
		if got := mustParse(t, tt.expr).Explain(); got != tt.want {
			t.Errorf("Explain(%q)\n got  %q\n want %q", tt.expr, got, tt.want)
		}
	}
}

func TestOrdinal(t *testing.T) {
	for n, want := range map[int]string{1: "1st", 2: "2nd", 3: "3rd", 4: "4th", 11: "11th", 12: "12th", 13: "13th", 21: "21st", 22: "22nd"} {
		if got := ordinal(n); got != want {
			t.Errorf("ordinal(%d) = %s, want %s", n, got, want)
		}
	}
}
