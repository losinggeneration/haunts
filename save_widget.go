package main

import (
  "github.com/MobRulesGames/glop/gui"
  "github.com/MobRulesGames/glop/gin"
)

type SaveWidget struct {
  *gui.VerticalTable
  filename *gui.TextEditLine
  on_save  func(string)
}

func MakeSaveWidget(on_save func(string)) *SaveWidget {
  var sw SaveWidget

  sw.VerticalTable = gui.MakeVerticalTable()
  sw.on_save = on_save
  sw.AddChild(gui.MakeTextLine("standard", "Enter Filename", 300, 1, 1, 1, 1))
  sw.filename = gui.MakeTextEditLine("standard", "filename", 300, 1, 1, 1, 1)
  sw.AddChild(sw.filename)
  sw.AddChild(gui.MakeButton("standard", "Save!", 300, 1, 1, 1, 1, func(int64) {
    sw.on_save(sw.filename.GetText())
  }))

  return &sw
}

func (sw *SaveWidget) Respond(ui *gui.Gui, event_group gui.EventGroup) bool {
  if found, event := event_group.FindEvent(gin.Return); found && event.Type == gin.Press {
    sw.on_save(sw.filename.GetText())
    return true
  }
  sw.VerticalTable.Respond(ui, event_group)
  return true
}
