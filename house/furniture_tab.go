package house

import (
  "image"
  "github.com/MobRulesGames/glop/gin"
  "github.com/MobRulesGames/glop/gui"
  "github.com/MobRulesGames/glop/util/algorithm"
  "github.com/MobRulesGames/haunts/base"
)

type FurniturePanel struct {
  *gui.VerticalTable
  name       *gui.TextEditLine
  room_size  *gui.ComboBox
  floor_path *gui.FileWidget
  wall_path  *gui.FileWidget

  Room       *roomDef
  RoomViewer *RoomViewer

  // If we're in the middle of moving an object and this widget gets collapsed
  // we want to put the object back where it was before we started dragging it.
  prev_object *Furniture

  // Distance from the mouse to the center of the object, in board coordinates
  drag_anchor struct{ x, y float32 }

  // The piece of furniture that we are currently dragging around
  furniture *Furniture

  key_map base.KeyMap
}

func (w *FurniturePanel) Collapse() {
  w.onEscape()
}
func (w *FurniturePanel) Expand() {
  w.RoomViewer.SetEditMode(editFurniture)
}

func makeFurniturePanel(room *roomDef, viewer *RoomViewer) *FurniturePanel {
  var fp FurniturePanel
  fp.Room = room
  fp.RoomViewer = viewer
  fp.key_map = base.GetDefaultKeyMap()
  if room.Name == "" {
    room.Name = "name"
  }
  fp.name = gui.MakeTextEditLine("standard", room.Name, 300, 1, 1, 1, 1)

  if room.Floor.Path == "" {
    room.Floor.Path = base.Path(datadir)
  }
  fp.floor_path = gui.MakeFileWidget(room.Floor.Path.String(), imagePathFilter)

  if room.Wall.Path == "" {
    room.Wall.Path = base.Path(datadir)
  }
  fp.wall_path = gui.MakeFileWidget(room.Wall.Path.String(), imagePathFilter)

  var args[]string
  algorithm.Map2(tags.RoomSizes, &args, func(a RoomSize) string { return a.String() })
  fp.room_size = gui.MakeComboTextBox(args, 300)
  for i := range tags.RoomSizes {
    if tags.RoomSizes[i].String() == room.Size.String() {
      fp.room_size.SetSelectedIndex(i)
      break
    }
  }
  fp.VerticalTable = gui.MakeVerticalTable()
  fp.VerticalTable.Params().Spacing = 3
  fp.VerticalTable.Params().Background.R = 0.3
  fp.VerticalTable.Params().Background.B = 1
  fp.VerticalTable.AddChild(fp.name)
  fp.VerticalTable.AddChild(fp.floor_path)
  fp.VerticalTable.AddChild(fp.wall_path)
  fp.VerticalTable.AddChild(fp.room_size)

  furn_table := gui.MakeVerticalTable()
  fnames := GetAllFurnitureNames()
  for i := range fnames {
    name := fnames[i]
    furn_table.AddChild(gui.MakeButton("standard", name, 300, 1, 1, 1, 1, func(t int64) {
      f := MakeFurniture(name)
      if f == nil {
        return
      }
      fp.furniture = f
      fp.furniture.temporary = true
      fp.Room.Furniture = append(fp.Room.Furniture, fp.furniture)
      dx, dy := fp.furniture.Dims()
      fp.drag_anchor.x = float32(dx) / 2
      fp.drag_anchor.y = float32(dy) / 2
    }))
  }
  fp.VerticalTable.AddChild(gui.MakeScrollFrame(furn_table, 300, 600))

  return &fp
}

func (w *FurniturePanel) onEscape() {
  if w.furniture != nil {
    if w.prev_object != nil {
      *w.furniture = *w.prev_object
      w.prev_object = nil
    } else {
      algorithm.Choose(&w.Room.Furniture, func(f *Furniture) bool {
        return f != w.furniture
      })
    }
    w.furniture = nil
  }
}

