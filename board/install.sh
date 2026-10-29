#!/usr/bin/env bash
# Installs the LifeQuest board as an Omarchy 4 shell plugin and copies the
# widgets it shows. lq itself comes from `make install`; this never touches it.
#
#   board/install.sh            install (keeps widgets you already edited)
#   board/install.sh --force    overwrite the widgets with the shipped ones
#   board/install.sh --remove   take the plugin out (widgets and data stay)
set -euo pipefail
here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
root=$(cd -- "$here/.." && pwd)
id=harlan.lifequest
cfg=${XDG_CONFIG_HOME:-$HOME/.config}
plugdir=$cfg/omarchy/plugins/$id
widgets=$cfg/lifequest/widgets

say() { printf '\033[1m%s\033[0m\n' "$*"; }
command -v omarchy >/dev/null || { echo "needs Omarchy 4 (the omarchy command)" >&2; exit 1; }

if [ "${1:-}" = "--remove" ]; then
  [ -d "$plugdir" ] && { omarchy plugin remove "$id" || rm -rf "$plugdir"; }
  say "removed the plugin. Widgets stay in $widgets, data in ~/life."
  exit 0
fi

mkdir -p "$widgets"
for f in "$root"/widgets/*.owl; do
  t="$widgets/$(basename "$f")"
  if [ -e "$t" ] && [ "${1:-}" != "--force" ] && ! cmp -s "$f" "$t"; then
    echo "kept your edited $(basename "$f") (--force to replace)"
  else
    cp "$f" "$t"
  fi
done

# A real copy (the validator refuses symlinks), staged outside the plugins
# folder so the shell's watcher never sees half of it.
stage=$(mktemp -d)/$id
cp -r "$here/plugin" "$stage"
omarchy plugin validate "$stage"
mkdir -p "$(dirname "$plugdir")" && rm -rf "$plugdir" && mv "$stage" "$plugdir"
# The shell picks a new plugin up on its next scan; enable needs it known.
command -v omarchy-shell >/dev/null && omarchy-shell shell rescanPlugins >/dev/null 2>&1 || true
omarchy plugin enable "$id"
say "LifeQuest board is on your desktop. Widgets: $widgets (reload as you save)."
command -v lq >/dev/null || echo "lq is not on PATH yet: make install && lq init && lq snapshot"
echo "Off again: omarchy plugin disable $id"
