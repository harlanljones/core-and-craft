// The same board without omarchy-shell: Omarchy 3.x (waybar era), or any
// Hyprland/wlroots session with Quickshell installed.
//
//   quickshell -p ~/.config/lifequest/plugin/shell.qml
//
// Autostart on Omarchy 3.x: add to ~/.config/hypr/autostart.conf
//   exec-once = quickshell -p ~/.config/lifequest/plugin/shell.qml
import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import "owl" as Owl
import "owl/Owl.js" as OwlJs

ShellRoot {
  id: root

  readonly property string home: Quickshell.env("HOME") || ""
  readonly property string stateHome: Quickshell.env("XDG_STATE_HOME") || root.home + "/.local/state"
  readonly property string configHome: Quickshell.env("XDG_CONFIG_HOME") || root.home + "/.config"
  readonly property string widgetDir: Quickshell.env("LIFEQUEST_WIDGETS") || root.configHome + "/lifequest/widgets"

  property var cfg: ({})
  FileView {
    path: root.configHome + "/lifequest/board.json"
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
    onLoaded: { try { root.cfg = JSON.parse(text()) } catch (e) { root.cfg = {} } }
  }
  function opt(key, fallback) { return root.cfg && root.cfg[key] !== undefined ? root.cfg[key] : fallback }

  // Omarchy 4 path first; Omarchy 3.x kept the theme under ~/.config.
  // (Without either, Owl.js falls back to Tokyo Night.)
  property var palette: OwlJs.themeFromToml("")
  FileView {
    id: theme4
    path: root.stateHome + "/omarchy/current/theme/colors.toml"
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
    onLoaded: root.palette = OwlJs.themeFromToml(text())
  }
  FileView {
    path: root.configHome + "/omarchy/current/theme/colors.toml"
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
    onLoaded: if (!theme4.loaded) root.palette = OwlJs.themeFromToml(text())
  }

  Owl.OwlHost {
    id: owlHost
    monoFamily: root.opt("font", "JetBrainsMono Nerd Font")
    iconFamily: root.opt("font", "JetBrainsMono Nerd Font")
    stateDir: root.stateHome + "/lifequest/owl"
  }

  Variants {
    model: Quickshell.screens
    delegate: Component {
      PanelWindow {
        id: surface
        required property var modelData
        screen: modelData
        color: "transparent"
        anchors { top: true; bottom: true; left: true; right: true }
        WlrLayershell.namespace: "lifequest"
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

  IpcHandler {
    target: "lifequest"
    function reload(): string { Quickshell.reload(true); return "ok" }
  }
}
