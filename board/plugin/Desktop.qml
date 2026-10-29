import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import qs.Commons
import "owl" as Owl
import "owl/Owl.js" as OwlJs

// Omarchy 4 shell plugin entry (kind "panel", keepLoaded): mounted at
// startup, it puts the OWL board on the desktop of every screen -- a
// layer-shell surface on the Bottom layer, above the wallpaper and beneath
// every window, the pattern anishfn/omarchy-widgets uses.
//
// Only the board's own rectangle takes input (mask), so the rest of the
// desktop stays clickable; keyboard focus is OnDemand so a widget's Input
// can be typed into after a click.
Item {
  id: root

  // Injected by omarchy-shell when the plugin loads.
  property var shell: null
  property var service: null
  property var manifest: null
  property string omarchyPath: ""

  readonly property string home: Quickshell.env("HOME") || ""
  readonly property string stateHome: Quickshell.env("XDG_STATE_HOME") || root.home + "/.local/state"
  readonly property string configHome: Quickshell.env("XDG_CONFIG_HOME") || root.home + "/.config"
  // Your widgets: drop or edit any .owl here and it reloads as you save.
  readonly property string widgetDir: Quickshell.env("ACTA_WIDGETS") || root.configHome + "/core-and-craft/widgets"

  // ---------------------------------------------------------------- settings
  // Override in ~/.config/core-and-craft/board.json:
  //   {"side": "right", "columns": 6, "cell": 96, "margin": 24, "opacity": 0.88, "screens": ["DP-1"]}
  property var cfg: ({})
  FileView {
    path: root.configHome + "/core-and-craft/board.json"
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
    onLoaded: { try { root.cfg = JSON.parse(text()) } catch (e) { root.cfg = {} } }
  }
  function opt(key, fallback) { return root.cfg && root.cfg[key] !== undefined ? root.cfg[key] : fallback }

  // ------------------------------------------------------------------- theme
  // Omarchy 4 keeps the current theme in ~/.local/state/omarchy/current/theme;
  // colors.toml there is the palette OWL's color names map onto.
  property var palette: OwlJs.themeFromToml("")
  FileView {
    path: root.stateHome + "/omarchy/current/theme/colors.toml"
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
    onLoaded: root.palette = OwlJs.themeFromToml(text())
  }

  Owl.OwlHost {
    id: owlHost
    // Follow the shell's font (Omarchy's `omarchy-font-set` changes it).
    monoFamily: Style.font.family
    iconFamily: Style.font.family
    stateDir: root.stateHome + "/core-and-craft/owl"
  }

  Variants {
    model: {
      var want = root.opt("screens", null)
      if (!want || !want.length) return Quickshell.screens
      return Quickshell.screens.filter(function(s) { return want.indexOf(s.name) >= 0 })
    }

    delegate: Component {
      PanelWindow {
        id: surface
        required property var modelData
        screen: modelData
        color: "transparent"
        anchors { top: true; bottom: true; left: true; right: true }

        WlrLayershell.namespace: "core-and-craft"
        WlrLayershell.layer: WlrLayer.Bottom
        WlrLayershell.keyboardFocus: WlrKeyboardFocus.OnDemand
        exclusionMode: ExclusionMode.Normal
        exclusiveZone: 0

        mask: Region { item: board }

        readonly property real margin: root.opt("margin", 24)

        Board {
          id: board
          dir: root.widgetDir
          palette: root.palette
          host: owlHost
          active: surface.visible
          columns: root.opt("columns", 6)
          pitch: root.opt("cell", 96)
          tileOpacity: root.opt("opacity", 0.88)
          width: implicitWidth
          height: implicitHeight
          y: surface.margin
          x: root.opt("side", "right") === "left" ? surface.margin : surface.width - width - surface.margin
        }
      }
    }
  }
}
