package game

import (
  "errors"
  "fmt"
  gl "github.com/MobRulesGames/gogl/gl21"
  "github.com/MobRulesGames/glop/gin"
  "github.com/MobRulesGames/glop/gui"
  "github.com/MobRulesGames/glop/util/algorithm"
  "github.com/MobRulesGames/haunts/base"
  "github.com/MobRulesGames/haunts/house"
  "github.com/MobRulesGames/haunts/sound"
  "github.com/MobRulesGames/haunts/texture"
  "path/filepath"
)

type placerLayout struct {
  Texture texture.Object
  Face    struct {
    X, Y int
  }
  Points_remaining struct {
    X, Y   int
    String string
    Size   int
  }
  Roster struct {
    X, Y, X2    int
    Max_options int
    Points      struct {
      X_off, Y_off int
      Size         int
    }
  }
  Name   TextArea
  Ap     TextArea
  Hp     TextArea
  Corpus TextArea
  Ego    TextArea
  Undo   Button
  Done   Button
}

type EntityPlacer struct {
  layout       placerLayout
  region       gui.Region
  roster       map[string]int
  roster_names []string
  ents         []*Entity
  ent_buttons  []*Button
  buttons      []*Button
  game         *Game
  hovered      *Entity
  show_points  bool
  points       int
  pattern      string
  mx, my       int
  last_t       int64
}

func MakeEntityPlacer(game *Game, roster_names []string, roster_costs []int, min, max int, pattern string) (*EntityPlacer, <-chan []*Entity, error) {
  var ep EntityPlacer
  err := base.LoadAndProcessObject(filepath.Join(base.GetDataDir(), "ui", "entity_placer", "config.json"), "json", &ep.layout)
  if err != nil {
    return nil, nil, err
  }
  if len(roster_names) != len(roster_costs) {
    return nil, nil, errors.New("Must have as many names as costs.")
  }
  if len(roster_names) <= 0 || len(roster_names) > ep.layout.Roster.Max_options {
    return nil, nil, errors.New(fmt.Sprintf("Can't have more than %d ents in a roster.", ep.layout.Roster.Max_options))
  }

  ep.layout.Undo.valid_func = func() bool {
    return len(ep.ents) > 0
  }
  ep.layout.Undo.f = func(interface{}) {
    ent := ep.ents[len(ep.ents)-1]
    ep.points += ep.roster[ent.Name]
    ep.ents = ep.ents[0 : len(ep.ents)-1]
    algorithm.Choose2(&game.Ents, func(e *Entity) bool { return e != ent })
    game.viewer.RemoveDrawable(ent)
  }

  ep.layout.Done.valid_func = func() bool {
    return ep.points >= 0 && min <= (max-ep.points)
  }
  done := make(chan []*Entity)
  ep.layout.Done.f = func(interface{}) {
    done <- ep.ents
    close(done)
    house.PopSpawnRegexp()
    game.viewer.RemoveDrawable(game.new_ent)
    game.new_ent = nil
  }
  ep.roster_names = roster_names
  ep.roster = make(map[string]int)
  for i, name := range ep.roster_names {
    ep.roster[name] = roster_costs[i]
  }
  ep.game = game
  ep.show_points = !(min == 1 && max == 1)
  ep.points = max
  ep.pattern = pattern
  house.PushSpawnRegexp(ep.pattern)
  x := ep.layout.Roster.X
  for _, name := range ep.roster_names {
    var b Button
    b.X = x
    x += (ep.layout.Roster.X2 - ep.layout.Roster.X) / (ep.layout.Roster.Max_options - 1)
    b.Y = ep.layout.Roster.Y
    ent := Entity{Defname: name}
    base.GetObject("entities", &ent)
    b.Texture = ent.Still
    cost := ep.roster[name]
    b.valid_func = func() bool {
      return ep.points >= cost
    }
    b.f = func(interface{}) {
      ep.game.viewer.RemoveDrawable(ep.game.new_ent)
      ep.game.new_ent = MakeEntity(ent.Name, ep.game)
      ep.game.viewer.AddDrawable(ep.game.new_ent)
    }
    ep.ent_buttons = append(ep.ent_buttons, &b)
  }

  ep.buttons = []*Button{
    &ep.layout.Undo,
    &ep.layout.Done,
  }
  for _, b := range ep.ent_buttons {
    ep.buttons = append(ep.buttons, b)
  }

  return &ep, done, nil
}

func (ep *EntityPlacer) Requested() gui.Dims {
  data := ep.layout.Texture.Data()
  return gui.Dims{data.Dx(), data.Dy()}
}

func (ep *EntityPlacer) Expandable() (bool, bool) {
  return false, false
}

func (ep *EntityPlacer) Rendered() gui.Region {
  return ep.region
}

