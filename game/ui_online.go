package game

import (
  "fmt"
  "github.com/MobRulesGames/glop/gin"
  "github.com/MobRulesGames/glop/gui"
  "github.com/MobRulesGames/glop/util/algorithm"
  "github.com/MobRulesGames/haunts/base"
  "github.com/MobRulesGames/haunts/mrgnet"
  "github.com/MobRulesGames/haunts/texture"
  "github.com/MobRulesGames/opengl/gl"
  "path/filepath"
  "time"
)

type gameListBox struct {
  Up     Button
  Down   Button
  Scroll ScrollingRegion
  Title  struct {
    Text string
    Size int
  }

  update chan mrgnet.ListGamesResponse
  time   time.Time
  games  []gameField
}

type gameField struct {
  join, delete ButtonLike
  name         string
  key          mrgnet.GameKey
  game         mrgnet.Game
}

type onlineLayout struct {
  Title struct {
    X, Y    int
    Texture texture.Object
  }
  Background texture.Object
  Back       Button

  User    TextEntry
  NewGame Button

  GameStats struct {
    X, Y, Dx, Dy int
    Size         int
  }

  Error struct {
    X, Y int
    Size int
    err  string
  }

  Text struct {
    String        string
    Size          int
    Justification string
  }

  Unstarted, Active gameListBox
}

type OnlineMenu struct {
  layout  onlineLayout
  region  gui.Region
  buttons []ButtonLike
  mx, my  int
  last_t  int64

  update_user  chan mrgnet.UpdateUserResponse
  update_alpha float64
  update_time  time.Time

  control struct {
    in  chan struct{}
    out chan struct{}
  }

  ui gui.WidgetParent

  hover_game *gameField
}

var net_id mrgnet.NetId

func InsertOnlineMenu(ui gui.WidgetParent) error {
  var sm OnlineMenu
  datadir := base.GetDataDir()
  err := base.LoadAndProcessObject(filepath.Join(datadir, "ui", "start", "online", "layout.json"), "json", &sm.layout)
  if err != nil {
    return err
  }
  sm.buttons = []ButtonLike{
    &sm.layout.Back,
    &sm.layout.Unstarted.Up,
    &sm.layout.Unstarted.Down,
    &sm.layout.Active.Up,
    &sm.layout.Active.Down,
    &sm.layout.User,
    &sm.layout.NewGame,
  }
  sm.control.in = make(chan struct{})
  sm.control.out = make(chan struct{})
  sm.layout.Back.f = func(interface{}) {
    ui.RemoveChild(&sm)
    InsertStartMenu(ui)
  }
  sm.ui = ui

  fmt.Sscanf(base.GetStoreVal("netid"), "%d", &net_id)
  if net_id == 0 {
    net_id = mrgnet.NetId(mrgnet.RandomId())
    base.SetStoreVal("netid", fmt.Sprintf("%d", net_id))
  }

  in_newgame := false
  sm.layout.NewGame.f = func(interface{}) {
    if in_newgame {
      return
    }
    in_newgame = true
    go func() {
      var req mrgnet.NewGameRequest
      req.Id = net_id
      var resp mrgnet.NewGameResponse
      done := make(chan bool, 1)
      go func() {
        mrgnet.DoAction("new", req, &resp)
        done <- true
      }()
      select {
      case <-done:
      case <-time.After(5 * time.Second):
        resp.Err = "Couldn't connect to server."
      }
      <-sm.control.in
      defer func() {
        in_newgame = false
        sm.control.out <- struct{}{}
      }()
      if resp.Err != "" {
        sm.layout.Error.err = resp.Err
        base.Error().Printf("Couldn't make new game: %v", resp.Err)
        return
      }
      ui.RemoveChild(&sm)
      err := InsertMapChooser(
        ui,
        func(name string) {
          ui.AddChild(MakeGamePanel(name, nil, nil, resp.Game_key))
        },
        InsertOnlineMenu,
      )
      if err != nil {
        base.Error().Printf("Error making Map Chooser: %v", err)
      }
    }()
  }

  for _, _glb := range []*gameListBox{&sm.layout.Active, &sm.layout.Unstarted} {
    glb := _glb
    glb.Up.f = func(interface{}) {
      glb.Scroll.Up()
    }
    glb.Up.valid_func = func() bool {
      return glb.Scroll.Height > glb.Scroll.Dy
    }
    glb.Down.f = func(interface{}) {
      glb.Scroll.Down()
    }
    glb.Down.valid_func = func() bool {
      return glb.Scroll.Height > glb.Scroll.Dy
    }

    glb.update = make(chan mrgnet.ListGamesResponse)
  }
  go func() {
    var resp mrgnet.ListGamesResponse
    mrgnet.DoAction("list", mrgnet.ListGamesRequest{Id: net_id, Unstarted: true}, &resp)
    sm.layout.Unstarted.update <- resp
  }()
  go func() {
    var resp mrgnet.ListGamesResponse
    mrgnet.DoAction("list", mrgnet.ListGamesRequest{Id: net_id, Unstarted: false}, &resp)
    sm.layout.Active.update <- resp
  }()

  sm.layout.User.Button.f = func(interface{}) {
    var req mrgnet.UpdateUserRequest
    req.Name = sm.layout.User.Entry.text
    req.Id = net_id
    var resp mrgnet.UpdateUserResponse
    go func() {
      mrgnet.DoAction("user", req, &resp)
      <-sm.control.in
      sm.layout.User.SetText(resp.Name)
      sm.update_alpha = 1.0
      sm.update_time = time.Now()
      sm.control.out <- struct{}{}
    }()
  }
  go func() {
    var resp mrgnet.UpdateUserResponse
    mrgnet.DoAction("user", mrgnet.UpdateUserRequest{Id: net_id}, &resp)
    <-sm.control.in
    sm.layout.User.SetText(resp.Name)
    sm.update_alpha = 1.0
    sm.update_time = time.Now()
    sm.control.out <- struct{}{}
  }()

  ui.AddChild(&sm)
  return nil
}

