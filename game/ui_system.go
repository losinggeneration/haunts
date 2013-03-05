package game

import (
  "path/filepath"
  "time"
  "github.com/MobRulesGames/glop/gin"
  "github.com/MobRulesGames/glop/gui"
  "../texture"
  "../base"
  "github.com/MobRulesGames/opengl/gl"
)

var Restart func()

type systemLayout struct {
  Main Button
  Sub  struct {
    Background texture.Object
    Return     Button
    Save       TextEntry
  }
}

type SystemMenu struct {
  layout      systemLayout
  region      gui.Region
  buttons     []ButtonLike
  mx, my      int
  last_t      int64
  focus       bool
  saved_time  time.Time
  saved_alpha float64
}

func MakeSystemMenu(gp *GamePanel, player *Player) (gui.Widget, error) {
  var sm SystemMenu
  datadir := base.GetDataDir()
  err := base.LoadAndProcessObject(filepath.Join(datadir, "ui", "system", "layout.json"), "json", &sm.layout)
  if err != nil {
    return nil, err
  }

  sm.layout.Main.f = func(interface{}) {}

  sm.buttons = []ButtonLike{
    &sm.layout.Sub.Return,
    &sm.layout.Sub.Save,
  }

  sm.layout.Sub.Return.f = func(_ui interface{}) {
    ui := _ui.(*gui.Gui)
    gp.game.Ents = nil
    gp.game.Think(1) // This should clean things up
    ui.DropFocus()
    Restart()
  }

  sm.layout.Sub.Save.Entry.text = player.Name
  sm.layout.Sub.Save.Button.f = func(interface{}) {
    UpdatePlayer(player, gp.script.L)
    str, err := base.ToGobToBase64(gp.game)
    if err != nil {
      base.Error().Printf("Error gobbing game state: %v", err)
      return
    }
    player.Game_state = str
    player.Name = sm.layout.Sub.Save.Text()
    player.No_init = true
    base.Log().Printf("Saving player: %v", player)
    err = SavePlayer(player)
    if err != nil {
      base.Warn().Printf("Unable to save player: %v", err)
      return
    }
    sm.saved_time = time.Now()
    sm.saved_alpha = 1.0
  }

  return &sm, nil
}

func (sm *SystemMenu) Requested() gui.Dims {
  return gui.Dims{1024, 768}
}

func (sm *SystemMenu) Expandable() (bool, bool) {
  return false, false
}

func (sm *SystemMenu) Rendered() gui.Region {
  return sm.region
}

func (sm *SystemMenu) Think(g *gui.Gui, t int64) {
  if sm.last_t == 0 {
    sm.last_t = t
    return
  }
  dt := t - sm.last_t
  sm.last_t = t
  if sm.mx == 0 && sm.my == 0 {
    sm.mx, sm.my = gin.In().GetCursor("Mouse").Point()
  }
  if sm.focus {
    for _, button := range sm.buttons {
      button.Think(sm.region.X, sm.region.Y, sm.mx, sm.my, dt)
    }
    // This makes it so that the button lights up while the menu
    sm.layout.Main.Think(0, 0, sm.layout.Main.bounds.x+1, sm.layout.Main.bounds.y+1, dt)
  } else {
    sm.layout.Main.Think(sm.region.X, sm.region.Y, sm.mx, sm.my, dt)
  }

  if sm.saved_time != (time.Time{}) && time.Now().Sub(sm.saved_time).Seconds() > 3 {
    sm.saved_alpha = doApproach(sm.saved_alpha, 0.0, dt)
  }

  sm.focus = (g.FocusWidget() == sm)
}

func (sm *SystemMenu) Respond(g *gui.Gui, group gui.EventGroup) bool {
  cursor := group.Events[0].Key.Cursor()
  if cursor != nil {
    sm.mx, sm.my = cursor.Point()
  }
  if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
    if sm.layout.Main.handleClick(sm.mx, sm.my, g) {
      if sm.focus {
        g.DropFocus()
      } else {
        g.TakeFocus(sm)
      }
      sm.focus = true
      base.Log().Printf("focus: %v %v", sm, g.FocusWidget())
      return true
    }
    if sm.focus {
      hit := false
      for _, button := range sm.buttons {
        if button.handleClick(sm.mx, sm.my, g) {
          hit = true
        }
      }
      if hit {
        return true
      }
    }
  } else {
    hit := false
    for _, button := range sm.buttons {
      if button.Respond(group, nil) {
        hit = true
      }
    }
    if hit {
      return true
    }
  }
  return (g.FocusWidget() == sm)
}

func (sm *SystemMenu) Draw(region gui.Region) {
  sm.region = region
  gl.Color4ub(255, 255, 255, 255)
  x := region.X + region.Dx - sm.layout.Main.Texture.Data().Dx()
  y := region.Y + region.Dy - sm.layout.Main.Texture.Data().Dy()
  sm.layout.Main.RenderAt(x, y)
}

func (sm *SystemMenu) DrawFocused(region gui.Region) {
  sm.region = region
  gl.Color4ub(255, 255, 255, 255)
  x := region.X + region.Dx/2 - sm.layout.Sub.Background.Data().Dx()/2
  y := region.Y + region.Dy/2 - sm.layout.Sub.Background.Data().Dy()/2
  sm.layout.Sub.Background.Data().RenderNatural(x, y)
  for _, button := range sm.buttons {
    button.RenderAt(x, y)
  }

  gl.Color4ub(255, 255, 255, byte(255*sm.saved_alpha))
  d := base.GetDictionary(sm.layout.Sub.Save.Button.Text.Size)
  sx := x + sm.layout.Sub.Save.Entry.X + sm.layout.Sub.Save.Entry.Dx + 10
  sy := y + sm.layout.Sub.Save.Button.Y
  d.RenderString("Game Saved!", float64(sx), float64(sy), 0, d.MaxHeight(), gui.Left)
}

func (sm *SystemMenu) String() string {
  return "system menu"
}
