package game

import (
  "fmt"
  gl "github.com/MobRulesGames/gogl/gl21"
  "github.com/MobRulesGames/glop/gin"
  "github.com/MobRulesGames/glop/gui"
  "github.com/MobRulesGames/glop/util/algorithm"
  "github.com/MobRulesGames/haunts/base"
  "github.com/MobRulesGames/haunts/sound"
  "github.com/MobRulesGames/haunts/texture"
  "math"
  "path/filepath"
)

type Option interface {
  // index is the index of this option into the layout's array of options,
  // and is also the index into the map selected.  hovered indicates whether
  // or not the mouse is over this particular option.  selected is a map from
  // index to hether or not that option is selected right now.
  String() string
  Draw(x, y, dx int)
  DrawInfo(x, y, dx, dy int)
  Height() int
  Think(hovered, selected, selectable bool, dt int64)
}

type colorOption struct {
  r, g, b, a gl.Ubyte
}

func (co *colorOption) String() string {
  return fmt.Sprintf("ColorOption(%d, %d, %d, %d)", co.r, co.g, co.b, co.a)
}
func (co *colorOption) Think(hovered, selected, selectable bool, dt int64) {
  var target gl.Ubyte
  switch {
  case selected:
    target = 255
  case selectable && hovered:
    target = 200
  case selectable && !hovered:
    target = 150
  default:
    target = 50
  }
  co.a = target
}
func (co *colorOption) Height() int {
  return 125
}
func (co *colorOption) Draw(x, y, dx int) {
  gl.Disable(gl.TEXTURE_2D)
  gl.Color4ub(co.r, co.g, co.b, co.a)
  gl.Begin(gl.QUADS)
  gl.Vertex2i(gl.Int(x), gl.Int(y))
  gl.Vertex2i(gl.Int(x), gl.Int(y+co.Height()))
  gl.Vertex2i(gl.Int(x+dx), gl.Int(y+co.Height()))
  gl.Vertex2i(gl.Int(x+dx), gl.Int(y))
  gl.End()
}
func (co *colorOption) DrawInfo(x, y, dx, dy int) {
  gl.Disable(gl.TEXTURE_2D)
  gl.Color4ub(co.r, co.g, co.b, co.a)
  gl.Begin(gl.QUADS)
  gl.Vertex2i(gl.Int(x), gl.Int(y))
  gl.Vertex2i(gl.Int(x), gl.Int(y+dy))
  gl.Vertex2i(gl.Int(x+dx), gl.Int(y+dy))
  gl.Vertex2i(gl.Int(x+dx), gl.Int(y))
  gl.End()
}

// A Selector determines whether a particular index can be clicked to toggle
// whether or not it is selected.
//
// index: The index of the option that the user is trying to select.  If index
// is -1 the function should return whether or not the current selected map is
// valid.  If index is -1 doit will be false.
//
// selected: a map from index to whether or not that index is already selected
// only selected indices should be stored in the map, when an index is
// deselected it should be removed from the map.
//
// doit: if this is true this function should also add/remove index from
// selected.
type Selector func(index int, selected map[int]bool, doit bool) bool

func SelectInRange(min, max int) Selector {
  return func(index int, selected map[int]bool, doit bool) (valid bool) {
    if index == -1 {
      valid = (len(selected) >= min && len(selected) <= max)
    } else {
      if _, ok := selected[index]; ok {
        valid = true
      } else {
        valid = len(selected) < max
      }
    }
    if doit && valid {
      if _, ok := selected[index]; ok {
        delete(selected, index)
      } else {
        selected[index] = true
      }
    }
    return
  }
}

func SelectExactlyOne(index int, selected map[int]bool, doit bool) (valid bool) {
  if index == -1 {
    valid = (len(selected) == 1)
  } else {
    valid = true
  }
  if doit {
    var other int
    for k, _ := range selected {
      other = k
    }
    delete(selected, other)
    selected[index] = true
  }
  return
}

type chooseLayout struct {
  Background texture.Object

  Scroller struct {
    X, Y    int
    Texture texture.Object
  }

  Options ScrollingRegion

  Up, Down, Back, Next Button

  Info struct {
    X, Y, Dx, Dy int
  }
}

type OptionBasic struct {
  Id           string
  Small        texture.Object
  Large        texture.Object
  Text         string
  Size         int
  alpha        byte
  was_over     bool
  was_selected bool
}