func (w *FurniturePanel) Respond(ui *gui.Gui, group gui.EventGroup) bool {
  if w.VerticalTable.Respond(ui, group) {
    return true
  }

  // On escape we want to revert the furniture we're moving back to where it was
  // and what state it was in before we selected it.  If we don't have any
  // furniture selected then we don't do anything.
  if found, event := group.FindEvent(gin.Escape); found && event.Type == gin.Press {
    w.onEscape()
    return true
  }

  // If we hit delete then we want to remove the furniture we're moving around
  // from the room.  If we're not moving anything around then nothing happens.
  if found, event := group.FindEvent(gin.DeleteOrBackspace); found && event.Type == gin.Press {
    algorithm.Choose(&w.Room.Furniture, func(f *Furniture) bool {
      return f != w.furniture
    })
    w.furniture = nil
    w.prev_object = nil
    return true
  }

  if found, event := group.FindEvent(w.key_map["rotate left"].Id()); found && event.Type == gin.Press {
    if w.furniture != nil {
      w.furniture.RotateLeft()
    }
  }
  if found, event := group.FindEvent(w.key_map["rotate right"].Id()); found && event.Type == gin.Press {
    if w.furniture != nil {
      w.furniture.RotateRight()
    }
  }
  if found, event := group.FindEvent(w.key_map["flip"].Id()); found && event.Type == gin.Press {
    if w.furniture != nil {
      w.furniture.Flip = !w.furniture.Flip
    }
  }
  if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
    if w.furniture != nil {
      if !w.furniture.invalid {
        w.furniture.temporary = false
        w.furniture = nil
      }
    } else if w.furniture == nil {
      bx, by := w.RoomViewer.WindowToBoard(event.Key.Cursor().Point())
      for i := range w.Room.Furniture {
        x, y := w.Room.Furniture[i].Pos()
        dx, dy := w.Room.Furniture[i].Dims()
        if int(bx) >= x && int(bx) < x+dx && int(by) >= y && int(by) < y+dy {
          w.furniture = w.Room.Furniture[i]
          w.prev_object = new(Furniture)
          *w.prev_object = *w.furniture
          w.furniture.temporary = true
          px, py := w.furniture.Pos()
          w.drag_anchor.x = bx - float32(px)
          w.drag_anchor.y = by - float32(py)
          break
        }
      }
    }
    return true
  }
  return false
}

func (w *FurniturePanel) Reload() {
  for i := range tags.RoomSizes {
    if tags.RoomSizes[i].String() == w.Room.Size.String() {
      w.room_size.SetSelectedIndex(i)
      break
    }
  }
  w.name.SetText(w.Room.Name)
  w.floor_path.SetPath(w.Room.Floor.Path.String())
  w.wall_path.SetPath(w.Room.Wall.Path.String())
  w.onEscape()
}

func (w *FurniturePanel) Think(ui *gui.Gui, t int64) {
  if w.furniture != nil {
    mx, my := gin.In().GetCursor("Mouse").Point()
    bx, by := w.RoomViewer.WindowToBoard(mx, my)
    f := w.furniture
    f.X = roundDown(bx - w.drag_anchor.x + 0.5)
    f.Y = roundDown(by - w.drag_anchor.y + 0.5)
    fdx, fdy := f.Dims()
    f.invalid = false
    if f.X < 0 {
      f.invalid = true
    }
    if f.Y < 0 {
      f.invalid = true
    }
    if f.X+fdx > w.Room.Size.Dx {
      f.invalid = true
    }
    if f.Y+fdy > w.Room.Size.Dy {
      f.invalid = true
    }
    for _, t := range w.Room.Furniture {
      if t == f {
        continue
      }
      tdx, tdy := t.Dims()
      r1 := image.Rect(t.X, t.Y, t.X+tdx, t.Y+tdy)
      r2 := image.Rect(f.X, f.Y, f.X+fdx, f.Y+fdy)
      if r1.Overlaps(r2) {
        f.invalid = true
      }
    }
  }

  w.VerticalTable.Think(ui, t)
  w.Room.Resize(tags.RoomSizes[w.room_size.GetComboedIndex()])
  w.Room.Name = w.name.GetText()
  w.Room.Floor.Path = base.Path(w.floor_path.GetPath())
  w.Room.Wall.Path = base.Path(w.wall_path.GetPath())
}
