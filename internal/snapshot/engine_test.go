package snapshot

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// now is Thursday 2026-10-29, 20:00 in Los Angeles (snapshot_test.go).
var todayLA = time.Date(2026, 10, 29, 0, 0, 0, 0, la)

func TestLevelCurve(t *testing.T) {
	cases := map[int]Level{
		0:   {Total: 0, Level: 1, Into: 0, Need: 100},
		99:  {Total: 99, Level: 1, Into: 99, Need: 100},
		100: {Total: 100, Level: 2, Into: 0, Need: 150},
		249: {Total: 249, Level: 2, Into: 149, Need: 150},
		250: {Total: 250, Level: 3, Into: 0, Need: 225},
		475: {Total: 475, Level: 4, Into: 0, Need: 338},
	}
	for xp, want := range cases {
		if got := levelOf(xp); got != want {
			t.Errorf("%d xp: got %+v, want %+v", xp, got, want)
		}
	}
}

func TestStreak(t *testing.T) {
	l := &qlog{}
	l.add("note.added", "note:a", "manual", "2026-10-26", `{}`)
	l.add("note.added", "note:b", "duet:faith", "2026-10-27", `{}`)
	l.add("day.rested", "day:2026-10-28", "manual", "2026-10-28", `{}`)
	l.add("typing.test", "skill:typing", "acta-mt", "2026-10-29T09:00:00-07:00", `{}`) // imports don't count
	if s := streakOf(l.evs, todayLA, la); s != (Streak{Days: 3, Today: false}) {
		t.Fatalf("through yesterday: %+v", s)
	}
	l.add("quest.done", "quest:x", "manual", "2026-10-29T10:00:00-07:00", `{}`)
	if s := streakOf(l.evs, todayLA, la); s != (Streak{Days: 4, Today: true}) {
		t.Fatalf("with today: %+v", s)
	}
	gap := &qlog{}
	gap.add("note.added", "note:a", "manual", "2026-10-27", `{}`)
	if s := streakOf(gap.evs, todayLA, la); s.Days != 0 {
		t.Fatalf("a missed day breaks it: %+v", s)
	}
}

func TestXPTable(t *testing.T) {
	b := base()
	b.add("skill.target_set", "2026-10-20", `{"target":80,"sustain_runs":1,"sustain_days":7}`)
	for i := 0; i < 7; i++ { // 7 qualifying runs today: capped at 25
		b.add("typing.test", "2026-10-29T09:00:00-07:00", runData(85, 98, ""))
	}
	b.add("typing.test", "2026-10-28T09:00:00-07:00", runData(70, 98, "")) // 5
	evs := b.evs
	l := &qlog{evs: evs}
	l.add("quest.added", "quest:p1", "manual", "2026-10-28", `{"title":"P1","priority":1}`)
	l.add("quest.added", "quest:p2", "manual", "2026-10-28", `{"title":"P2"}`)
	l.add("quest.done", "quest:p1", "manual", "2026-10-29T10:00:00-07:00", `{}`) // 15
	l.add("quest.done", "quest:p2", "manual", "2026-10-29T10:00:00-07:00", `{}`) // 10
	l.add("note.added", "note:x", "manual", "2026-10-29", `{}`)                  // 15
	l.add("checkin.asked", "checkin:q", "duet:faith", "2026-10-29", `{"question":"?"}`)
	l.add("checkin.answered", "checkin:q", "manual", "2026-10-29", `{"text":"a"}`)  // 5
	l.add("checkin.answered", "checkin:q", "manual", "2026-10-29", `{"text":"b"}`)  // 0: first answer only
	l.add("study.session", "skill:latin", "manual", "2026-10-29", `{"minutes":90}`) // 18 -> capped 12
	s := Build(l.evs, nil, Source{}, now, la)
	// typing 25 today + 5 yesterday + passed 50 (target 80, one run) = 80;
	// quests 25, note 15, check-in 5, study 12 = 57.
	if s.XP.Total != 137 {
		t.Fatalf("total %d, history %v", s.XP.Total, s.History)
	}
	if len(s.History) != 30 || s.History[29] != 132 || s.History[28] != 5 {
		t.Fatalf("history tail %v", s.History[27:])
	}
	if s.Pillars[4].Pillar != "competence" || s.Pillars[4].XP != 137 || s.Pillars[4].Level != 2 {
		t.Fatalf("competence %+v", s.Pillars[4])
	}
}

