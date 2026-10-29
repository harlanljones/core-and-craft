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

# snap <name> [acta log args...]: a snapshot from the seed plus one optional event.
snap() {
  local name=$1; shift
  export ACTA_DATA="$tmp/$name/data" ACTA_STATE="$tmp/$name/state" ACTA_TZ=UTC
  "$acta" init >/dev/null
  cp "$root"/seed/*.jsonl "$ACTA_DATA/inbox/"
  "$acta" ingest >/dev/null
  [ $# -gt 0 ] && "$acta" log "$@" >/dev/null
  "$acta" snapshot >/dev/null
  cat "$ACTA_STATE/snapshot.json"
}
seed=$(snap seed)
target=$(snap target skill.target_set --subject skill:typing --agent manual --data '{"metric":"wpm","target":100}')

fail=0
check() {
  local name=$1 label=$2 value=$3
  jq -n --slurpfile base "$here/fixture.base.json" --argjson s "$value" \
    '$base[0] + {sources: {s: {value: $s}}}' > "$tmp/fx.json"
  if out=$(node "$here/walk.js" "$owl" "$root/widgets/$name" "$tmp/fx.json" 2>"$tmp/err"); then
    errs=$(grep -o '"ERROR[^"]*"' <<<"$out" | sort -u || true)
    if [ -n "$errs" ]; then echo "FAIL $name [$label]"; echo "$errs" | sed 's/^/   /'; fail=1
    else echo "ok   $name [$label]"; fi
    [ -s "$tmp/err" ] && sed 's/^/   /' "$tmp/err"
    echo "$out" > "$here/out/${name%.owl}.$label.json"
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
exit $fail