func (ob *OptionBasic) String() string {
  return ob.Id
}
func (ob *OptionBasic) Draw(x, y, dx int) {
  gl.Color4ub(255, 255, 255, gl.Ubyte(ob.alpha))
  ob.Small.Data().RenderNatural(x, y)
}
func (ob *OptionBasic) DrawInfo(x, y, dx, dy int) {
  gl.Color4ub(255, 255, 255, 255)
  tx := x + (dx-ob.Large.Data().Dx())/2
  ty := y + dy - ob.Large.Data().Dy()
  ob.Large.Data().RenderNatural(tx, ty)
  d := base.GetDictionary(ob.Size)
  d.RenderParagraph(ob.Text, float64(x), float64(y+dy-ob.Large.Data().Dy())-d.MaxHeight(), 0, float64(dx), d.MaxHeight(), gui.Left, gui.Top)
}
func (ob *OptionBasic) Height() int {
  return ob.Small.Data().Dy()
}
func (ob *OptionBasic) Think(hovered, selected, selectable bool, dt int64) {
  if selectable && hovered && !ob.was_over {
    sound.PlaySound("Haunts/SFX/UI/Tick", 0.75)
  }
  ob.was_over = hovered
  if ob.was_selected != selected {
    sound.PlaySound("Haunts/SFX/UI/Select", 0.75)
  }
  ob.was_selected = selected
  switch {
  case selected:
    ob.alpha = 255
  case selectable && hovered:
    ob.alpha = 200
  case selectable && !hovered:
    ob.alpha = 150
  default:
    ob.alpha = 50
  }
}

type Chooser struct {
  layout             chooseLayout
  region             gui.Region
  buttons            []*Button
  non_scroll_buttons []*Button
  options            []Option
  selected           map[int]bool
  selector           Selector
  min, max           int
  info_region        gui.Region
  mx, my             int

  last_t int64
}

func InsertMapChooser(ui gui.WidgetParent, chosen func(string), resert func(ui gui.WidgetParent) error) error {
  var bops []OptionBasic
  datadir := base.GetDataDir()
  err := base.LoadAndProcessObject(filepath.Join(datadir, "ui", "start", "versus", "map_select.json"), "json", &bops)
  if err != nil {
    base.Error().Printf("Unable to insert MapChooser: %v", err)
    return err
  }
  var opts []Option
  algorithm.Map2(bops, &opts, func(ob OptionBasic) Option { return &ob })
  for _, opt := range opts {
    base.Log().Printf(opt.String())
  }

  var ch Chooser
  err = base.LoadAndProcessObject(filepath.Join(datadir, "ui", "chooser", "layout.json"), "json", &ch.layout)
  if err != nil {
    base.Error().Printf("Unable to insert MapChooser: %v", err)
    return err
  }
  ch.options = opts
  ch.buttons = []*Button{
    &ch.layout.Up,
    &ch.layout.Down,
    &ch.layout.Back,
    &ch.layout.Next,
  }
  ch.non_scroll_buttons = []*Button{
    &ch.layout.Back,
    &ch.layout.Next,
  }
  ch.layout.Up.f = func(interface{}) {
    ch.layout.Options.Up()
  }
  ch.layout.Down.f = func(interface{}) {
    ch.layout.Options.Down()
  }
  ch.selected = make(map[int]bool)
  ch.layout.Back.f = func(interface{}) {
    ui.RemoveChild(&ch)
    err := resert(ui)
    if err != nil {
      base.Error().Printf("Unable to make Start Menu: %v", err)
      return
    }
  }
  ch.layout.Next.f = func(interface{}) {
    for i := range ch.options {
      if ch.selected[i] {
        ui.RemoveChild(&ch)
        chosen(ch.options[i].String())
      }
    }
  }
  ch.layout.Next.valid_func = func() bool {
    return ch.selector(-1, ch.selected, false)
  }
  ch.min, ch.max = 1, 1
  if ch.min == 1 && ch.max == 1 {
    ch.selector = SelectExactlyOne
  } else {
    ch.selector = SelectInRange(ch.min, ch.max)
  }
  ch.info_region = gui.Region{
    gui.Point{ch.layout.Info.X, ch.layout.Info.Y},
    gui.Dims{ch.layout.Info.Dx, ch.layout.Info.Dy},
  }
  ui.AddChild(&ch)
  return nil
}

func MakeChooser(opts []Option) (*Chooser, <-chan []string, error) {
  var ch Chooser
  datadir := base.GetDataDir()
  err := base.LoadAndProcessObject(filepath.Join(datadir, "ui", "chooser", "layout.json"), "json", &ch.layout)
  if err != nil {
    return nil, nil, err
  }
  ch.options = opts
  ch.buttons = []*Button{
    &ch.layout.Up,
    &ch.layout.Down,
    &ch.layout.Back,
    &ch.layout.Next,
  }
  ch.non_scroll_buttons = []*Button{
    &ch.layout.Back,
    &ch.layout.Next,
  }
  ch.layout.Up.f = func(interface{}) {
    ch.layout.Options.Up()
  }
  ch.layout.Down.f = func(interface{}) {
    ch.layout.Options.Down()
  }
  done := make(chan []string, 1)
  ch.selected = make(map[int]bool)
  ch.layout.Back.f = func(interface{}) {
    done <- nil
    close(done)
  }
  ch.layout.Next.f = func(interface{}) {
    var res []string
    for i := range ch.options {
      if ch.selected[i] {
        res = append(res, ch.options[i].String())
      }
    }
    done <- res
    close(done)
  }
  ch.layout.Next.valid_func = func() bool {
    return ch.selector(-1, ch.selected, false)
  }
  ch.min, ch.max = 1, 1
  if ch.min == 1 && ch.max == 1 {
    ch.selector = SelectExactlyOne
  } else {
    ch.selector = SelectInRange(ch.min, ch.max)
  }
  ch.info_region = gui.Region{
    gui.Point{ch.layout.Info.X, ch.layout.Info.Y},
    gui.Dims{ch.layout.Info.Dx, ch.layout.Info.Dy},
  }
  return &ch, done, nil
}

