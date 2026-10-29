package quest

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestParseAcceptsAndNormalizes(t *testing.T) {
	cases := map[string]string{
		"":                 "once",
		"once":             "once",
		"daily":            "daily",
		"weekdays":         "weekdays",
		"weekly:thu,mon":   "weekly:mon,thu",
		"weekly:sun,sun":   "weekly:sun",
		"every:3d":         "every:3d",
		"monthly:1":        "monthly:1",
		"monthly:31":       "monthly:31",
		" Weekly:MON ":     "weekly:mon",
		"weekly:mon,tue,w": "",
	}
	for in, want := range cases {
		e, err := Parse(in)
		if want == "" {
			if err == nil {
				t.Errorf("%q: want error, got %v", in, e)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if e.String() != want {
			t.Errorf("%q: got %q, want %q", in, e.String(), want)
		}
	}
}

func TestParseRejects(t *testing.T) {
	for _, in := range []string{"hourly", "weekly:", "weekly:funday", "every:0d", "every:3w", "every:400d", "monthly:0", "monthly:32", "monthly:x", "daily:2"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("%q: want error", in)
		}
	}
}

func TestDue(t *testing.T) {
	anchor := day("2026-10-26") // a Monday
	cases := []struct {
		every string
		day   string
		want  bool
	}{
		{"once", "2026-10-25", false}, // before it was added
		{"once", "2026-10-26", true},
		{"once", "2026-11-30", true}, // open until done
		{"daily", "2026-10-25", false},
		{"daily", "2026-10-31", true},
		{"weekdays", "2026-10-31", false}, // Saturday
		{"weekdays", "2026-11-02", true},
		{"weekly:mon,thu", "2026-10-29", true},
		{"weekly:mon,thu", "2026-10-30", false},
		{"every:3d", "2026-10-26", true},
		{"every:3d", "2026-10-28", false},
		{"every:3d", "2026-10-29", true},
		{"monthly:1", "2026-11-01", true},
		{"monthly:1", "2026-11-02", false},
		{"monthly:31", "2026-11-30", true}, // short month: its last day
		{"monthly:31", "2026-10-31", true},
		{"monthly:31", "2026-10-30", false},
	}
	for _, c := range cases {
		e, err := Parse(c.every)
		if err != nil {
			t.Fatal(err)
		}
		if got := e.Due(anchor, day(c.day)); got != c.want {
			t.Errorf("%s on %s: got %v, want %v", c.every, c.day, got, c.want)
		}
	}
}

func TestRecurring(t *testing.T) {
	once, _ := Parse("once")
	daily, _ := Parse("daily")
	if once.Recurring() || !daily.Recurring() {
		t.Fatal("once is not recurring, daily is")
	}
}