func TestEngineRanksAndExplains(t *testing.T) {
	b := base()
	b.add("skill.target_set", "2026-10-20", `{"target":100}`)
	b.add("typing.test", "2026-10-29T09:00:00-07:00", runData(88, 98, ""))
	l := &qlog{evs: b.evs}
	l.add("quest.added", "quest:drill", "manual", "2026-10-20", `{"title":"Drill","every":"daily","skill":"skill:typing","rule":"runs:2"}`)
	l.add("quest.added", "quest:call", "manual", "2026-10-19", `{"title":"Call","every":"weekly:mon","priority":1}`)
	l.add("quest.added", "quest:read", "manual", "2026-10-28", `{"title":"Read","priority":3}`)
	l.add("quest.added", "quest:nap", "manual", "2026-10-28", `{"title":"Nap"}`)
	l.add("quest.added", "quest:pitch", "duet:career", "2026-10-29", `{"title":"Pitch"}`)
	l.add("quest.snoozed", "quest:nap", "manual", "2026-10-29", `{"until":"2026-11-01"}`)
	l.add("quest.done", "quest:call", "manual", "2026-10-19", `{}`)
	l.evs[5].Pillar = "relationships" // quest:call, after the 4 skill events and drill
	s := Build(l.evs, nil, Source{}, now, la)

	var ids []string
	for _, q := range s.Quests {
		ids = append(ids, q.ID)
	}
	// drill and read are due today; call (weekly:mon) isn't, but missed Monday.
	if want := []string{"quest:drill", "quest:read", "quest:call"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("order %v", ids)
	}
	drill := s.Quests[0]
	if !drill.Due || drill.DoneToday || drill.Auto || drill.OverdueDays != 9 {
		t.Fatalf("drill %+v", drill)
	}
	if want := []string{"overdue", "due today"}; !reflect.DeepEqual(reasonKinds(drill.Reasons), want) {
		t.Fatalf("drill reasons %v", drill.Reasons)
	}
	call := s.Quests[2]
	if call.Due || call.OverdueDays != 3 || call.Reasons[0] != "priority 1" {
		t.Fatalf("call %+v", call)
	}
	if len(s.Proposed) != 1 || s.Proposed[0].ID != "quest:pitch" || s.Proposed[0].From != "duet:career" {
		t.Fatalf("proposed %+v", s.Proposed)
	}

	// A second qualifying run meets runs:2: the drill finishes itself.
	l.evs = append(l.evs, l.evs[3]) // the 88 wpm run again, as a new event
	l.evs[len(l.evs)-1].ID, l.evs[len(l.evs)-1].Seq = "evt_00000000000000ff", 999
	s2 := Build(l.evs, nil, Source{}, now, la)
	last := s2.Quests[len(s2.Quests)-1]
	if last.ID != "quest:drill" || !last.Auto || !last.DoneToday || last.Due {
		t.Fatalf("auto-done drill should sort last: %+v", s2.Quests)
	}

	// Same log, same answer.
	a, _ := json.Marshal(Build(l.evs, nil, Source{}, now, la).Quests)
	c, _ := json.Marshal(Build(l.evs, nil, Source{}, now, la).Quests)
	if string(a) != string(c) {
		t.Fatal("engine is not deterministic")
	}
}

// reasonKinds keeps a stable word from each reason for comparison.
func reasonKinds(rs []string) []string {
	var out []string
	for _, r := range rs {
		switch {
		case len(r) > 7 && r[len(r)-7:] == "overdue":
			out = append(out, "overdue")
		default:
			out = append(out, r)
		}
	}
	return out
}

func TestCheckinsInSnapshot(t *testing.T) {
	l := &qlog{}
	l.add("checkin.asked", "checkin:sleep", "duet:recovery", "2026-10-28", `{"question":"Sleep window?"}`)
	l.add("checkin.asked", "checkin:old", "duet:faith", "2026-10-01", `{"question":"Old?"}`)
	l.add("checkin.answered", "checkin:old", "manual", "2026-10-02", `{"text":"done"}`)
	l.add("checkin.asked", "checkin:wins", "duet:career", "2026-10-27", `{"question":"Wins?"}`)
	l.add("checkin.answered", "checkin:wins", "manual", "2026-10-28", `{"text":"shipped lq"}`)
	cs := Build(l.evs, nil, Source{}, now, la).Checkins
	if len(cs) != 2 || cs[0].ID != "checkin:sleep" || !cs[0].Open || cs[1].Open || cs[1].Answer != "shipped lq" || cs[1].Answered != "2026-10-28" {
		t.Fatalf("checkins %+v", cs)
	}
}