func (sm *OnlineMenu) Requested() gui.Dims {
  return gui.Dims{1024, 768}
}

func (sm *OnlineMenu) Expandable() (bool, bool) {
  return false, false
}

func (sm *OnlineMenu) Rendered() gui.Region {
  return sm.region
}

type onlineButtonSlice []ButtonLike

func (o onlineButtonSlice) Len() int      { return len(o) }
func (o onlineButtonSlice) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o onlineButtonSlice) Less(i, j int) bool {
  return o[i].(*Button).Text.String < o[j].(*Button).Text.String
}

func (sm *OnlineMenu) Think(g *gui.Gui, t int64) {
  if sm.last_t == 0 {
    sm.last_t = t
    return
  }
  dt := t - sm.last_t
  sm.last_t = t
  if sm.mx == 0 && sm.my == 0 {
    sm.mx, sm.my = gin.In().GetCursor("Mouse").Point()
  }

  done := false
  for !done {
    select {
    case sm.control.in <- struct{}{}:
      <-sm.control.out
    default:
      done = true
    }
  }

  var net_id mrgnet.NetId
  fmt.Sscanf(base.GetStoreVal("netid"), "%d", &net_id)
  for i := range []*gameListBox{&sm.layout.Active, &sm.layout.Unstarted} {
    glb := []*gameListBox{&sm.layout.Active, &sm.layout.Unstarted}[i]
    select {
    case list := <-glb.update:
      glb.games = glb.games[0:0]
      for j := range list.Games {
        var b Button
        var name string
        base.Log().Printf("Adding button: %s", list.Games[j].Name)
        b.Text.Justification = sm.layout.Text.Justification
        b.Text.Size = sm.layout.Text.Size
        if net_id == list.Games[j].Denizens_id {
          name = list.Games[j].Name
        } else {
          name = list.Games[j].Name
        }
        b.Text.String = "Join!"
        game_key := list.Game_keys[j]
        active := (glb == &sm.layout.Active)
        in_joingame := false
        b.f = func(interface{}) {
          if in_joingame {
            return
          }
          in_joingame = true
          if active {
            go func() {
              var req mrgnet.StatusRequest
              req.Id = net_id
              req.Game_key = game_key
              var resp mrgnet.StatusResponse
              done := make(chan bool, 1)
              go func() {
                mrgnet.DoAction("status", req, &resp)
                done <- true
              }()
              select {
              case <-done:
              case <-time.After(5 * time.Second):
                resp.Err = "Couldn't connect to server."
              }
              <-sm.control.in
              defer func() {
                in_joingame = false
                sm.control.out <- struct{}{}
              }()
              if resp.Err != "" || resp.Game == nil {
                sm.layout.Error.err = resp.Err
                base.Error().Printf("Couldn't join game: %v", resp.Err)
                return
              }
              sm.ui.RemoveChild(sm)
              sm.ui.AddChild(MakeGamePanel("", nil, nil, game_key))
            }()
          } else {
            go func() {
              var req mrgnet.JoinGameRequest
              req.Id = net_id
              req.Game_key = game_key
              var resp mrgnet.JoinGameResponse
              done := make(chan bool, 1)
              go func() {
                mrgnet.DoAction("join", req, &resp)
                done <- true
              }()
              select {
              case <-done:
              case <-time.After(5 * time.Second):
                resp.Err = "Couldn't connect to server."
              }
              <-sm.control.in
              defer func() {
                in_joingame = false
                sm.control.out <- struct{}{}
              }()
              if resp.Err != "" || !resp.Successful {
                sm.layout.Error.err = resp.Err
                base.Error().Printf("Couldn't join game: %v", resp.Err)
                return
              }
              sm.ui.RemoveChild(sm)
              sm.ui.AddChild(MakeGamePanel("", nil, nil, game_key))
            }()
          }
        }
        if active {
          d := Button{}
          d.Text.String = "Delete!"
          d.Text.Justification = "right"
          d.Text.Size = sm.layout.Text.Size
          d.f = func(interface{}) {
            go func() {
              var req mrgnet.KillRequest
              req.Id = net_id
              req.Game_key = game_key
              var resp mrgnet.KillResponse
              done := make(chan bool, 1)
              go func() {
                mrgnet.DoAction("kill", req, &resp)
                done <- true
              }()
              select {
              case <-done:
              case <-time.After(5 * time.Second):
                resp.Err = "Couldn't connect to server."
              }
              <-sm.control.in
              if resp.Err != "" {
                sm.layout.Error.err = resp.Err
                base.Error().Printf("Couldn't kill game: %v", resp.Err)
              } else {
                algorithm.Choose2(&glb.games, func(gf gameField) bool {
                  return gf.key != req.Game_key
                })
              }
              sm.control.out <- struct{}{}
            }()
          }
          glb.games = append(glb.games, gameField{&b, &d, name, list.Game_keys[j], list.Games[j]})
        } else {
          glb.games = append(glb.games, gameField{&b, nil, name, list.Game_keys[j], list.Games[j]})
        }
      }
      glb.Scroll.Height = int(base.GetDictionary(sm.layout.Text.Size).MaxHeight() * float64(len(list.Games)))

    default:
    }

    sm.hover_game = nil
    if (gui.Point{sm.mx, sm.my}.Inside(glb.Scroll.Region())) {
      for i := range glb.games {
        game := &glb.games[i]
        var region gui.Region
        region.X = game.join.(*Button).bounds.x
        region.Y = game.join.(*Button).bounds.y
        region.Dx = glb.Scroll.Dx
        region.Dy = int(base.GetDictionary(sm.layout.Text.Size).MaxHeight())
        if (gui.Point{sm.mx, sm.my}.Inside(region)) {
          sm.hover_game = game
        }
        game.join.Think(sm.region.X, sm.region.Y, sm.mx, sm.my, dt)
        if game.delete != nil {
          game.delete.Think(sm.region.X, sm.region.Y, sm.mx, sm.my, dt)
        }
      }
    } else {
      for _, game := range glb.games {
        game.join.Think(sm.region.X, sm.region.Y, 0, 0, dt)
        if game.delete != nil {
          game.delete.Think(sm.region.X, sm.region.Y, 0, 0, dt)
        }
      }
    }
    glb.Scroll.Think(dt)
  }

  if sm.update_alpha > 0.0 && time.Now().Sub(sm.update_time).Seconds() >= 2 {
    sm.update_alpha = doApproach(sm.update_alpha, 0.0, dt)
  }

  for _, button := range sm.buttons {
    button.Think(sm.region.X, sm.region.Y, sm.mx, sm.my, dt)
  }
}

