package actions

import (
	"encoding/gob"
	"github.com/runningwild/glop/gin"
	"github.com/runningwild/glop/gui"
	"github.com/runningwild/glop/sprite"
	"github.com/runningwild/haunts/base"
	"github.com/runningwild/haunts/game"
	"github.com/runningwild/haunts/game/status"
	"github.com/runningwild/haunts/texture"
	"github.com/runningwild/opengl/gl"
	lua "github.com/xenith-studios/golua"
	"path/filepath"
)

func registerBasicAttacks() map[string]func() game.Action {
	attack_actions := make(map[string]*BasicAttackDef)
	base.RemoveRegistry("actions-attack_actions")
	base.RegisterRegistry("actions-attack_actions", attack_actions)
	base.RegisterAllObjectsInDir("actions-attack_actions", filepath.Join(base.GetDataDir(), "actions", "basic_attacks"), ".json", "json")
	makers := make(map[string]func() game.Action)
	for name := range attack_actions {
		cname := name
		makers[cname] = func() game.Action {
			a := BasicAttack{Defname: cname}
			base.GetObject("actions-attack_actions", &a)
			if !a.Target_allies && !a.Target_enemies {
				base.Error().Printf("Basic Attack '%s' cannot target anything!  Either Target_allies or Target_enemies must be true", a.Name)
			}
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
	game.RegisterActionMakers(registerBasicAttacks)
	gob.Register(&BasicAttack{})
	gob.Register(&basicAttackExec{})
}

// Basic Attacks are single target and instant, they are also readyable
type BasicAttack struct {
	Defname string
	*BasicAttackDef
	basicAttackTempData

	Current_ammo int
}
type BasicAttackDef struct {
	Name           string
	Kind           status.Kind
	Ap             int
	Ammo           int // 0 = infinity
	Strength       int
	Range          int
	Damage         int
	Target_allies  bool
	Target_enemies bool
	Animation      string
	Conditions     []string
	Texture        texture.Object
	Sounds         map[string]string
}
type basicAttackTempData struct {
	ent *game.Entity

	// Potential targets
	targets []*game.Entity

	// The selected target for the attack
	target *game.Entity

	// exec that we're currently executing
	exec *basicAttackExec
}

type basicAttackExec struct {
	id int
	game.BasicActionExec
	Target game.EntityId
}

func (exec basicAttackExec) Push(L *lua.State, g *game.Game) {
	exec.BasicActionExec.Push(L, g)
	if L.IsNil(-1) {
		return
	}
	target := g.EntityById(exec.Target)
	L.PushString("Target")
	game.LuaPushEntity(L, target)
	L.SetTable(-3)
}

func (a *BasicAttack) SoundMap() map[string]string {
	return a.Sounds
}

func (a *BasicAttack) Push(L *lua.State) {
	L.NewTable()
	L.PushString("Type")
	L.PushString("Basic Attack")
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
	L.PushString("Ammo")
	if a.Current_ammo == -1 {
		L.PushInteger(1000)
	} else {
		L.PushInteger(a.Current_ammo)
	}
	L.SetTable(-3)
}

// Results - used by the ai to get feedback on what its actions did.
type BasicAttackResult struct {
	Hit bool
}

var exec_id int

// TODO: This thing leaks memory since we never bother to purge it.  It would
// make the most sense to purge it OnRound(), but we'd have to make a way to
// register OnRound callbacks with the game.
var results map[int]BasicAttackResult

func init() {
	results = make(map[int]BasicAttackResult)
}
func GetBasicAttackResult(e game.ActionExec) *BasicAttackResult {
	res, ok := results[e.(*basicAttackExec).id]
	if !ok {
		return nil
	}
	return &res
}

func dist(x, y, x2, y2 int) int {
	dx := x - x2
	if dx < 0 {
		dx = -dx
	}
	dy := y - y2
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}
func (a *BasicAttack) AP() int {
	return a.Ap
}
func (a *BasicAttack) Pos() (int, int) {
	return 0, 0
}
func (a *BasicAttack) Dims() (int, int) {
	return 0, 0
}
func (a *BasicAttack) String() string {
	return a.Name
}
func (a *BasicAttack) Icon() *texture.Object {
	return &a.Texture
}
func (a *BasicAttack) Readyable() bool {
	return true
}
func (a *BasicAttack) validTarget(source, target *game.Entity) bool {
	if source.Stats == nil || target.Stats == nil {
		return false
	}
	if distBetweenEnts(source, target) > a.Range {
		return false
	}
	x2, y2 := target.Pos()
	dx, dy := target.Dims()
	if !source.HasLos(x2, y2, dx, dy) {
		return false
	}
	if target.Stats.HpCur() <= 0 {
		return false
	}
	if source.Side() == target.Side() && !a.Target_allies {
		return false
	}
	if source.Side() != target.Side() && !a.Target_enemies {
		return false
	}
	return true
}
func (a *BasicAttack) findTargets(ent *game.Entity, g *game.Game) []*game.Entity {
	var targets []*game.Entity
	for _, target := range g.Ents {
		if a.validTarget(ent, target) {
			targets = append(targets, target)
		}
	}
	return targets
}
func (a *BasicAttack) Preppable(ent *game.Entity, g *game.Game) bool {
	return a.Current_ammo != 0 && ent.Stats.ApCur() >= a.Ap && len(a.findTargets(ent, g)) > 0
}
func (a *BasicAttack) Prep(ent *game.Entity, g *game.Game) bool {
	if !a.Preppable(ent, g) {
		return false
	}
	a.ent = ent
	a.targets = a.findTargets(ent, g)
	return true
}
func (a *BasicAttack) AiAttackTarget(ent *game.Entity, target *game.Entity) game.ActionExec {
	if !a.validTarget(ent, target) {
		return nil
	}
	return a.makeExec(ent, target)
}
func (a *BasicAttack) makeExec(ent, target *game.Entity) *basicAttackExec {
	var exec basicAttackExec
	exec.id = exec_id
	exec_id++
	exec.SetBasicData(ent, a)
	exec.Target = target.Id
	return &exec
}
func (a *BasicAttack) HandleInput(group gui.EventGroup, g *game.Game) (bool, game.ActionExec) {
	target := g.HoveredEnt()
	if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
		if target == nil || !a.validTarget(a.ent, target) {
			return true, nil
		}
		return true, a.makeExec(a.ent, target)
	}
	return false, nil
}
func (a *BasicAttack) RenderOnFloor() {
	gl.Disable(gl.TEXTURE_2D)
	gl.Begin(gl.QUADS)
	gl.Color4d(1.0, 0.2, 0.2, 0.8)
	for _, ent := range a.targets {
		ix, iy := ent.Pos()
		x := float64(ix)
		y := float64(iy)
		gl.Vertex2d(x+0, y+0)
		gl.Vertex2d(x+0, y+1)
		gl.Vertex2d(x+1, y+1)
		gl.Vertex2d(x+1, y+0)
	}
	gl.End()
}
func (a *BasicAttack) Cancel() {
	a.basicAttackTempData = basicAttackTempData{}
}
func (a *BasicAttack) Maintain(dt int64, g *game.Game, ae game.ActionExec) game.MaintenanceStatus {
	if ae != nil {
		a.exec = ae.(*basicAttackExec)
		a.ent = g.EntityById(ae.EntityId())
		a.target = a.ent.Game().EntityById(a.exec.Target)

		// Track this information for the ais
		if a.ent.Side() != a.target.Side() {
			a.ent.Info.LastEntThatIAttacked = a.target.Id
			a.target.Info.LastEntThatAttackedMe = a.ent.Id
		}

		if a.Ap > a.ent.Stats.ApCur() {
			base.Error().Printf("Got a basic attack that required more ap than available: %v", a.exec)
			base.Error().Printf("Ent: %s, Ap: %d", a.ent.Name, a.ent.Stats.ApCur())
			return game.Complete
		}

		if !a.validTarget(a.ent, a.target) {
			base.Error().Printf("Got a basic attack that was invalid for some reason: %v", a.exec)
			return game.Complete
		}
	}
	if a.ent.Sprite().State() == "ready" && a.target.Sprite().State() == "ready" {
		a.target.TurnToFace(a.ent.Pos())
		a.ent.TurnToFace(a.target.Pos())
		if a.Current_ammo > 0 {
			a.Current_ammo--
		}
		a.ent.Stats.ApplyDamage(-a.Ap, 0, status.Unspecified)
		var defender_cmds []string
		if g.DoAttack(a.ent, a.target, a.Strength, a.Kind) {
			for _, name := range a.Conditions {
				a.target.Stats.ApplyCondition(status.MakeCondition(name))
			}
			a.target.Stats.ApplyDamage(0, -a.Damage, a.Kind)
			if a.target.Stats.HpCur() <= 0 {
				defender_cmds = []string{"defend", "killed"}
			} else {
				defender_cmds = []string{"defend", "damaged"}
			}
			results[a.exec.id] = BasicAttackResult{Hit: true}
		} else {
			defender_cmds = []string{"defend", "undamaged"}
			results[a.exec.id] = BasicAttackResult{Hit: false}
		}
		sprites := []*sprite.Sprite{a.ent.Sprite(), a.target.Sprite()}
		sprite.CommandSync(sprites, [][]string{[]string{a.Animation}, defender_cmds}, "hit")
		return game.Complete
	}
	return game.InProgress
}
func (a *BasicAttack) Interrupt() bool {
	return true
}
