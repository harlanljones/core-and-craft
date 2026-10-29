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
