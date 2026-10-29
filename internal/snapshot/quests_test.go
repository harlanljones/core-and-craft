package snapshot

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/harlanljones/core-and-craft/internal/event"
)

// qlog builds a log of quest events in seq order.
type qlog struct{ evs []event.Event }

func (l *qlog) add(kind, subject, agent, at, data string) string {
	n := len(l.evs) + 1
	prec := event.PrecisionSecond
	if len(at) == 10 {
		prec = event.PrecisionDay
	}
	id := fmt.Sprintf("evt_%016x", n)
	l.evs = append(l.evs, event.Event{
		ID: id, V: 1, At: at, AtPrecision: prec, Seq: int64(n),
		RecordedAt: "2026-10-29T17:00:00Z", Pillar: "competence", Kind: kind, Subject: subject,
		Source: event.Source{Via: "manual", Agent: agent}, Data: json.RawMessage(data),
	})
	return id
}

func (l *qlog) retract(target string) {
	l.add("event.retracted", target, "manual", "2026-10-29", `{"reason":"undo"}`)
}

func byID(qs []Quest) map[string]Quest {
	m := map[string]Quest{}
	for _, q := range qs {
		m[q.ID] = q
	}
	return m
}

func TestQuestFoldStatuses(t *testing.T) {
	l := &qlog{}
	l.add("quest.added", "quest:drill", "manual", "2026-10-26", `{"title":"Drill","priority":1,"every":"daily","skill":"skill:typing","rule":"runs:1"}`)
	l.add("quest.added", "quest:budget", "duet:economic", "2026-10-27", `{"title":"Budget"}`)
	l.add("quest.added", "quest:call", "claude-code", "2026-10-27", `{"title":"Call","status":"proposed"}`)
	l.add("quest.added", "quest:read", "manual", "2026-10-27", `{"title":"Read","every":"Weekly:THU,mon"}`)
	l.add("quest.added", "quest:old", "manual", "2026-10-20", `{"title":"Old"}`)
	l.add("quest.added", "quest:drill", "manual", "2026-10-28", `{"title":"Duplicate add is ignored"}`)
	l.add("quest.accepted", "quest:call", "manual", "2026-10-28", `{}`)
	l.add("quest.declined", "quest:budget", "manual", "2026-10-28", `{}`)
	l.add("quest.dropped", "quest:old", "manual", "2026-10-28", `{}`)
	l.add("quest.edited", "quest:read", "manual", "2026-10-28", `{"priority":3,"title":"Read Confessions"}`)
	l.add("quest.snoozed", "quest:read", "manual", "2026-10-28", `{"until":"2026-11-02"}`)
	qs := byID(Quests(Effective(l.evs), la))

	drill := qs["quest:drill"]
	if drill.Status != "active" || drill.Title != "Drill" || drill.Priority != 1 || drill.Every != "daily" || drill.Skill != "skill:typing" || drill.Rule != "runs:1" || drill.Added != "2026-10-26" || drill.From != "manual" {
		t.Errorf("drill %+v", drill)
	}
	if qs["quest:budget"].Status != "declined" {
		t.Errorf("an agent quest without status is proposed, then declined: %+v", qs["quest:budget"])
	}
	if qs["quest:call"].Status != "active" {
		t.Errorf("accepted proposal is active: %+v", qs["quest:call"])
	}
	if qs["quest:old"].Status != "dropped" {
		t.Errorf("old %+v", qs["quest:old"])
	}
	read := qs["quest:read"]
	if read.Title != "Read Confessions" || read.Priority != 3 || read.Every != "weekly:mon,thu" || read.SnoozedUntil != "2026-11-02" {
		t.Errorf("read %+v", read)
	}
	if qs["quest:old"].Priority != 2 {
		t.Errorf("default priority is 2: %+v", qs["quest:old"])
	}
}

func TestQuestDoneOnceAndRecurring(t *testing.T) {
	l := &qlog{}
	l.add("quest.added", "quest:once", "manual", "2026-10-27", `{"title":"Once"}`)
	l.add("quest.added", "quest:daily", "manual", "2026-10-27", `{"title":"Daily","every":"daily"}`)
	l.add("quest.done", "quest:once", "manual", "2026-10-28T09:00:00-07:00", `{}`)
	l.add("quest.done", "quest:daily", "manual", "2026-10-28T09:00:00-07:00", `{}`)
	undo := l.add("quest.done", "quest:daily", "manual", "2026-10-29T09:00:00-07:00", `{}`)
	l.retract(undo)
	qs := byID(Quests(Effective(l.evs), la))
	if q := qs["quest:once"]; q.Status != "done" || q.LastDone != "2026-10-28" {
		t.Errorf("once %+v", q)
	}
	d := qs["quest:daily"]
	if d.Status != "active" || d.LastDone != "2026-10-28" || len(d.DoneDates) != 1 || !d.DoneOn("2026-10-28") || d.DoneOn("2026-10-29") {
		t.Errorf("daily %+v", d)
	}
}

func TestDoneOnRetractedOnceQuestReopens(t *testing.T) {
	l := &qlog{}
	l.add("quest.added", "quest:once", "manual", "2026-10-27", `{"title":"Once"}`)
	done := l.add("quest.done", "quest:once", "manual", "2026-10-28", `{}`)
	l.retract(done)
	if q := Quests(Effective(l.evs), la)[0]; q.Status != "active" || q.LastDone != "" {
		t.Errorf("undone once quest %+v", q)
	}
}
