package actions

import (
  "encoding/gob"
  "github.com/MobRulesGames/glop/gin"
  "github.com/MobRulesGames/glop/gui"
  "github.com/MobRulesGames/glop/util/algorithm"
  "github.com/MobRulesGames/haunts/base"
  "github.com/MobRulesGames/haunts/game"
  "github.com/MobRulesGames/haunts/game/status"
  "github.com/MobRulesGames/haunts/house"
  "github.com/MobRulesGames/haunts/texture"
  "github.com/MobRulesGames/opengl/gl"
  lua "github.com/MobRulesGames/golua"
  "path/filepath"
)

func registerAoeAttacks() map[string]func() game.Action {
  aoe_actions := make(map[string]*AoeAttackDef)
  base.RemoveRegistry("actions-aoe_actions")
  base.RegisterRegistry("actions-aoe_actions", aoe_actions)
  base.RegisterAllObjectsInDir("actions-aoe_actions", filepath.Join(base.GetDataDir(), "actions", "aoe_attacks"), ".json", "json")
  makers := make(map[string]func() game.Action)
  for name := range aoe_actions {
    cname := name
    makers[cname] = func() game.Action {
      a := AoeAttack{Defname: cname}
      base.GetObject("actions-aoe_actions", &a)
      if a.Ammo > 0 {
        a.Current_ammo = a.Ammo
      } else {
        a.Current_ammo = -1
      }
      return &a
    }
  }
  return makers
}

func init() {
  game.RegisterActionMakers(registerAoeAttacks)
  gob.Register(&AoeAttack{})
  gob.Register(&aoeExec{})
}

// Aoe Attacks are untargeted and instant, they are also readyable
type AoeAttack struct {
  Defname string
  *AoeAttackDef
  aoeAttackTempData

  Current_ammo int
}
type AoeAttackDef struct {
  Name       string
  Kind       status.Kind
  Ap         int
  Ammo       int // 0 = infinity
  Strength   int
  Range      int
  Diameter   int
  Damage     int
  Animation  string
  Conditions []string
  Texture    texture.Object
  Sounds     map[string]string
}
type aoeAttackTempData struct {
  ent *game.Entity

  // position of the target cell - in the case of an even diameter this will be
  // the lower-left of the center
  tx, ty int

  // All entities in the blast radius - could include the acting entity
  targets []*game.Entity

  exec *aoeExec
}
type aoeExec struct {
  game.BasicActionExec
  X, Y int
}

func (exec aoeExec) Push(L *lua.State, g *game.Game) {
  exec.BasicActionExec.Push(L, g)
  if L.IsNil(-1) {
    return
  }
  L.PushString("Pos")
  game.LuaPushPoint(L, exec.X, exec.Y)
  L.SetTable(-3)
}

func (a *AoeAttack) SoundMap() map[string]string {
  return a.Sounds
}

func (a *AoeAttack) Push(L *lua.State) {
  L.NewTable()
  L.PushString("Type")
  L.PushString("Aoe Attack")
  L.SetTable(-3)
  L.PushString("Name")
  L.PushString(a.Name)
  L.SetTable(-3)
  L.PushString("Ap")
  L.PushInteger(a.Ap)
  L.SetTable(-3)
  L.PushString("Damage")
  L.PushInteger(a.Damage)
  L.SetTable(-3)
  L.PushString("Strength")
  L.PushInteger(a.Strength)
  L.SetTable(-3)
  L.PushString("Range")
  L.PushInteger(a.Range)
  L.SetTable(-3)
  L.PushString("Diameter")
  L.PushInteger(a.Diameter)
  L.SetTable(-3)
  L.PushString("Ammo")
  if a.Current_ammo == -1 {
    L.PushInteger(1000)
  } else {
    L.PushInteger(a.Current_ammo)
  }
  L.SetTable(-3)
}