func (ep *EntityPlacer) Think(g *gui.Gui, t int64) {
  if ep.last_t == 0 {
    ep.last_t = t
    return
  }
  dt := t - ep.last_t
  ep.last_t = t
  if ep.mx == 0 && ep.my == 0 {
    ep.mx, ep.my = gin.In().GetCursor("Mouse").Point()
  }
  for _, button := range ep.buttons {
    button.Think(ep.region.X, ep.region.Y, ep.mx, ep.my, dt)
  }

  hovered := false
  for i, button := range ep.ent_buttons {
    if button.Over(ep.mx, ep.my) {
      hovered = true
      if ep.hovered == nil || ep.roster_names[i] != ep.hovered.Name {
        ep.hovered = MakeEntity(ep.roster_names[i], ep.game)
      }
    }
  }
  if hovered == false {
    ep.hovered = nil
  }
  if ep.hovered != nil {
    ep.hovered.Think(dt)
  }
}

func (ep *EntityPlacer) Respond(g *gui.Gui, group gui.EventGroup) bool {
  cursor := group.Events[0].Key.Cursor()
  if cursor != nil {
    ep.mx, ep.my = cursor.Point()
  }

  // If we're dragging around an ent then we'll update its position here.
  if ep.game.new_ent != nil {
    bx, by := DiscretizePoint32(ep.game.viewer.WindowToBoard(ep.mx, ep.my))
    ep.game.new_ent.X, ep.game.new_ent.Y = float64(bx), float64(by)
  }

  if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
    for _, button := range ep.buttons {
      if button.handleClick(ep.mx, ep.my, nil) {
        return true
      }
    }
  }

  if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
    if pointInsideRect(ep.mx, ep.my, ep.region.X, ep.region.Y, ep.region.Dx, ep.region.Dy) {
      return true
    }
  }

  if ep.game.new_ent == nil {
    return false
  }

  if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
    ent := ep.game.new_ent
    if ep.game.placeEntity(ep.pattern) {
      cost := ep.roster[ent.Name]
      ep.points -= cost
      ep.ents = append(ep.ents, ent)
      sound.PlaySound("Haunts/SFX/UI/Place", 1.0)
      if cost <= ep.points {
        ep.game.new_ent = MakeEntity(ent.Name, ep.game)
        ep.game.viewer.AddDrawable(ep.game.new_ent)
      }
      return true
    }
  }

  return false
}

func (ep *EntityPlacer) Draw(region gui.Region) {
  ep.region = region
  gl.Color4ub(255, 255, 255, 255)
  ep.layout.Texture.Data().RenderNatural(region.X, region.Y)
  for _, button := range ep.buttons {
    button.RenderAt(ep.region.X, ep.region.Y)
  }
  d := base.GetDictionary(ep.layout.Roster.Points.Size)
  x_off := ep.layout.Roster.Points.X_off
  y_off := ep.layout.Roster.Points.Y_off
  for i, button := range ep.ent_buttons {
    cost := ep.roster[ep.roster_names[i]]
    x := float64(button.X + x_off)
    y := float64(button.Y + y_off)
    d.RenderString(fmt.Sprintf("%d", cost), x, y, 0, d.MaxHeight(), gui.Right)
  }
  gl.Color4ub(255, 255, 255, 255)
  var ent *Entity
  if !pointInsideRect(ep.mx, ep.my, region.X, region.Y, region.Dx, region.Dy) {
    ent = ep.game.new_ent
  }
  if ep.hovered != nil {
    ent = ep.hovered
  }
  if ent != nil {
    ent.Still.Data().RenderNatural(ep.layout.Face.X, ep.layout.Face.Y)
    ep.layout.Name.RenderString(ent.Name)
    ep.layout.Ap.RenderString(fmt.Sprintf("Ap:%d", ent.Stats.ApCur()))
    ep.layout.Hp.RenderString(fmt.Sprintf("Hp:%d", ent.Stats.HpCur()))
    ep.layout.Corpus.RenderString(fmt.Sprintf("Corpus:%d", ent.Stats.Corpus()))
    ep.layout.Ego.RenderString(fmt.Sprintf("Ego:%d", ent.Stats.Ego()))
  }
  if ep.show_points {
    d := base.GetDictionary(ep.layout.Points_remaining.Size)
    x := float64(ep.layout.Points_remaining.X)
    y := float64(ep.layout.Points_remaining.Y)
    d.RenderString(ep.layout.Points_remaining.String, x, y, 0, d.MaxHeight(), gui.Left)
    w := d.StringWidth(ep.layout.Points_remaining.String)
    d.RenderString(fmt.Sprintf("%d", ep.points), x+w, y, 0, d.MaxHeight(), gui.Right)
  }
}

func (ep *EntityPlacer) DrawFocused(region gui.Region) {
}

func (ep *EntityPlacer) String() string {
  return "entity placer"
}
