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
command -v omarchy >/dev/null || { echo "needs Omarchy 4 (the omarchy command)" >&2; exit 1; }

if [ "${1:-}" = "--remove" ]; then
  [ -d "$plugdir" ] && { omarchy plugin remove "$id" || rm -rf "$plugdir"; }
  [ -d "$cfg/omarchy/plugins/$old" ] && { omarchy plugin remove "$old" || rm -rf "$cfg/omarchy/plugins/$old"; }
  say "removed the plugin. Widgets stay in $widgets, data in ~/core-and-craft."
  exit 0
fi

# Only one board on the desktop: drop the pre-rename plugin if it is still there.
if [ -d "$cfg/omarchy/plugins/$old" ]; then omarchy plugin remove "$old" || rm -rf "$cfg/omarchy/plugins/$old"; fi

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
say "CORE & CRAFT board is on your desktop. Widgets: $widgets (reload as you save)."
command -v acta >/dev/null || echo "acta is not on PATH yet: make install && acta init && acta snapshot"
echo "Off again: omarchy plugin disable $id"
