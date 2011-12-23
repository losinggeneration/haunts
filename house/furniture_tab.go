package house

import (
  "glop/util/algorithm"
  "glop/gui"
  "glop/gin"
  "path/filepath"
  "time"
  "haunts/base"
)

type FurniturePanel struct {
  *gui.VerticalTable
  name       *gui.TextEditLine
  room_size  *gui.ComboBox
  floor_path *gui.FileWidget
  wall_path  *gui.FileWidget
  themes     *gui.CheckBoxes
  sizes      *gui.CheckBoxes
  decor      *gui.CheckBoxes

  Room       *Room
  RoomViewer *RoomViewer

  // If we're moving around temporary objects in the room or something they'll
  // be stored temporarily here
  object *Furniture

  // If we're in the middle of moving an object and this widget gets collapsed
  // we want to put the object back where it was before we started dragging it.
  prev_object *Furniture

  // Distance from the mouse to the center of the object, in board coordinates
  drag_anchor struct{ x,y float32 }

  // True iff the selected object should be placed when the mouse button is
  // released.  If false this object will be placed when the mouse button is
  // clicked.
  drop_on_release bool
}

func (w *FurniturePanel) Collapse() {
  if w.object != nil && w.prev_object != nil {
    w.RoomViewer.AddFurniture(w.prev_object)
  }
  w.RoomViewer.SetTempObject(nil)
  w.prev_object = nil
  w.object = nil
}
func (w *FurniturePanel) Expand() {
  w.RoomViewer.SetEditMode(editFurniture)
}

func makeFurniturePanel(room *Room, viewer *RoomViewer, datadir string) *FurniturePanel {
  var fp FurniturePanel
  fp.Room = room
  fp.RoomViewer = viewer
  if room.Name == "" {
    room.Name = "name"
  }
  fp.name = gui.MakeTextEditLine("standard", room.Name, 300, 1, 1, 1, 1)  

  if room.Floor_path == "" {
    room.Floor_path = datadir
  }
  fp.floor_path = gui.MakeFileWidget(room.Floor_path, imagePathFilter)

  if room.Wall_path == "" {
    room.Wall_path = datadir
  }
  fp.wall_path = gui.MakeFileWidget(room.Wall_path, imagePathFilter)

  fp.room_size = gui.MakeComboTextBox(algorithm.Map(tags.RoomSizes, []string{}, func(a interface{}) interface{} { return a.(RoomSize).String() }).([]string), 300)
  for i := range tags.RoomSizes {
    if tags.RoomSizes[i].String() == room.Size.String() {
      fp.room_size.SetSelectedIndex(i)
      break
    }
  }
  fp.themes = gui.MakeCheckTextBox(tags.Themes, 300, room.Themes)
  fp.sizes = gui.MakeCheckTextBox(tags.HouseSizes, 300, room.Sizes)
  fp.decor = gui.MakeCheckTextBox(tags.Decor, 300, room.Decor)

  fp.VerticalTable = gui.MakeVerticalTable()
  fp.VerticalTable.Params().Spacing = 3  
  fp.VerticalTable.Params().Background.R = 0.3
  fp.VerticalTable.Params().Background.B = 1
  fp.VerticalTable.AddChild(fp.name)
  fp.VerticalTable.AddChild(fp.floor_path)
  fp.VerticalTable.AddChild(fp.wall_path)
  fp.VerticalTable.AddChild(fp.room_size)
  fp.VerticalTable.AddChild(fp.themes)
  fp.VerticalTable.AddChild(fp.sizes)
  fp.VerticalTable.AddChild(fp.decor)
  fnames := GetAllFurnitureNames()
  for i := range fnames {
    name := fnames[i]
    fp.VerticalTable.AddChild(gui.MakeButton("standard", name, 300, 1, 1, 1, 1, func(t int64) {
      f := MakeFurniture(name)
      if f == nil { return }
      fp.object = f
      fp.drop_on_release = false
      dx,dy := fp.object.Dims()
      fp.drag_anchor.x = float32(dx - 1) / 2
      fp.drag_anchor.y = float32(dy - 1) / 2
      fp.RoomViewer.SetTempObject(fp.object)
    }))
  }
  fp.VerticalTable.AddChild(gui.MakeButton("standard", "Save!", 300, 1, 1, 1, 1, func(t int64) {
    target_path := room.Save(datadir, time.Now().UnixNano())
    if target_path != "" {
      base.SetStoreVal("last room path", target_path)
      // The paths can change when we save them so we should update the widgets
      if !filepath.IsAbs(room.Floor_path) {
        room.Floor_path = filepath.Join(target_path, room.Floor_path)
        fp.floor_path.SetPath(room.Floor_path)
      }
      if !filepath.IsAbs(room.Wall_path) {
        room.Wall_path = filepath.Join(target_path, room.Wall_path)
        fp.wall_path.SetPath(room.Wall_path)
      }
    }
  }))
  return &fp
}