func (c *Chooser) optionsHeight() int {
  h := 0
  for _, options := range c.options {
    h += options.Height()
  }
  return h
}

type doOnOptionData struct {
  hovered      bool
  selected     bool
  selectable   bool
  x, y, dx, dy int
}

func (c *Chooser) doOnOptions(f func(index int, opt Option, data doOnOptionData)) {
  var data doOnOptionData
  data.x = c.layout.Options.X + c.region.X
  data.y = c.region.Y + c.layout.Options.Top()
  data.dx = c.layout.Options.Dx
  in_box := pointInsideRect(c.mx, c.my, c.layout.Options.X, c.layout.Options.Y, c.layout.Options.Dx, c.layout.Options.Dy)
  for i, option := range c.options {
    data.dy = option.Height()
    data.y -= data.dy
    data.hovered = in_box && pointInsideRect(c.mx, c.my, data.x, data.y, data.dx, data.dy)
    data.selected = c.selected[i]
    data.selectable = c.selector(i, c.selected, false)
    f(i, option, data)
  }
}
func doApproach(cur, target float64, dt int64) float64 {
  dt = 16 // KLUDGE: makes it work even on windows, which doesn't work, who knows why
  delta := target - cur
  delta *= math.Exp(-45.0 / float64(1+dt))
  cur += delta
  if math.Abs(cur-target) < 1e-2 {
    return target
  }
  return cur
}
func (c *Chooser) Requested() gui.Dims {
  return gui.Dims{1024, 768}
}
func (c *Chooser) Expandable() (bool, bool) {
  return false, false
}
func (c *Chooser) Rendered() gui.Region {
  return c.region
}
func (c *Chooser) Think(g *gui.Gui, t int64) {
  if c.last_t == 0 {
    c.last_t = t
    return
  }
  dt := t - c.last_t
  c.last_t = t
  c.layout.Options.Height = c.optionsHeight()
  c.layout.Options.Think(dt)
  if c.mx == 0 && c.my == 0 {
    c.mx, c.my = gin.In().GetCursor("Mouse").Point()
  }
  buttons := c.buttons
  if c.optionsHeight() <= c.layout.Options.Dy {
    buttons = c.non_scroll_buttons
  }
  for _, button := range buttons {
    button.Think(c.region.X, c.region.Y, c.mx, c.my, dt)
  }
  c.doOnOptions(func(index int, opt Option, data doOnOptionData) {
    opt.Think(data.hovered, data.selected, data.selectable, dt)
  })
}
func (c *Chooser) Respond(g *gui.Gui, group gui.EventGroup) bool {
  cursor := group.Events[0].Key.Cursor()
  if cursor != nil {
    c.mx, c.my = cursor.Point()
  }
  if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
    buttons := c.buttons
    if c.optionsHeight() <= c.layout.Options.Dy {
      buttons = c.non_scroll_buttons
    }
    for _, button := range buttons {
      if button.handleClick(c.mx, c.my, nil) {
        return true
      }
      clicked := false
      c.doOnOptions(func(index int, opt Option, data doOnOptionData) {
        if clicked {
          return
        }
        if data.hovered {
          c.selector(index, c.selected, true)
          clicked = true
        }
      })
    }
  }
  return false
}
func (c *Chooser) Draw(region gui.Region) {
  c.region = region
  gl.Color4ub(255, 255, 255, 255)
  c.layout.Background.Data().RenderNatural(region.X, region.Y)
  tex := c.layout.Scroller.Texture.Data()
  tex.RenderNatural(region.X+c.layout.Scroller.X, region.Y+c.layout.Scroller.Y)

  buttons := c.buttons
  if c.optionsHeight() <= c.layout.Options.Dy {
    buttons = c.non_scroll_buttons
  }
  for _, button := range buttons {
    button.RenderAt(region.X, region.Y)
  }

  c.layout.Options.Region().PushClipPlanes()
  hovered := -1
  c.doOnOptions(func(index int, opt Option, data doOnOptionData) {
    if data.hovered {
      hovered = index
    }
    opt.Draw(data.x, data.y, data.dx)
  })
  c.layout.Options.Region().PopClipPlanes()
  c.info_region.PushClipPlanes()
  if hovered != -1 {
    c.options[hovered].DrawInfo(c.layout.Info.X, c.layout.Info.Y, c.layout.Info.Dx, c.layout.Info.Dy)
  } else {
    if c.min == 1 && c.max == 1 && len(c.selected) == 1 {
      var index int
      for index = range c.selected {
      }
      c.options[index].DrawInfo(c.layout.Info.X, c.layout.Info.Y, c.layout.Info.Dx, c.layout.Info.Dy)
    }
  }
  c.info_region.PopClipPlanes()
}
func (c *Chooser) DrawFocused(region gui.Region) {
  c.Draw(region)
}
func (c *Chooser) String() string {
  return "chooser"
}