func (a *AoeAttack) AP() int {
  return a.Ap
}
func (a *AoeAttack) Pos() (int, int) {
  return a.tx, a.ty
}
func (a *AoeAttack) Dims() (int, int) {
  return a.Diameter, a.Diameter
}
func (a *AoeAttack) String() string {
  return a.Name
}
func (a *AoeAttack) Icon() *texture.Object {
  return &a.Texture
}
func (a *AoeAttack) Readyable() bool {
  return true
}
func (a *AoeAttack) Preppable(ent *game.Entity, g *game.Game) bool {
  return a.Current_ammo != 0 && ent.Stats.ApCur() >= a.Ap
}
func (a *AoeAttack) Prep(ent *game.Entity, g *game.Game) bool {
  if !a.Preppable(ent, g) {
    return false
  }
  a.ent = ent
  bx, by := g.GetViewer().WindowToBoard(gin.In().GetCursor("Mouse").Point())
  a.tx = int(bx)
  a.ty = int(by)
  return true
}
func (a *AoeAttack) HandleInput(group gui.EventGroup, g *game.Game) (bool, game.ActionExec) {
  cursor := group.Events[0].Key.Cursor()
  if cursor != nil && cursor.Name() == "Mouse" {
    bx, by := g.GetViewer().WindowToBoard(cursor.Point())
    a.tx = int(bx)
    a.ty = int(by)
  }
  if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
    ex, ey := a.ent.Pos()
    if dist(ex, ey, a.tx, a.ty) <= a.Range && a.ent.HasLos(a.tx, a.ty, 1, 1) {
      var exec aoeExec
      exec.SetBasicData(a.ent, a)
      exec.X, exec.Y = a.tx, a.ty
      return true, &exec
    } else {
      return true, nil
    }
    return true, nil
  }
  return false, nil
}
func (a *AoeAttack) RenderOnFloor() {
  if a.ent == nil {
    return
  }
  ex, ey := a.ent.Pos()
  if dist(ex, ey, a.tx, a.ty) <= a.Range && a.ent.HasLos(a.tx, a.ty, 1, 1) {
    gl.Color4ub(255, 255, 255, 200)
  } else {
    gl.Color4ub(255, 64, 64, 200)
  }
  base.EnableShader("box")
  base.SetUniformF("box", "dx", float32(a.Diameter))
  base.SetUniformF("box", "dy", float32(a.Diameter))
  base.SetUniformI("box", "temp_invalid", 0)
  x := a.tx - (a.Diameter+1)/2
  y := a.ty - (a.Diameter+1)/2
  (&texture.Object{}).Data().Render(float64(x), float64(y), float64(a.Diameter), float64(a.Diameter))
  base.EnableShader("")
}
func (a *AoeAttack) Cancel() {
  a.aoeAttackTempData = aoeAttackTempData{}
}

type AiAoeTarget int

const (
  AiAoeHitAlliesOk AiAoeTarget = iota
  AiAoeHitMinionsOk
  AiAoeHitNoAllies
)

func (a *AoeAttack) AiBestTarget(ent *game.Entity, extra_dist int, spec AiAoeTarget) (x, y int, targets []*game.Entity) {
  ex, ey := ent.Pos()
  max := 0
  best_dist := 10000
  var bx, by int
  var radius int
  if a.Range > 0 {
    radius += a.Range
  }
  if extra_dist > 0 {
    radius += extra_dist
  }
  for x := ex - radius; x <= ex+radius; x++ {
    for y := ey - radius; y <= ey+radius; y++ {
      if !ent.HasLos(x, y, 1, 1) {
        continue
      }
      targets = a.getTargetsAt(ent.Game(), x, y)
      ok := true
      count := 0
      for i := range targets {
        if targets[i].Side() != ent.Side() {
          count++
        } else if ent.Side() == game.SideHaunt && spec == AiAoeHitMinionsOk {
          if targets[i].HauntEnt == nil || targets[i].HauntEnt.Level != game.LevelMinion {
            ok = false
          }
        } else if spec != AiAoeHitAlliesOk {
          ok = false
        }
      }
      dx := x - ex
      if dx < 0 {
        dx = -dx
      }
      dy := y - ey
      if dy < 0 {
        dy = -dy
      }
      dist := dx
      if dy > dx {
        dist = dy
      }
      if ok && (count > max || count == max && dist < best_dist) {
        max = count
        best_dist = dist
        bx, by = x, y
      }
    }
  }
  return bx, by, a.getTargetsAt(ent.Game(), bx, by)
}
func (a *AoeAttack) AiAttackPosition(ent *game.Entity, x, y int) game.ActionExec {
  if !ent.HasLos(x, y, 1, 1) {
    base.Log().Printf("Don't have los")
    return nil
  }
  if a.Ap > ent.Stats.ApCur() {
    base.Log().Printf("Don't have the ap")
    return nil
  }
  var exec aoeExec
  exec.SetBasicData(ent, a)
  exec.X, exec.Y = x, y
  return &exec
}

// Used for doing los computation on aoe attacks, so we don't have to allocate
// and deallocate lots of these.  Only one ai is ever running at a time so
// this should be ok.
var grid [4][][]bool

