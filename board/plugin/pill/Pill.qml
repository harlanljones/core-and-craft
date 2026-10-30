import QtQuick
import Quickshell
import Quickshell.Io
import qs.Commons
import qs.Ui

// Bar pill for the CORE & CRAFT board: the Pax streak, check-ins waiting (?)
// and proposals moored (◇), read from acta's snapshot.json. A click opens the
// town map over the board; a right click asks acta for a fresh snapshot.
BarWidget {
  id: root
  moduleName: "harlan.core-and-craft"

  readonly property string stateHome: Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") || "") + "/.local/state"
  property var snap: ({})
  readonly property int days: snap.streak && snap.streak.days ? snap.streak.days : 0
  readonly property bool kept: snap.streak ? snap.streak.today === true : false
  readonly property int asks: (snap.checkins || []).filter(function(c) { return c.open !== false }).length
  readonly property int proposals: (snap.proposed || []).length
  readonly property string label: "Pax " + root.days
    + (root.asks > 0 ? "  ? " + root.asks : "")
    + (root.proposals > 0 ? "  ◇ " + root.proposals : "")

  FileView {
    path: root.stateHome + "/core-and-craft/snapshot.json"
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
    onLoaded: { try { root.snap = JSON.parse(text()) } catch (e) { root.snap = ({}) } }
  }

  // the screen this pill is on, so the click opens that screen's map
  readonly property string screenName: root.Window.window && root.Window.window.screen ? root.Window.window.screen.name : ""

  implicitWidth: button.implicitWidth
  implicitHeight: button.implicitHeight

  // Text to read, like the clock: WidgetButton sizes to its label, where
  // BarIconButton clamps to an icon slot and shrinks a long label to a speck.
  WidgetButton {
    id: button
    anchors.fill: parent
    bar: root.bar
    text: root.vertical ? "" : root.label
    labelVisible: !root.vertical
    hasVisualContent: text !== ""
    horizontalMargin: 8.75
    verticalPadding: 8.75
    // Kept today reads plain; not yet kept reads as the accent.
    tooltipText: root.kept ? "Pax: streak kept today" : "Pax: streak not yet kept today"

    onPressed: function(b) {
      if (!root.bar) return
      if (b === Qt.RightButton) root.bar.run("acta snapshot")
      else root.bar.run("omarchy-shell -q core-and-craft openMap " + root.screenName)
    }
  }
}
