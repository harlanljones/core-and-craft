// Package quest holds the parts of quests that more than one package needs:
// the recurrence rules ("every") that say on which days a quest is due.
package quest

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Every is a parsed recurrence rule:
//
//	once            due from the day it was added until done (the default)
//	daily           every day
//	weekdays        Monday to Friday
//	weekly:mon,thu  the named days
//	every:3d        every N days counted from the day it was added (1..365)
//	monthly:1       that day of each month; past a short month's end, its last day
type Every struct {
	Kind string // once, daily, weekdays, weekly, every, monthly
	Days []time.Weekday
	N    int
}

var dayNames = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// Parse reads a rule, case- and space-insensitively. Empty means once.
func Parse(s string) (Every, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	kind, arg, hasArg := strings.Cut(s, ":")
	bad := func() (Every, error) {
		return Every{}, fmt.Errorf("every: %q is not once, daily, weekdays, weekly:mon,thu, every:Nd or monthly:D", s)
	}
	switch kind {
	case "", "once", "daily", "weekdays":
		if hasArg {
			return bad()
		}
		if kind == "" {
			kind = "once"
		}
		return Every{Kind: kind}, nil
	case "weekly":
		seen := map[time.Weekday]bool{}
		for _, name := range strings.Split(arg, ",") {
			i := indexOf(dayNames, strings.TrimSpace(name))
			if i < 0 {
				return bad()
			}
			seen[time.Weekday(i)] = true
		}
		e := Every{Kind: kind}
		for d := range seen {
			e.Days = append(e.Days, d)
		}
		// Monday first, as people read a week.
		sort.Slice(e.Days, func(i, j int) bool { return (e.Days[i]+6)%7 < (e.Days[j]+6)%7 })
		return e, nil
	case "every":
		n, err := strconv.Atoi(strings.TrimSuffix(arg, "d"))
		if !strings.HasSuffix(arg, "d") || err != nil || n < 1 || n > 365 {
			return bad()
		}
		return Every{Kind: kind, N: n}, nil
	case "monthly":
		n, err := strconv.Atoi(arg)
		if err != nil || n < 1 || n > 31 {
			return bad()
		}
		return Every{Kind: kind, N: n}, nil
	}
	return bad()
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

// String is the canonical form Parse accepts.
func (e Every) String() string {
	switch e.Kind {
	case "weekly":
		names := make([]string, len(e.Days))
		for i, d := range e.Days {
			names[i] = dayNames[d]
		}
		return "weekly:" + strings.Join(names, ",")
	case "every":
		return fmt.Sprintf("every:%dd", e.N)
	case "monthly":
		return fmt.Sprintf("monthly:%d", e.N)
	}
	return e.Kind
}

// Recurring reports whether the quest comes back after it is done.
func (e Every) Recurring() bool { return e.Kind != "once" }

// Due reports whether a quest added on anchor is due on day. Both are
// calendar dates; only their year, month and day are used. Nothing is due
// before it was added. A once quest stays due until it is done, which the
// caller tracks.
func (e Every) Due(anchor, day time.Time) bool {
	a := date(anchor)
	d := date(day)
	if d.Before(a) {
		return false
	}
	switch e.Kind {
	case "once", "daily":
		return true
	case "weekdays":
		return d.Weekday() != time.Saturday && d.Weekday() != time.Sunday
	case "weekly":
		for _, w := range e.Days {
			if d.Weekday() == w {
				return true
			}
		}
		return false
	case "every":
		days := int(d.Sub(a).Hours()/24 + 0.5)
		return days%e.N == 0
	case "monthly":
		last := time.Date(d.Year(), d.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
		want := e.N
		if want > last {
			want = last
		}
		return d.Day() == want
	}
	return false
}

func date(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
