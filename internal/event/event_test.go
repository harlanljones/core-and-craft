package event

import (
	"encoding/json"
	"strings"
	"testing"
)

func valid() Event {
	return Event{
		ID: "evt_0123456789abcdef", V: 1, At: "2026-10-29", AtPrecision: PrecisionDay, Seq: 1,
		RecordedAt: "2026-10-29T17:45:00Z", Pillar: "competence", Kind: "typing.test",
		Subject: "skill:typing", Source: Source{Via: "chat", Agent: "duet:competence"},
		Data: json.RawMessage(`{"wpm":88,"acc":97}`),
	}
}

func TestValidateAcceptsGoodEvent(t *testing.T) {
	if err := Validate(valid(), true); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := map[string]func(*Event){
		"version":         func(e *Event) { e.V = 2 },
		"day not date":    func(e *Event) { e.At = "2026-10-29T10:00:00-07:00" },
		"second no zone":  func(e *Event) { e.AtPrecision = PrecisionSecond; e.At = "2026-10-29T10:00:00" },
		"pillar initial":  func(e *Event) { e.Pillar = "c" },
		"unknown kind":    func(e *Event) { e.Kind = "quest.tapped" },
		"bare subject":    func(e *Event) { e.Subject = "typing" },
		"agent":           func(e *Event) { e.Source.Agent = "someone" },
		"empty duet":      func(e *Event) { e.Source.Agent = "duet:" },
		"data not object": func(e *Event) { e.Data = json.RawMessage(`[1]`) },
		"missing seq":     func(e *Event) { e.Seq = 0 },
		"bad id":          func(e *Event) { e.ID = "evt_xyz" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e := valid()
			mutate(&e)
			if Validate(e, true) == nil {
				t.Fatal("want error")
			}
		})
	}
}

func TestRetractSubjectIsEventID(t *testing.T) {
	e := valid()
	e.Kind, e.Subject, e.Data = "event.retracted", "mt:65a1b2c3d4", json.RawMessage(`{"reason":"wrong mode"}`)
	if err := Validate(e, true); err != nil {
		t.Fatal(err)
	}
}

func TestNativeIDIgnoresSeqAndKeyOrder(t *testing.T) {
	a := valid()
	b := valid()
	b.Seq = 42
	b.Data = json.RawMessage(`{ "acc": 97, "wpm": 88 }`)
	ida, err := NativeID(a)
	if err != nil {
		t.Fatal(err)
	}
	idb, _ := NativeID(b)
	if ida != idb {
		t.Fatalf("ids differ: %s %s", ida, idb)
	}
	if !strings.HasPrefix(ida, "evt_") || len(ida) != 20 {
		t.Fatalf("bad id %s", ida)
	}
}

func TestNativeIDSeparatesIdenticalRunsByRecordedAt(t *testing.T) {
	a := valid()
	b := valid()
	b.RecordedAt = "2026-10-29T17:45:01Z"
	ida, _ := NativeID(a)
	idb, _ := NativeID(b)
	if ida == idb {
		t.Fatal("identical runs logged at different times must not collide")
	}
}

