package game

// import (
//   "github.com/MobRulesGames/glop/gin"
//   "github.com/MobRulesGames/glop/gui"
//   "github.com/MobRulesGames/haunts/house"
//   "github.com/MobRulesGames/haunts/base"
//   "path/filepath"
// )

// type UiSelectSide struct {
//   layout  UiSelectSideLayout
//   region  gui.Region
//   buttons []*Button

//   // mouse position
//   mx, my int
// }

// type UiSelectSideLayout struct {
//   Denizens, Intruders Button
// }

// func MakeUiSelectSide(gp *GamePanel, house_def *house.HouseDef) (gui.Widget, error) {
//   var ui UiSelectSide

//   datadir := base.GetDataDir()
//   err := base.LoadAndProcessObject(filepath.Join(datadir, "ui", "select_side", "config.json"), "json", &ui.layout)
//   if err != nil {
//     return nil, err
//   }

//   ui.region.Dx = 1024
//   ui.region.Dy = 768
//   ui.layout.Denizens.f = func(interface {}) {
//     gp.LoadHouse(house_def, SideHaunt)
//   }
//   ui.layout.Intruders.f = func(interface {}) {
//     gp.LoadHouse(house_def, SideExplorers)
//   }
//   ui.buttons = []*Button{
//     &ui.layout.Denizens,
//     &ui.layout.Intruders,
//   }

//   return &ui, nil
// }

// func (ui *UiSelectSide) String() string {
//   return "ui select side"
// }

// func (ui *UiSelectSide) Requested() gui.Dims {
//   return gui.Dims{1024, 768}
// }

// func (ui *UiSelectSide) Expandable() (bool, bool) {
//   return false, false
// }

// func (ui *UiSelectSide) Rendered() gui.Region {
//   return ui.region
// }

// func (ui *UiSelectSide) Think(g *gui.Gui, dt int64) {
// }

// func (ui *UiSelectSide) Respond(g *gui.Gui, group gui.EventGroup) bool {
//   ui.mx, ui.my = gin.In().GetCursor("Mouse").Point()
//   if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
//     for _, button := range ui.buttons {
//       if button.handleClick(ui.mx, ui.my, nil) {
//         return true
//       }
//     }
//   }
//   for _, button := range ui.buttons {
//     if button.Respond(group, ui) {
//       return true
//     }
//   }
//   return false
// }

// func (ui *UiSelectSide) Draw(region gui.Region) {
//   for _, button := range ui.buttons {
//     button.RenderAt(region.X, region.Y, ui.mx, ui.my)
//   }
// }

// func (ui *UiSelectSide) DrawFocused(region gui.Region) {
// }