func init() {
  grid[0] = make([][]bool, house.LosTextureSize)
  grid[1] = make([][]bool, house.LosTextureSize)
  grid[2] = make([][]bool, house.LosTextureSize)
  grid[3] = make([][]bool, house.LosTextureSize)
  raw := make([]bool, 4*house.LosTextureSizeSquared)
  stride := house.LosTextureSizeSquared
  for i := 0; i < house.LosTextureSize; i++ {
    grid[0][i] = raw[stride*0+i*house.LosTextureSize : stride*0+(i+1)*house.LosTextureSize]
    grid[1][i] = raw[stride*1+i*house.LosTextureSize : stride*1+(i+1)*house.LosTextureSize]
    grid[2][i] = raw[stride*2+i*house.LosTextureSize : stride*2+(i+1)*house.LosTextureSize]
    grid[3][i] = raw[stride*3+i*house.LosTextureSize : stride*3+(i+1)*house.LosTextureSize]
  }
}

func (a *AoeAttack) getTargetsAt(g *game.Game, tx, ty int) []*game.Entity {
  x := tx - (a.Diameter+1)/2
  y := ty - (a.Diameter+1)/2
  x2 := tx + a.Diameter/2
  y2 := ty + a.Diameter/2

  // If the diameter is even we need to run los from all four positions
  // around the center of the aoe.
  num_centers := 1
  if a.Diameter%2 == 0 {
    num_centers = 4
  }

  var targets []*game.Entity
  for i := 0; i < num_centers; i++ {
    // If num_centers is 4 then this will calculate the los for all four
    // positions around the center
    g.DetermineLos(tx+i%2, ty+i/2, a.Diameter, grid[i])
  }
  for _, ent := range g.Ents {
    entx, enty := ent.Pos()
    has_los := false
    for i := 0; i < num_centers; i++ {
      has_los = has_los || grid[i][entx][enty]
    }
    if has_los && entx >= x && entx < x2 && enty >= y && enty < y2 {
      targets = append(targets, ent)
    }
  }
  algorithm.Choose2(&targets, func(e *game.Entity) bool {
    return e.Stats != nil
  })

  return targets
}

func (a *AoeAttack) Maintain(dt int64, g *game.Game, ae game.ActionExec) game.MaintenanceStatus {
  if ae != nil {
    a.exec = ae.(*aoeExec)
    a.targets = a.getTargetsAt(g, a.exec.X, a.exec.Y)
    if a.Current_ammo > 0 {
      a.Current_ammo--
    }
    a.ent = g.EntityById(ae.EntityId())
    if !a.ent.HasLos(a.exec.X, a.exec.Y, 1, 1) {
      base.Error().Printf("Entity %d tried to target position (%d, %d) with an aoe but doesn't have los to it: %v", a.ent.Id, a.exec.X, a.exec.Y, a.exec)
      return game.Complete
    }
    if a.Ap > a.ent.Stats.ApCur() {
      base.Error().Printf("Got an aoe attack that required more ap than available: %v", a.exec)
      return game.Complete
    }
    a.ent.Stats.ApplyDamage(-a.Ap, 0, status.Unspecified)

    // Track this information for the ais - the attacking ent will only
    // remember one ent that it hit, but that's ok
    for _, target := range a.targets {
      if target.Side() != a.ent.Side() {
        target.Info.LastEntThatAttackedMe = a.ent.Id
        a.ent.Info.LastEntThatIAttacked = target.Id
      }
    }
  }
  if a.ent.Sprite().State() != "ready" {
    return game.InProgress
  }
  for _, target := range a.targets {
    if target.Stats.HpCur() > 0 && target.Sprite().State() != "ready" {
      return game.InProgress
    }
  }
  a.ent.TurnToFace(a.exec.X, a.exec.Y)
  for _, target := range a.targets {
    target.TurnToFace(a.ent.Pos())
  }
  a.ent.Sprite().Command(a.Animation)
  for _, target := range a.targets {
    if g.DoAttack(a.ent, target, a.Strength, a.Kind) {
      for _, name := range a.Conditions {
        target.Stats.ApplyCondition(status.MakeCondition(name))
      }
      target.Stats.ApplyDamage(0, -a.Damage, a.Kind)
      if target.Stats.HpCur() <= 0 {
        target.Sprite().CommandN([]string{"defend", "killed"})
      } else {
        target.Sprite().CommandN([]string{"defend", "damaged"})
      }
    } else {
      target.Sprite().CommandN([]string{"defend", "undamaged"})
    }
  }
  return game.Complete
}
func (a *AoeAttack) Interrupt() bool {
  return true
}
