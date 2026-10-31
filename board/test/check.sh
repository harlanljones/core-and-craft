#!/usr/bin/env bash
# Compiles and evaluates every widget in widgets/ with the vendored OWL
# runtime (Owl.js under node, the same code the Quickshell board runs)
# against snapshots built by the real acta from seed/, and fails on compile
# errors or ERROR values in the render tree.
#
#   board/test/check.sh      (needs node, jq and bin/acta from `make build`)
set -euo pipefail
here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
root=$(cd -- "$here/../.." && pwd)
owl="$root/board/plugin/owl/Owl.js"
acta="$root/bin/acta"
[ -x "$acta" ] || { echo "no $acta: run make build" >&2; exit 1; }
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
mkdir -p "$here/out"

NOW=1793311200 # 2026-10-29T22:00:00Z: day; the town's night and dusk are checked below

# snap <name> [acta log args...]: a snapshot from the seed plus one optional event.
snap() {
  local name=$1; shift
  export ACTA_DATA="$tmp/$name/data" ACTA_STATE="$tmp/$name/state" ACTA_TZ=UTC
  "$acta" init >/dev/null
  "$acta" ingest "$root"/seed/*.jsonl >/dev/null
  [ $# -gt 0 ] && "$acta" log "$@" >/dev/null
  "$acta" snapshot >/dev/null
  cat "$ACTA_STATE/snapshot.json"
}
seed=$(snap seed)
target=$(snap target skill.target_set --subject skill:typing --agent manual --data '{"metric":"wpm","target":100}')

# busy: the seed plus quests (one done, one auto-proposed pair), proposals from
# agents and two open check-ins, for the town.
busy() {
  export ACTA_DATA="$tmp/busy/data" ACTA_STATE="$tmp/busy/state" ACTA_TZ=UTC
  "$acta" init >/dev/null
  "$acta" ingest "$root"/seed/*.jsonl >/dev/null
  "$acta" quest add --pillar competence --title "15 min typing drill" --priority 1 --every daily --skill skill:typing --rule runs:9 >/dev/null
  "$acta" quest add --pillar recovery --title "Lights out by 22:30" --every daily >/dev/null
  "$acta" quest add --pillar career --title "Ship the board PR" --priority 2 >/dev/null
  "$acta" quest add --pillar relationships --title "Call Mom" --every weekly:mon,thu >/dev/null
  "$acta" quest add --pillar economic --title "Review October spend" --priority 3 >/dev/null
  done=$("$acta" quest add --pillar faith --title "Evening prayer" --every daily | cut -d' ' -f1)
  "$acta" quest done "$done" >/dev/null
  "$acta" quest add --pillar economic --title "Pay the card" --agent duet:economic >/dev/null
  "$acta" quest add --pillar outlook --title "Read 20 pages" --agent duet:outlook >/dev/null
  "$acta" checkin ask --pillar recovery --question "What's your target sleep window?" --agent duet:recovery >/dev/null
  "$acta" checkin ask --pillar time --question "Which hours are protected?" --agent duet:time >/dev/null
  # Two questions to pillar agents: one answered, one still out.
  ask=$("$acta" ask --pillar time --text "Which meeting can go this week?" --via board | cut -d' ' -f1)
  "$acta" log pillar.answered --subject "$ask" --pillar time --agent duet:time --via chat --data '{"text":"The Thursday sync: it repeats the Monday notes."}' >/dev/null
  "$acta" ask --pillar recovery --text "When should screens go off?" --via board >/dev/null
  "$acta" snapshot >/dev/null
  cat "$ACTA_STATE/snapshot.json"
}
busy=$(busy)
# the same day with every pillar levelled up and two quests overdue (a storm), day not yet kept
levels=$(jq '.pillars |= map(.level = 5 | .today = 3 | .events_7d = 9) | .leveled_today = ["competence","faith"] | .xp.level = 12
  | .quests |= map(.overdue_days = 2) | .streak = {days: 23, today: false}' <<<"$busy")

fail=0
check() {
  local name=$1 label=$2 value=$3 at=${4:-$NOW} save=${5:-yes}
  jq -n --slurpfile base "$here/fixture.base.json" --argjson s "$value" --argjson now "$at" \
    '$base[0] + {sources: {s: {value: $s}, now: {value: $now}}}' > "$tmp/fx.json"
  if out=$(node "$here/walk.js" "$owl" "$root/widgets/$name" "$tmp/fx.json" 2>"$tmp/err"); then
    errs=$(grep -o '"ERROR[^"]*"' <<<"$out" | sort -u || true)
    if [ -n "$errs" ]; then echo "FAIL $name [$label]"; echo "$errs" | sed 's/^/   /'; fail=1
    else echo "ok   $name [$label]"; fi
    [ -s "$tmp/err" ] && sed 's/^/   /' "$tmp/err"
    if [ "$save" = yes ]; then echo "$out" > "$here/out/${name%.owl}.$label.json"; fi
  else
    echo "FAIL $name [$label]"; sed 's/^/   /' "$tmp/err"; fail=1
  fi
}
for f in "$root"/widgets/*.owl; do
  n=$(basename "$f")
  check "$n" seed "$seed"
  check "$n" target "$target"
  check "$n" empty null
done

# The town draws from the day's busy data too: night, dusk, a storm with
# every pillar levelled up, and the whole of it with an empty snapshot.
check 10-town.owl busy "$busy"
check 10-town.owl night "$busy" $((NOW - 18 * 3600)) no
check 10-town.owl dusk "$busy" $((NOW - 5 * 3600)) no
check 10-town.owl storm "$levels" $((NOW - 18 * 3600)) no
# Every tap on it, against the acta command (or popup) the spec gives it.
if TZ=UTC node "$here/taps.js" "$owl" "$root/widgets/10-town.owl" <(echo "$busy") ; then :; else fail=1; fi
exit $fail
