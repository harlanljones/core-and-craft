package snapshot

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/harlanljones/core-and-craft/internal/event"
)

var la, _ = time.LoadLocation("America/Los_Angeles")

type builder struct {
	evs []event.Event
}

func (b *builder) add(kind, at string, data string) string {
	prec := event.PrecisionSecond
	if len(at) == 10 {
		prec = event.PrecisionDay
	}
	id := fmt.Sprintf("evt_%016x", len(b.evs)+1)
	subject := "skill:typing"
	b.evs = append(b.evs, event.Event{
		ID: id, V: 1, At: at, AtPrecision: prec, Seq: int64(len(b.evs) + 1),
		RecordedAt: "2026-10-29T17:45:00Z", Pillar: "competence", Kind: kind,
		Subject: subject, Source: event.Source{Via: "chat", Agent: "lq"},
		Data: json.RawMessage(data),
	})
	return id
}

func (b *builder) retract(target string) {
	b.evs = append(b.evs, event.Event{
		ID: fmt.Sprintf("evt_%016x", len(b.evs)+1), V: 1, At: "2026-10-29", AtPrecision: event.PrecisionDay,
		Seq: int64(len(b.evs) + 1), RecordedAt: "2026-10-29T18:00:00Z", Pillar: "competence",
		Kind: "event.retracted", Subject: target, Source: event.Source{Via: "manual", Agent: "lq"},
		Data: json.RawMessage(`{"reason":"test"}`),
	})
}

const protocol = `{"mode":"time","mode2":"60","language":"english","punctuation":false,"numbers":false,"funbox":"none","lazyMode":false,"min_acc":97}`

func base() *builder {
	b := &builder{}
	b.add("skill.defined", "2026-10-29", `{"name":"Typing","metric":"best_wpm_at_acc"}`)
	b.add("skill.protocol_set", "2026-10-29", protocol)
	return b
}

func runData(wpm, acc float64, extra string) string {
	return fmt.Sprintf(`{"wpm":%v,"acc":%v,"mode":"time","mode2":"60","language":"english","punctuation":false,"numbers":false%s}`, wpm, acc, extra)
}

var now = time.Date(2026, 10, 29, 20, 0, 0, 0, la)

func typing(t *testing.T, s Snapshot) Skill {
	t.Helper()
	if len(s.Skills) != 1 {
		t.Fatalf("want 1 skill, got %d", len(s.Skills))
	}
	return s.Skills[0]
}

func TestGateCountsOnlyProtocolRunsAtAccuracy(t *testing.T) {
	b := base()
	b.add("typing.test", "2026-10-29T09:00:00-07:00", runData(85, 97, ""))
	b.add("typing.test", "2026-10-29T09:05:00-07:00", runData(120, 95, ""))                     // below accuracy
	b.add("typing.test", "2026-10-29T09:10:00-07:00", runData(130, 99, `,"bailedOut":true`))    // bailed out
	b.add("typing.test", "2026-10-29T09:15:00-07:00", runData(140, 99, `,"funbox":["mirror"]`)) // funbox
	b.add("typing.test", "2026-10-29T09:20:00-07:00", runData(150, 99, `,"lazyMode":true`))     // lazy mode
	b.add("typing.test", "2026-10-29T09:25:00-07:00", `{"wpm":160,"acc":99,"mode":"words","mode2":"25","language":"english"}`)
	b.add("typing.test", "2026-10-29T09:30:00-07:00", runData(88, 97, `,"funbox":[]`))
	sk := typing(t, Build(b.evs, nil, Source{}, now, la))
	if sk.Best == nil || *sk.Best != 88 {
		t.Fatalf("best %v", sk.Best)
	}
	if sk.RunsQualifying != 2 || sk.RunsTotal != 7 {
		t.Fatalf("qualifying %d total %d", sk.RunsQualifying, sk.RunsTotal)
	}
	if sk.TodayBest == nil || *sk.TodayBest != 88 {
		t.Fatalf("today best %v", sk.TodayBest)
	}
}