func (w *FurniturePanel) Respond(ui *gui.Gui, group gui.EventGroup) bool {
  if w.VerticalTable.Respond(ui, group) {
    return true
  }
  if found,event := group.FindEvent(gin.Escape); found && event.Type == gin.Press {
    if w.object != nil {
      w.RoomViewer.SetTempObject(nil)
      w.object = nil
    }
    return true
  }
  if found,event := group.FindEvent(gin.MouseLButton); found {
    if w.object != nil && (event.Type == gin.Press || (event.Type == gin.Release && w.drop_on_release)) {
      w.RoomViewer.SetTempObject(nil)
      w.RoomViewer.AddFurniture(w.object)
      w.Room.Furniture = append(w.Room.Furniture, w.object)
      w.object = nil
    } else if w.object == nil && event.Type == gin.Press {
      bx,by := w.RoomViewer.WindowToBoard(event.Key.Cursor().Point())
      w.object = w.RoomViewer.SelectFurnitureAt(event.Key.Cursor().Point())
      if w.object != nil {
        w.prev_object = new(Furniture)
        *w.prev_object = *w.object
      }
      w.Room.Furniture = algorithm.Choose(w.Room.Furniture, func(a interface{}) bool {
        return a.(*Furniture) != w.object
      }).([]*Furniture)
      w.drop_on_release = true
      if w.object != nil {
        px,py := w.object.Pos()
        w.drag_anchor.x = bx - float32(px) - 0.5
        w.drag_anchor.y = by - float32(py) - 0.5
      }
    }
    return true
  }
  return false
}

func (w *FurniturePanel) Think(ui *gui.Gui, t int64) {
  if w.object != nil {
    mx,my := gin.In().GetCursor("Mouse").Point()
    bx,by := w.RoomViewer.WindowToBoard(mx, my)
    w.object.X = int(bx - w.drag_anchor.x)
    w.object.Y = int(by - w.drag_anchor.y)
    w.RoomViewer.MoveFurniture()
  }
  w.VerticalTable.Think(ui, t)
  w.Room.Name = w.name.GetText()

  w.Room.Resize(tags.RoomSizes[w.room_size.GetComboedIndex()])
  w.RoomViewer.SetDims(w.Room.Size.Dx, w.Room.Size.Dy)
  if w.Room.Size.Dx + w.Room.Size.Dy != len(w.Room.WallData) {
    wall_data := make([]WallData, w.Room.Size.Dx + w.Room.Size.Dy)
    for i := 0; i < len(wall_data) && i < len(w.Room.WallData); i++ {
      wall_data[i] = w.Room.WallData[i]
    }
    w.Room.WallData = wall_data
  }

  w.Room.Floor_path = w.floor_path.GetPath()
  w.Room.Wall_path = w.wall_path.GetPath()

  w.RoomViewer.ReloadFloor(w.Room.Floor_path)
  w.RoomViewer.ReloadWall(w.Room.Wall_path)


  for i := range tags.Themes {
    selected := false
    for _,j := range w.themes.GetSelectedIndexes() {
      if j == i {
        selected = true
        break
      }
    }
    if selected {
      w.Room.Themes[tags.Themes[i]] = true
    } else if _,ok := w.Room.Themes[tags.Themes[i]]; ok {
      delete(w.Room.Themes, tags.Themes[i])
    }
  }

  for i := range tags.Decor {
    selected := false
    for _,j := range w.decor.GetSelectedIndexes() {
      if j == i {
        selected = true
        break
      }
    }
    if selected {
      w.Room.Decor[tags.Decor[i]] = true
    } else if _,ok := w.Room.Decor[tags.Decor[i]]; ok {
      delete(w.Room.Decor, tags.Decor[i])
    }
  }
}