// Pins the hash recipe so remote writers in other languages can match it:
// sha256 over compact, key-sorted JSON of agent, at, data, kind,
// recorded_at, subject.
func TestNativeIDGolden(t *testing.T) {
	got, err := NativeID(valid())
	if err != nil {
		t.Fatal(err)
	}
	const want = "evt_" + goldenHex
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

// goldenHex was computed independently in Python:
// json.dumps(body, sort_keys=True, separators=(",", ":"), ensure_ascii=False).
const goldenHex = "7d5123a3cb46a431"

// questEvent is a valid quest.added from Harlan, ready to mutate.
func questEvent() Event {
	e := valid()
	e.Kind, e.Subject, e.Source = "quest.added", "quest:typing-drill", Source{Via: "manual", Agent: "manual"}
	e.Data = json.RawMessage(`{"title":"15 min typing drill","priority":1,"every":"daily","skill":"skill:typing","rule":"runs:1"}`)
	return e
}

func TestQuestAndCheckinKindsAccepted(t *testing.T) {
	cases := map[string]func(*Event){
		"quest.added":         func(e *Event) {},
		"quest.added minimal": func(e *Event) { e.Data = json.RawMessage(`{"title":"Call mom"}`) },
		"agent proposes": func(e *Event) {
			e.Source.Agent = "duet:competence"
			e.Data = json.RawMessage(`{"title":"x","status":"proposed"}`)
		},
		"agent, no status": func(e *Event) { e.Source.Agent = "claude-code"; e.Data = json.RawMessage(`{"title":"x"}`) },
		"quest.done":       func(e *Event) { e.Kind, e.Data = "quest.done", json.RawMessage(`{}`) },
		"quest.accepted":   func(e *Event) { e.Kind, e.Data = "quest.accepted", json.RawMessage(`{}`) },
		"quest.declined":   func(e *Event) { e.Kind, e.Data = "quest.declined", json.RawMessage(`{"reason":"not now"}`) },
		"quest.dropped":    func(e *Event) { e.Kind, e.Data = "quest.dropped", json.RawMessage(`{}`) },
		"quest.snoozed":    func(e *Event) { e.Kind, e.Data = "quest.snoozed", json.RawMessage(`{"until":"2026-11-02"}`) },
		"quest.edited": func(e *Event) {
			e.Kind, e.Data = "quest.edited", json.RawMessage(`{"priority":3,"every":"weekly:sun"}`)
		},
		"checkin.asked": func(e *Event) {
			e.Kind, e.Subject, e.Data = "checkin.asked", "checkin:sleep-window", json.RawMessage(`{"question":"What's your target sleep window?"}`)
		},
		"checkin.answered": func(e *Event) {
			e.Kind, e.Subject, e.Data = "checkin.answered", "checkin:sleep-window", json.RawMessage(`{"text":"22:30 to 06:30"}`)
		},
		"day.rested":               func(e *Event) { e.Kind, e.Subject, e.Data = "day.rested", "day:2026-10-29", json.RawMessage(`{}`) },
		"quest.added minutes rule": func(e *Event) { e.Data = json.RawMessage(`{"title":"Read","skill":"skill:latin","rule":"minutes:15"}`) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e := questEvent()
			mutate(&e)
			if err := Validate(e, true); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestQuestAndCheckinKindsRejected(t *testing.T) {
	cases := map[string]func(*Event){
		"no title":           func(e *Event) { e.Data = json.RawMessage(`{"priority":1}`) },
		"blank title":        func(e *Event) { e.Data = json.RawMessage(`{"title":"  "}`) },
		"priority 0":         func(e *Event) { e.Data = json.RawMessage(`{"title":"x","priority":0}`) },
		"priority 1.5":       func(e *Event) { e.Data = json.RawMessage(`{"title":"x","priority":1.5}`) },
		"bad every":          func(e *Event) { e.Data = json.RawMessage(`{"title":"x","every":"hourly"}`) },
		"rule without skill": func(e *Event) { e.Data = json.RawMessage(`{"title":"x","rule":"runs:1"}`) },
		"bad rule":           func(e *Event) { e.Data = json.RawMessage(`{"title":"x","skill":"skill:typing","rule":"runs:0"}`) },
		"bad skill":          func(e *Event) { e.Data = json.RawMessage(`{"title":"x","skill":"typing"}`) },
		"bad status":         func(e *Event) { e.Data = json.RawMessage(`{"title":"x","status":"done"}`) },
		"agent self-activates": func(e *Event) {
			e.Source.Agent = "duet:faith"
			e.Data = json.RawMessage(`{"title":"x","status":"active"}`)
		},
		"quest kind, skill subject": func(e *Event) { e.Subject = "skill:typing" },
		"snooze no until":           func(e *Event) { e.Kind, e.Data = "quest.snoozed", json.RawMessage(`{}`) },
		"snooze bad until":          func(e *Event) { e.Kind, e.Data = "quest.snoozed", json.RawMessage(`{"until":"soon"}`) },
		"edit nothing":              func(e *Event) { e.Kind, e.Data = "quest.edited", json.RawMessage(`{}`) },
		"edit bad every":            func(e *Event) { e.Kind, e.Data = "quest.edited", json.RawMessage(`{"every":"sometimes"}`) },
		"ask no question":           func(e *Event) { e.Kind, e.Subject, e.Data = "checkin.asked", "checkin:x", json.RawMessage(`{}`) },
		"answer no text": func(e *Event) {
			e.Kind, e.Subject, e.Data = "checkin.answered", "checkin:x", json.RawMessage(`{"text":""}`)
		},
		"checkin on quest": func(e *Event) { e.Kind, e.Data = "checkin.asked", json.RawMessage(`{"question":"?"}`) },
		"rest bad subject": func(e *Event) { e.Kind, e.Subject, e.Data = "day.rested", "day:today", json.RawMessage(`{}`) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e := questEvent()
			mutate(&e)
			if Validate(e, true) == nil {
				t.Fatal("want error")
			}
		})
	}
}