func TestRetractedRunDropsOut(t *testing.T) {
	b := base()
	b.add("typing.test", "2026-10-29T09:00:00-07:00", runData(85, 97, ""))
	pb := b.add("typing.test", "2026-10-29T09:30:00-07:00", runData(99, 98, ""))
	b.retract(pb)
	sk := typing(t, Build(b.evs, nil, Source{}, now, la))
	if *sk.Best != 85 || sk.RunsTotal != 1 {
		t.Fatalf("best %v total %d", *sk.Best, sk.RunsTotal)
	}
}

func TestSustainedPassNeedsRunsInsideWindow(t *testing.T) {
	b := base()
	b.add("skill.target_set", "2026-10-20", `{"metric":"best_wpm_at_acc","target":90,"sustain_runs":3,"sustain_days":7}`)
	b.add("typing.test", "2026-10-10T09:00:00-07:00", runData(91, 97, ""))
	b.add("typing.test", "2026-10-20T09:00:00-07:00", runData(92, 97, ""))
	b.add("typing.test", "2026-10-25T09:00:00-07:00", runData(90, 98, ""))
	sk := typing(t, Build(b.evs, nil, Source{}, now, la))
	if sk.Passed {
		t.Fatal("two runs inside a week should not pass")
	}
	b.add("typing.test", "2026-10-26T09:00:00-07:00", runData(93, 97, ""))
	sk = typing(t, Build(b.evs, nil, Source{}, now, la))
	if !sk.Passed || sk.PassedAt != "2026-10-26T09:00:00-07:00" {
		t.Fatalf("passed %v at %s", sk.Passed, sk.PassedAt)
	}
}

func TestNoTargetMeansNotPassed(t *testing.T) {
	b := base()
	b.add("typing.test", "2026-10-29", runData(200, 100, ""))
	sk := typing(t, Build(b.evs, nil, Source{}, now, la))
	if sk.Target != nil || sk.Passed {
		t.Fatal("no target set, so nothing passes")
	}
}

func TestDailySeriesAndPillars(t *testing.T) {
	b := base()
	b.add("typing.test", "2026-10-28T22:30:00-07:00", runData(80, 97, ""))
	b.add("typing.test", "2026-10-29", runData(88, 97, ""))
	s := Build(b.evs, nil, Source{}, now, la)
	sk := typing(t, s)
	if len(sk.Daily) != 30 || sk.Daily[29].Date != "2026-10-29" || sk.Daily[28].Date != "2026-10-28" {
		t.Fatalf("daily %+v", sk.Daily[27:])
	}
	if *sk.Daily[28].Best != 80 || *sk.Daily[29].Best != 88 {
		t.Fatal("daily bests")
	}
	if s.Today.TypingTests != 1 {
		t.Fatalf("today %d", s.Today.TypingTests)
	}
	if len(s.Pillars) != 9 || s.Pillars[4].Pillar != "competence" || s.Pillars[4].Events7d != 4 {
		t.Fatalf("pillars %+v", s.Pillars)
	}
}

func TestChatRunMirroredByAPIRunCountsOnce(t *testing.T) {
	b := base()
	b.add("typing.test", "2026-10-29", runData(88, 97, ""))
	b.add("typing.test", "2026-10-29", runData(88, 97, ""))
	b.add("typing.test", "2026-10-29", runData(85, 97, ""))
	api := b.add("typing.test", "2026-10-29T10:15:00-07:00", runData(88.4, 97.2, ""))
	b.evs[len(b.evs)-1].Source = event.Source{Via: "monkeytype-api", Agent: "lq-mt", Ref: "r1"}
	sk := typing(t, Build(b.evs, nil, Source{}, now, la))
	// One 88/97 chat run is the API result; the other 88 and the 85 have no
	// API twin and stay.
	if sk.RunsTotal != 3 || sk.RunsQualifying != 3 || sk.BestEvent != api || *sk.Best != 88.4 {
		t.Fatalf("got total %d qualifying %d best %v from %s", sk.RunsTotal, sk.RunsQualifying, *sk.Best, sk.BestEvent)
	}
}