func (sm *OnlineMenu) Respond(g *gui.Gui, group gui.EventGroup) bool {
  cursor := group.Events[0].Key.Cursor()
  if cursor != nil {
    sm.mx, sm.my = cursor.Point()
  }
  if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
    for _, button := range sm.buttons {
      if button.handleClick(sm.mx, sm.my, nil) {
        return true
      }
    }
    for _, glb := range []*gameListBox{&sm.layout.Active, &sm.layout.Unstarted} {
      inside := gui.Point{sm.mx, sm.my}.Inside(glb.Scroll.Region())
      if cursor == nil || inside {
        for _, game := range glb.games {
          if game.join.handleClick(sm.mx, sm.my, nil) {
            return true
          }
          if game.delete != nil && game.delete.handleClick(sm.mx, sm.my, nil) {
            return true
          }
        }
      }
    }
  }

  hit := false
  for _, button := range sm.buttons {
    if button.Respond(group, nil) {
      hit = true
    }
  }
  for _, glb := range []*gameListBox{&sm.layout.Active, &sm.layout.Unstarted} {
    inside := gui.Point{sm.mx, sm.my}.Inside(glb.Scroll.Region())
    if cursor == nil || inside {
      for _, game := range glb.games {
        if game.join.Respond(group, nil) {
          hit = true
        }
        if game.delete != nil && game.delete.Respond(group, nil) {
          hit = true
        }
      }
    }
  }
  if hit {
    return true
  }
  return false
}

