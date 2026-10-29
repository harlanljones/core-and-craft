#!/usr/bin/env bash
# Installs the CORE & CRAFT board as an Omarchy 4 shell plugin and copies the
# widgets it shows. acta itself comes from `make install`; this never touches it.
#
#   board/install.sh            install (keeps widgets you already edited)
#   board/install.sh --force    overwrite the widgets with the shipped ones
#   board/install.sh --remove   take the plugin out (widgets and data stay)
set -euo pipefail
here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
root=$(cd -- "$here/.." && pwd)
id=harlan.core-and-craft
old=harlan.lifequest # the pre-rename plugin; removed on install and --remove
cfg=${XDG_CONFIG_HOME:-$HOME/.config}
plugdir=$cfg/omarchy/plugins/$id
widgets=$cfg/core-and-craft/widgets

say() { printf '\033[1m%s\033[0m\n' "$*"; }
# drop <id>: disable (clears it from shell.json) and remove a plugin, if present.
drop() {
  omarchy plugin disable "$1" >/dev/null 2>&1 || true
  [ -d "$cfg/omarchy/plugins/$1" ] && omarchy plugin remove "$1" --yes >/dev/null
  return 0
}
command -v omarchy >/dev/null || { echo "needs Omarchy 4 (the omarchy command)" >&2; exit 1; }

if [ "${1:-}" = "--remove" ]; then
  drop "$id"
  drop "$old"
  say "removed the plugin. Widgets stay in $widgets, data in ~/core-and-craft."
  exit 0
fi

# Only one board on the desktop: drop the pre-rename plugin if it is still there.
drop "$old"

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
# The shell picks a new plugin up on a rescan. Right after the copy it may be
# busy reloading and not answer, so rescan and enable in a retry loop.
for _ in $(seq 20); do
  omarchy-shell shell rescanPlugins >/dev/null 2>&1 || true
  omarchy plugin enable "$id" >/dev/null 2>&1 && break
  sleep 0.5
done
omarchy plugin list | grep -q "^$id .*enabled" || { echo "could not enable $id: run omarchy plugin enable $id" >&2; exit 1; }
say "CORE & CRAFT board is on your desktop. Widgets: $widgets (reload as you save)."
command -v acta >/dev/null || echo "acta is not on PATH yet: make install && acta init && acta snapshot"
echo "Off again: omarchy plugin disable $id"
