import QtQuick
import qs.Ui as Ui

Ui.BarWidget {
  id: root
  moduleName: "io.github.huckoder.ptcgl"

  readonly property bool opened: details.opened
  readonly property bool popoutSwitchClosing: details.popoutSwitchClosing

  function open() {
    details.open()
  }

  function close() {
    details.close()
  }

  function toggle() {
    if (root.opened) root.close()
    else root.open()
  }

  function closeForPopoutSwitch() {
    details.closeForPopoutSwitch()
  }

  implicitWidth: button.implicitWidth
  implicitHeight: button.implicitHeight

  Panel {
    id: details
    bar: root.bar
    anchorItem: button
    hostWidget: root
  }

  Ui.WidgetButton {
    id: button
    anchors.fill: parent
    bar: root.bar
    text: root.vertical ? "TCG" : "PTCGL"
    fontSize: root.vertical ? 10 : 12
    tooltipText: "Pokémon TCG Live"
    onPressed: function(buttonCode) {
      if (buttonCode === Qt.LeftButton) root.toggle()
    }
  }
}