func (sm *OnlineMenu) Draw(region gui.Region) {
  sm.region = region
  gl.Color4ub(255, 255, 255, 255)
  sm.layout.Background.Data().RenderNatural(region.X, region.Y)
  title := sm.layout.Title
  title.Texture.Data().RenderNatural(region.X+title.X, region.Y+title.Y)
  for _, button := range sm.buttons {
    button.RenderAt(sm.region.X, sm.region.Y)
  }

  d := base.GetDictionary(sm.layout.Text.Size)
  for _, glb := range []*gameListBox{&sm.layout.Active, &sm.layout.Unstarted} {
    title_d := base.GetDictionary(glb.Title.Size)
    title_x := float64(glb.Scroll.X + glb.Scroll.Dx/2)
    title_y := float64(glb.Scroll.Y + glb.Scroll.Dy)
    gl.Disable(gl.TEXTURE_2D)
    gl.Color4ub(255, 255, 255, 255)
    title_d.RenderString(glb.Title.Text, title_x, title_y, 0, title_d.MaxHeight(), gui.Center)

    sx := glb.Scroll.X
    sy := glb.Scroll.Top()
    glb.Scroll.Region().PushClipPlanes()
    for _, game := range glb.games {
      sy -= int(d.MaxHeight())
      game.join.RenderAt(sx, sy)
      gl.Disable(gl.TEXTURE_2D)
      gl.Color4ub(255, 255, 255, 255)
      d.RenderString(game.name, float64(sx+50), float64(sy), 0, d.MaxHeight(), gui.Left)
      if game.delete != nil {
        game.delete.RenderAt(sx+50+glb.Scroll.Dx-100, sy)
      }
    }
    glb.Scroll.Region().PopClipPlanes()
  }

  gl.Color4ub(255, 255, 255, byte(255*sm.update_alpha))
  sx := sm.layout.User.Entry.X + sm.layout.User.Entry.Dx + 10
  sy := sm.layout.User.Button.Y
  d.RenderString("Name Updated", float64(sx), float64(sy), 0, d.MaxHeight(), gui.Left)

  if sm.hover_game != nil {
    game := sm.hover_game
    gl.Disable(gl.TEXTURE_2D)
    gl.Color4ub(255, 255, 255, 255)
    d := base.GetDictionary(sm.layout.GameStats.Size)
    x := float64(sm.layout.GameStats.X + sm.layout.GameStats.Dx/2)
    y := float64(sm.layout.GameStats.Y+sm.layout.GameStats.Dy) - d.MaxHeight()

    if game.game.Denizens_id == net_id {
      d.RenderString("You: Denizens", x, y, 0, d.MaxHeight(), gui.Center)
    } else {
      d.RenderString("You: Intruders", x, y, 0, d.MaxHeight(), gui.Center)
    }
    y -= d.MaxHeight()
    if game.game.Denizens_id == net_id {
      var opponent string
      if game.game.Intruders_name == "" {
        opponent = "no opponent yet"
      } else {
        opponent = fmt.Sprintf("Vs: %s", game.game.Intruders_name)
      }
      d.RenderString(opponent, x, y, 0, d.MaxHeight(), gui.Center)
    } else {
      d.RenderString(fmt.Sprintf("Vs: %s", game.game.Denizens_name), x, y, 0, d.MaxHeight(), gui.Center)
    }
    y -= d.MaxHeight()
    if (game.game.Denizens_id == net_id) == (len(game.game.Execs)%2 == 0) {
      d.RenderString("Your move", x, y, 0, d.MaxHeight(), gui.Center)
    } else {
      d.RenderString("Their move", x, y, 0, d.MaxHeight(), gui.Center)
    }
  }

  if sm.layout.Error.err != "" {
    gl.Color4ub(255, 0, 0, 255)
    l := sm.layout.Error
    d := base.GetDictionary(l.Size)
    d.RenderString(fmt.Sprintf("ERROR: %s", l.err), float64(l.X), float64(l.Y), 0, d.MaxHeight(), gui.Left)
  }
}

func (sm *OnlineMenu) DrawFocused(region gui.Region) {
}

func (sm *OnlineMenu) String() string {
  return "online menu"
}
