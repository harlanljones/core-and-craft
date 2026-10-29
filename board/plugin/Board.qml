pragma ComponentBehavior: Bound

// A board of OWL widgets: every *.owl in `dir`, packed first-fit on
// Omalauncher's 6-column grid, each tile framed like an Omarchy window and
// drawn in `palette` (Owl.themeFromToml of the current theme). Shared by the
// Omarchy plugin (Desktop.qml) and the standalone shell (shell.qml).
//
// Adapted from omarchy-widgets' runtimes/quickshell/shell.qml (MIT, Simon
// Schubert): same tile anatomy -- OwlWidget compiles and runs the file,
// OwlNode draws its render tree, a 2 dp frame in the theme's muted color.

import QtQuick
import Qt.labs.folderlistmodel
import "owl" as Owl
import "owl/Owl.js" as OwlJs

Item {
  id: board

  property string dir: ""
  property var palette: OwlJs.themeFromToml("")
  property Owl.OwlHost host: Owl.OwlHost {}
  // Sources run only while this is true (screen locked or hidden: false).
  property bool active: true
  property int columns: 6
  // One grid cell's side in px; the launcher's cell is 69 dp.
  property real pitch: 96
  property real gap: 6
  property real tileOpacity: 0.88

  // A widget's screen { } is laid out for a board 8 rows tall; a board with
  // fewer tiles grows to that while a screen is open, and the tiles stay put.
  property int screenRows: 8

  readonly property real dp: pitch / 69
  implicitWidth: columns * pitch - gap
  implicitHeight: Math.max(rowsUsed, board.screenHeld ? board.screenRows : 0) * pitch - gap

  FolderListModel {
    id: files
    folder: board.dir ? "file://" + board.dir : ""
    nameFilters: ["*.owl"]
    showDirs: false
    sortField: FolderListModel.Name
  }

  // First fit at each widget's own size (or its minSize when it is wider
  // than the board), in file-name order: 10-, 20-, ... as on the phone.
  // Anything the search cannot place goes on the next free row, so a tile
  // never falls back onto another tile's origin.
  readonly property var places: {
    var used = [], out = []
    for (var i = 0; i < tiles.count; i++) {
      var t = tiles.itemAt(i)
      var size = t && t.owl.program ? t.owl.size : [3, 2]
      var w = Math.min(size[0], board.columns), h = size[1]
      var spot = null
      search: for (var y = 0; y < 64; y++) for (var x = 0; x + w <= board.columns; x++) {
        var free = true
        for (var yy = y; yy < y + h && free; yy++) for (var xx = x; xx < x + w; xx++) if (used[yy] && used[yy][xx]) { free = false; break }
        if (!free) continue
        spot = { x: x, y: y, w: w, h: h }
        break search
      }
      if (!spot) {
        var y2 = 0
        for (var r = 0; r < used.length; r++) if (used[r]) y2 = r + 1
        spot = { x: 0, y: y2, w: w, h: h }
      }
      for (var a = spot.y; a < spot.y + h; a++) {
        used[a] = used[a] || []
        for (var b = spot.x; b < spot.x + w; b++) used[a][b] = true
      }
      out.push(spot)
    }
    return out
  }
  // A widget's screen { } covers the board until it closes. The town uses this.
  property string screenPath: ""
  function screenOwner() {
    if (!screenPath) return null
    for (var i = 0; i < tiles.count; i++) {
      var t = tiles.itemAt(i)
      if (t && t.owl && t.owl.path === screenPath) return t.owl
    }
    return null
  }
  // The screen stays up for its fade-out: the last owner and node are kept
  // for the 200 ms the layer takes to go.
  property bool screenHeld: false
  property var heldOwner: null
  property var heldNode: null
  function layerOwner() { return board.screenOwner() || board.heldOwner }
  onScreenNodeChanged: {
    if (board.screenNode) {
      board.heldNode = board.screenNode
      board.heldOwner = board.screenOwner()
      board.screenHeld = true
      holdTimer.stop()
    } else {
      holdTimer.restart()
    }
  }
  Timer { id: holdTimer; interval: 220; onTriggered: board.screenHeld = false }
  // Esc closes the map, as it closes any Omarchy panel.
  Shortcut {
    sequence: "Escape"
    enabled: board.screenNode !== null
    onActivated: { var w = board.screenOwner(); if (w) w.closeScreen() }
  }

  readonly property var screenNode: {
    var w = board.screenOwner()
    if (!w || !w.screenOpen || !w.screenTree) return null
    void w.generation
    var kids = w.screenTree.children || []
    if (!kids.length) return null
    return kids.length === 1 ? kids[0] : { kind: "Column", p: { gap: 0 }, h: {}, a: {}, c: kids }
  }

  readonly property int rowsUsed: {
    var rows = 0
    for (var i = 0; i < places.length; i++) rows = Math.max(rows, places[i].y + places[i].h)
    return Math.max(rows, 1)
  }

  Repeater {
    id: tiles
    model: files
    delegate: Rectangle {
      id: tile
      required property string filePath
      required property int index
      readonly property alias owl: widget
      readonly property var place: board.places[tile.index] || { x: 0, y: tile.index * 2, w: Math.min(3, board.columns), h: 2 }
      x: tile.place.x * board.pitch
      y: tile.place.y * board.pitch
      width: tile.place.w * board.pitch - board.gap
      height: tile.place.h * board.pitch - board.gap
      color: widget.alpha(widget.ink("background"), board.tileOpacity)
      clip: true
      // The other tiles don't show while a screen is open: they fade out and back.
      opacity: board.screenNode === null ? 1 : 0
      visible: opacity > 0
      Behavior on opacity { NumberAnimation { duration: 160; easing.type: Easing.OutCubic } }

      Connections {
        target: widget
        function onScreenRequested() { board.screenPath = widget.path }
        function onScreenClosed() { if (board.screenPath === widget.path) board.screenPath = "" }
      }

      Owl.OwlWidget {
        id: widget
        anchors.fill: parent
        anchors.margins: (widget.program ? widget.program.padding : 12) * widget.dp
        path: tile.filePath
        host: board.host
        palette: board.palette
        active: board.active
        cols: tile.place.w
        rows: tile.place.h
        dp: board.dp
        screenWidth: board.width
        screenHeight: board.height
      }

      TapHandler {
        id: tileTap
        enabled: !!(widget.tree && widget.tree.onTap)
        onTapped: (p) => widget.tree.onTap(p.position.x / widget.dp, p.position.y / widget.dp)
      }
      // A tile that opens something answers the pointer like Omarchy's own: a
      // pointing hand, a wash on hover, a deeper one while pressed.
      HoverHandler {
        id: tileHover
        enabled: tileTap.enabled
        cursorShape: Qt.PointingHandCursor
      }

      Owl.OwlNode {
        anchors.fill: widget
        visible: !widget.error && !!widget.tree
        widget: widget
        node: !widget.tree ? null : widget.tree.children.length === 1 ? widget.tree.children[0]
          : { kind: "Column", p: { gap: 0 }, h: {}, a: {}, c: widget.tree.children }
      }

      // A widget that does not compile says why in its tile (and in
      // .errors/<file>.txt beside it, for an agent to read).
      Text {
        anchors.fill: widget
        visible: !!widget.error
        text: widget.error
        color: widget.ink("red")
        font.family: board.host.monoFamily
        font.pixelSize: 10
        wrapMode: Text.Wrap
      }

      Rectangle {
        anchors.fill: parent
        color: widget.ink("foreground")
        visible: opacity > 0
        opacity: !tileTap.enabled ? 0 : tileTap.pressed ? 0.12 : tileHover.hovered ? 0.05 : 0
        Behavior on opacity { NumberAnimation { duration: 120; easing.type: Easing.OutCubic } }
      }

      // Omalauncher's frame: square, 2 dp, the theme's muted color. It brightens
      // to the accent while the pointer is over a tile that takes a tap.
      Rectangle {
        anchors.fill: parent
        color: "transparent"
        border.width: Math.max(1, 2 * widget.dp)
        border.color: tileTap.enabled && tileHover.hovered ? widget.ink("accent") : widget.ink("muted")
        Behavior on border.color { ColorAnimation { duration: 120 } }
      }
    }
  }

  Loader {
    anchors.fill: parent
    z: 2
    active: board.screenHeld
    sourceComponent: screenLayer
  }

  Component {
    id: screenLayer
    Rectangle {
      id: layer
      // Opens with a 160 ms fade and a small settle from 97.5%; closes by the reverse.
      property bool open: false
      Component.onCompleted: open = Qt.binding(function() { return board.screenNode !== null })
      opacity: layer.open ? 1 : 0
      scale: layer.open ? 1 : 0.975
      Behavior on opacity { NumberAnimation { duration: 160; easing.type: Easing.OutCubic } }
      Behavior on scale { NumberAnimation { duration: 200; easing.type: Easing.OutCubic } }
      color: {
        var w = board.layerOwner()
        return w ? w.alpha(w.ink("background"), 0.96) : "transparent"
      }
      // A right click goes back, as in any Omarchy panel; the X and Esc do too.
      TapHandler {
        acceptedButtons: Qt.RightButton
        onTapped: { var w = board.screenOwner(); if (w) w.closeScreen() }
      }
      Owl.OwlNode {
        anchors.fill: parent
        anchors.margins: 12 * board.dp
        widget: board.layerOwner()
        node: board.screenNode || board.heldNode
      }
      Rectangle {
        anchors.fill: parent
        color: "transparent"
        border.width: Math.max(1, 2 * board.dp)
        border.color: {
          var w = board.layerOwner()
          return w ? w.ink("muted") : "transparent"
        }
      }
    }
  }
}
