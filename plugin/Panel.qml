pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import qs.Commons
import qs.Ui as Ui
import "Model.js" as Model

Ui.Panel {
  id: root
  moduleName: "io.github.huckoder.ptcgl"
  manageIpc: false

  property var anchorItem: null
  property var hostWidget: null
  property var launcherInfo: Model.loading()
  property var diagnostics: ({ state: "loading", detail: "Checking game setup…" })
  property bool refreshing: false
  property string queryStage: ""
  property int selectedIndex: 0
  property string selectedAction: ""
  property var expandedSections: ({})
  readonly property string launcherPath: Quickshell.env("HOME") + "/.local/bin/ptcgl-omarchy"
  readonly property string installHelper: Model.localPath(Qt.resolvedUrl("../scripts/plugin-install.sh"))
  // Omarchy exposes these theme groups and bar facades as dynamic QtObjects.
  readonly property var popupColors: Color.popups
  readonly property var themeFont: Style.font
  readonly property var hostBar: root.bar
  readonly property var actions: Model.actions(launcherInfo, diagnostics, refreshing, expandedSections)
  readonly property string summary: Model.title(launcherInfo, diagnostics)
  readonly property string detail: launcherInfo.state === "compatible" ? diagnostics.detail : launcherInfo.detail

  function open() {
    root.controller.show()
    refresh(true)
  }

  function switchPanel(direction) {
    if (hostBar && typeof hostBar.switchPanelFrom === "function")
      return hostBar.switchPanelFrom(root.hostWidget || root, direction)
    return false
  }

  function refresh(checkVersion) {
    if (refreshing) return
    selectedAction = actions[selectedIndex] ? actions[selectedIndex].id : ""
    refreshing = true
    if (checkVersion || launcherInfo.state !== "compatible") {
      launcherInfo = Model.loading()
      diagnostics = { state: "loading", detail: "Checking game setup and activity…" }
      queryStage = "version"
      query.command = ["timeout", "5s", "bash", "-c",
        '[[ -e "$1" ]] || exit 44; exec "$1" version', "ptcgl-companion", launcherPath]
    } else {
      queryStage = "status"
      query.command = ["timeout", "7s", launcherPath, "status", "--json"]
    }
    query.running = true
    queryDeadline.restart()
  }

  function finishQuery(exitCode) {
    if (!refreshing) return
    queryDeadline.stop()
    if (queryStage === "version") {
      launcherInfo = Model.versionResult(queryOutput.text, exitCode)
      if (launcherInfo.state === "compatible") {
        queryStage = "status"
        // Do not restart a Process from inside its exit signal handler.
        Qt.callLater(function() {
          query.command = ["timeout", "7s", root.launcherPath, "status", "--json"]
          query.running = true
          queryDeadline.restart()
        })
        return
      }
    } else {
      diagnostics = Model.statusResult(queryOutput.text, exitCode)
    }
    refreshing = false
    selectedIndex = Model.nextEnabled(actions, -1, 1)
    for (let i = 0; i < actions.length; i++) {
      if (actions[i].id === selectedAction && actions[i].enabled) { selectedIndex = i; break }
    }
  }

  function moveCursor(direction) {
    selectedIndex = Model.nextEnabled(actions, selectedIndex, direction)
    Qt.callLater(ensureSelectionVisible)
  }

  function ensureSelectionVisible() {
    var item = actionRepeater.itemAt(selectedIndex)
    if (!item) return
    if (item.y < scroller.contentY) scroller.contentY = item.y
    else if (item.y + item.height > scroller.contentY + scroller.height)
      scroller.contentY = item.y + item.height - scroller.height
  }

  function activate(index) {
    var action = actions[index]
    if (!action || !action.enabled) return
    if (action.id.indexOf("toggle:") === 0) {
      const section = action.id.slice(7)
      const expanded = Object.assign({}, expandedSections)
      expanded[section] = !expanded[section]
      expandedSections = expanded
      for (let i = 0; i < actions.length; i++) {
        if (actions[i].id === action.id) { selectedIndex = i; break }
      }
      Qt.callLater(ensureSelectionVisible)
      return
    }
    if (action.id === "refresh") {
      refresh(true)
      return
    }
    var command = Model.actionCommand(action.id, launcherPath, installHelper)
    if (command.length === 0) return
    // Detached commands survive popup close, plugin reload, and shell restart.
    Quickshell.execDetached(command)
    root.close()
  }

  Process {
    id: query
    stdout: StdioCollector { id: queryOutput; waitForEnd: true }
    stderr: StdioCollector { waitForEnd: true }
    // Quickshell's qmltypes omit the unused QProcess::ExitStatus enum.
    // qmllint disable signal-handler-parameters
    onExited: function(exitCode) { root.finishQuery(exitCode) }
    // qmllint enable signal-handler-parameters
  }

  // Also recover if the executable (including timeout itself) could not start.
  Timer {
    id: queryDeadline
    interval: 18000
    onTriggered: {
      query.running = false
      root.finishQuery(124)
    }
  }

  Timer {
    interval: 2000
    repeat: true
    running: root.opened && !root.refreshing && root.launcherInfo.state === "compatible" && root.diagnostics.state !== "outdated"
    onTriggered: root.refresh(false)
  }

  Ui.KeyboardPanel {
    id: popup
    anchorItem: root.anchorItem
    owner: root.hostWidget || root
    bar: root.bar
    open: root.opened
    focusTarget: keyCatcher
    contentWidth: popup.fittedContentWidth(Style.space(400))
    contentHeight: popup.fittedContentHeight(content.implicitHeight)

    Ui.PanelKeyCatcher {
      id: keyCatcher
      anchors.fill: parent
      onCloseRequested: root.close()
      onTabRequested: function(direction) { root.switchPanel(direction) }
      onMoveRequested: function(dx, dy) {
        if (dy !== 0) root.moveCursor(dy)
      }
      onActivateRequested: root.activate(root.selectedIndex)
      onTextKey: function(text) {
        if (text === "r") root.refresh(true)
      }

      Flickable {
        id: scroller
        anchors.fill: parent
        clip: true
        contentWidth: width
        contentHeight: content.implicitHeight
        boundsBehavior: Flickable.StopAtBounds

        Column {
          id: content
          width: scroller.width
          spacing: Style.space(4)

          Text {
            width: parent.width
            text: "Pokémon TCG Live"
            textFormat: Text.PlainText
            color: root.popupColors.text
            font.family: root.themeFont.family
            font.pixelSize: root.themeFont.subtitle
            font.bold: true
            wrapMode: Text.WordWrap
          }

          Text {
            width: parent.width
            text: root.summary + (root.launcherInfo.version ? " · " + root.launcherInfo.version : "")
            textFormat: Text.PlainText
            color: Color.accent
            font.family: root.themeFont.family
            font.pixelSize: root.themeFont.bodySmall
            wrapMode: Text.WordWrap
          }

          Text {
            width: parent.width
            text: root.detail
            textFormat: Text.PlainText
            color: root.popupColors.text
            opacity: 0.75
            font.family: root.themeFont.family
            font.pixelSize: root.themeFont.bodySmall
            wrapMode: Text.Wrap
            bottomPadding: Style.space(8)
          }

          Repeater {
            id: actionRepeater
            model: root.actions

            delegate: Column {
              id: actionItem
              required property int index
              required property var modelData
              readonly property bool showSection: !!modelData.section &&
                (index === 0 || root.actions[index - 1].section !== modelData.section)
              width: content.width
              spacing: Style.space(3)

              Ui.PanelSectionHeader {
                visible: actionItem.showSection
                text: actionItem.modelData.section || ""
                foreground: root.popupColors.text
                topPadding: Style.space(8)
              }

              Ui.Button {
                id: actionButton
                width: actionItem.width
                text: actionItem.modelData.text
                enabled: actionItem.modelData.enabled
                opacity: actionButton.enabled ? 1 : 0.4
                foreground: root.popupColors.text
                fontSize: root.themeFont.bodySmall
                bordered: actionItem.modelData.primary === true
                leftAlign: actionItem.modelData.primary !== true
                hasCursor: root.selectedIndex === actionItem.index && actionButton.enabled
                onHovered: function(hovered) {
                  if (hovered && actionButton.enabled) root.selectedIndex = actionItem.index
                }
                onClicked: root.activate(actionItem.index)
              }
            }
          }

          Text {
            visible: !!root.expandedSections.advanced
            width: parent.width
            text: Model.updateDetail(root.diagnostics)
            textFormat: Text.PlainText
            color: root.popupColors.text
            opacity: 0.75
            font.family: root.themeFont.family
            font.pixelSize: root.themeFont.bodySmall
            wrapMode: Text.WordWrap
          }

          Text {
            width: parent.width
            text: "↑↓ / j k · Enter · r refresh · Esc close\nGame patches are checked in-game at launch."
            textFormat: Text.PlainText
            color: root.popupColors.text
            opacity: 0.55
            font.family: root.themeFont.family
            font.pixelSize: root.themeFont.caption
            wrapMode: Text.WordWrap
            topPadding: Style.space(8)
          }
        }
      }
    }
  }
}
