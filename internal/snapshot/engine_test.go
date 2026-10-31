package snapshot

import (
	"encoding/json"
	"reflect"
	"strings"
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

// pillars[].today counts the pillar's events dated today; leveled_today names
// the pillars whose level today's xp raised (✦ on the town's signs).
func TestPillarTodayAndLeveledToday(t *testing.T) {
	l := &qlog{}
	for i := 0; i < 6; i++ { // 6 notes x 15 = 90 xp yesterday: still level 1
		l.add("note.added", "note:y"+string(rune('a'+i)), "manual", "2026-10-28", `{}`)
	}
	s := Build(l.evs, nil, Source{}, now, la)
	if len(s.LeveledToday) != 0 || s.LeveledToday == nil {
		t.Fatalf("nothing levelled yet, want an empty list: %#v", s.LeveledToday)
	}
	if s.Pillars[4].Today != 0 || s.Pillars[4].Events7d != 6 {
		t.Fatalf("competence %+v", s.Pillars[4])
	}

	l.add("note.added", "note:today", "manual", "2026-10-29", `{}`) // 105 xp: level 2
	l.add("study.session", "skill:latin", "duet:faith", "2026-10-29", `{"minutes":5}`)
	l.evs[len(l.evs)-1].Pillar = "faith" // 1 xp, no level
	s = Build(l.evs, nil, Source{}, now, la)
	if want := []string{"competence"}; !reflect.DeepEqual(s.LeveledToday, want) {
		t.Fatalf("leveled_today %v", s.LeveledToday)
	}
	if s.Pillars[4].Today != 1 || s.Pillars[6].Pillar != "agents" || s.Pillars[7].Pillar != "faith" || s.Pillars[7].Today != 1 {
		t.Fatalf("today per pillar: %+v", s.Pillars)
	}
	if b, _ := json.Marshal(s.Pillars[4]); !strings.Contains(string(b), `"today":1`) {
		t.Fatalf("json %s", b)
	}

	// Already level 2 at dawn: today's xp doesn't level it again.
	l.add("note.added", "note:more", "manual", "2026-10-29", `{}`)
	if s = Build(l.evs, nil, Source{}, now, la); len(s.LeveledToday) != 1 {
		t.Fatalf("still the one level-up: %v", s.LeveledToday)
	}
}

// A Monkeytype run pulled by acta-mt keeps the streak, and earns xp, like
// any other event. Count-once still applies: a hand-logged copy of the same
// run adds neither a day nor xp.
func TestImportedRunKeepsStreakAndCountsOnce(t *testing.T) {
	b := base()
	b.add("typing.test", "2026-10-27", runData(80, 98, ""))
	b.mt("2026-10-28T10:00:00-07:00", runData(85.4, 97.2, ""))
	b.mt("2026-10-29T10:00:00-07:00", runData(87.6, 98.1, ""))
	s := Build(b.evs, nil, Source{}, now, la)
	if s.Streak != (Streak{Days: 3, Today: true}) {
		t.Fatalf("an import-only day must keep the streak: %+v", s.Streak)
	}
	// Today's run was also logged from a screenshot: still one run, 5 xp, one day.
	b.add("typing.test", "2026-10-29", runData(88, 98, ""))
	s2 := Build(b.evs, nil, Source{}, now, la)
	if s2.Streak != s.Streak || s2.XP.Total != s.XP.Total {
		t.Fatalf("hand-logged copy counted twice: streak %+v xp %d, was streak %+v xp %d", s2.Streak, s2.XP.Total, s.Streak, s.XP.Total)
	}
	if s2.XP.Total != 15 {
		t.Fatalf("want 3 runs x 5 xp, got %d", s2.XP.Total)
	}
}

// asks are Harlan's questions to a pillar's agent: open ones, plus those
// answered in the last 7 days, newest first. No xp either way.
func TestAsksInSnapshot(t *testing.T) {
	l := &qlog{}
	l.add("pillar.asked", "ask:old", "manual", "2026-10-01", `{"text":"Old?"}`)
	l.add("pillar.answered", "ask:old", "duet:competence", "2026-10-02", `{"text":"long ago"}`)
	l.add("pillar.asked", "ask:cut", "manual", "2026-10-28", `{"text":"What should I cut?"}`)
	l.add("pillar.answered", "ask:cut", "duet:competence", "2026-10-28", `{"text":"The Thursday sync."}`)
	l.add("pillar.asked", "ask:sleep", "manual", "2026-10-29", `{"text":"Sleep window?"}`)
	s := Build(l.evs, nil, Source{}, now, la)
	as := s.Asks
	if len(as) != 2 || as[0].ID != "ask:sleep" || !as[0].Open || as[0].Pillar != "competence" {
		t.Fatalf("asks %+v", as)
	}
	if as[1].Open || as[1].Answer != "The Thursday sync." || as[1].From != "duet:competence" || as[1].Answered != "2026-10-28" {
		t.Fatalf("answered ask %+v", as[1])
	}
	if s.XP.Total != 0 {
		t.Fatalf("asks earn no xp, got %d", s.XP.Total)
	}
}

// An answer filed under another pillar does not close the ask: a channel
// speaks for its own pillar only.
func TestAskIgnoresAnotherPillarsAnswer(t *testing.T) {
	l := &qlog{}
	l.add("pillar.asked", "ask:sleep", "manual", "2026-10-29", `{"text":"Sleep window?"}`)
	l.add("pillar.answered", "ask:sleep", "duet:time", "2026-10-29", `{"text":"Spend less."}`)
	l.evs[1].Pillar = "time"
	as := Build(l.evs, nil, Source{}, now, la).Asks
	if len(as) != 1 || !as[0].Open || as[0].Answer != "" || as[0].From != "" {
		t.Fatalf("another pillar's answer closed the ask: %+v", as)
	}
}

// A note earns xp once; a new version or a shared node earns nothing.
func TestNoteXPOncePerNote(t *testing.T) {
	sum := strings.Repeat("0", 64)
	l := &qlog{}
	l.add("note.added", "note:study", "duet:competence", "2026-10-29", `{"path":"notes/competence/study.md","sha256":"`+sum+`"}`)            // 15
	l.add("note.added", "note:study", "manual", "2026-10-29", `{"path":"notes/competence/study.md","sha256":"`+sum+`"}`)                     // a new version: 0
	l.add("note.added", "note:martin-luther", "duet:competence", "2026-10-29", `{"path":"notes/nodes/martin-luther.md","sha256":"`+sum+`"}`) // 0
	if s := Build(l.evs, nil, Source{}, now, la); s.XP.Total != 15 {
		t.Fatalf("total %d", s.XP.Total)
	}
}
