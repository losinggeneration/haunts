package actions

import (
	"encoding/gob"
	"path/filepath"

	"github.com/MobRulesGames/glop/gin"
	"github.com/MobRulesGames/glop/gui"
	lua "github.com/MobRulesGames/golua/lua"
	"github.com/MobRulesGames/haunts/base"
	"github.com/MobRulesGames/haunts/game"
	"github.com/MobRulesGames/haunts/game/status"
	"github.com/MobRulesGames/haunts/texture"
	"github.com/MobRulesGames/opengl/gl"
)

func registerSummonActions() map[string]func() game.Action {
	summons_actions := make(map[string]*SummonActionDef)
	base.RemoveRegistry("actions-summons_actions")
	base.RegisterRegistry("actions-summons_actions", summons_actions)
	base.RegisterAllObjectsInDir("actions-summons_actions", filepath.Join(base.GetDataDir(), "actions", "summons"), ".json", "json")
	makers := make(map[string]func() game.Action)
	for name := range summons_actions {
		cname := name
		makers[cname] = func() game.Action {
			a := SummonAction{Defname: cname}
			base.GetObject("actions-summons_actions", &a)
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
	game.RegisterActionMakers(registerSummonActions)
	gob.Register(&SummonAction{})
	gob.Register(&summonExec{})
}

// Summon Actions target a single cell, are instant, and unreadyable.
type SummonAction struct {
	Defname string
	*SummonActionDef
	summonActionTempData

	Current_ammo int
}
type SummonActionDef struct {
	Name         string
	Kind         status.Kind
	Personal_los bool
	Ap           int
	Ammo         int // 0 = infinity
	Range        int
	Ent_name     string
	Animation    string
	Conditions   []string
	Texture      texture.Object
	Sounds       map[string]string
}
type summonActionTempData struct {
	ent    *game.Entity
	cx, cy int
	spawn  *game.Entity
}
type summonExec struct {
	game.BasicActionExec
	Pos int
}

func (exec summonExec) Push(L *lua.State, g *game.Game) {
	exec.BasicActionExec.Push(L, g)
	if L.IsNil(-1) {
		return
	}
	_, x, y := g.FromVertex(exec.Pos)
	L.PushString("Pos")
	game.LuaPushPoint(L, x, y)
	L.SetTable(-3)
}

func (a *SummonAction) SoundMap() map[string]string {
	return a.Sounds
}

func (a *SummonAction) Push(L *lua.State) {
	L.NewTable()
	L.PushString("Type")
	L.PushString("Summon")
	L.SetTable(-3)
	L.PushString("Name")
	L.PushString(a.Name)
	L.SetTable(-3)
	L.PushString("Ap")
	L.PushInteger(int64(a.Ap))
	L.SetTable(-3)
	L.PushString("Entity")
	L.PushString(a.Ent_name)
	L.SetTable(-3)
	L.PushString("Los")
	L.PushBoolean(a.Personal_los)
	L.SetTable(-3)
	L.PushString("Range")
	L.PushInteger(int64(a.Range))
	L.SetTable(-3)
	L.PushString("Ammo")
	if a.Current_ammo == -1 {
		L.PushInteger(1000)
	} else {
		L.PushInteger(int64(a.Current_ammo))
	}
	L.SetTable(-3)

}

func (a *SummonAction) AP() int {
	return a.Ap
}
func (a *SummonAction) Pos() (int, int) {
	return a.cx, a.cy
}
func (a *SummonAction) Dims() (int, int) {
	return 1, 1
}
func (a *SummonAction) String() string {
	return a.Name
}
func (a *SummonAction) Icon() *texture.Object {
	return &a.Texture
}
func (a *SummonAction) Readyable() bool {
	return false
}
func (a *SummonAction) Preppable(ent *game.Entity, g *game.Game) bool {
	return a.Current_ammo != 0 && ent.Stats.ApCur() >= a.Ap
}
func (a *SummonAction) Prep(ent *game.Entity, g *game.Game) bool {
	if !a.Preppable(ent, g) {
		return false
	}
	a.ent = ent
	return true
}
func (a *SummonAction) HandleInput(group gui.EventGroup, g *game.Game) (bool, game.ActionExec) {
	cursor := group.Events[0].Key.Cursor()
	if cursor != nil {
		bx, by := g.GetViewer().WindowToBoard(cursor.Point())
		bx += 0.5
		by += 0.5
		if bx < 0 {
			bx--
		}
		if by < 0 {
			by--
		}
		a.cx = int(bx)
		a.cy = int(by)
	}

	if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
		if g.IsCellOccupied(a.cx, a.cy) {
			return true, nil
		}
		if a.Personal_los && !a.ent.HasLos(a.cx, a.cy, 1, 1) {
			return true, nil
		}
		if a.ent.Stats.ApCur() >= a.Ap {
			var exec summonExec
			exec.SetBasicData(a.ent, a)
			exec.Pos = a.ent.Game().ToVertex(a.cx, a.cy)
			return true, &exec
		}
		return true, nil
	}
	return false, nil
}
func (a *SummonAction) RenderOnFloor() {
	if a.ent == nil {
		return
	}
	ex, ey := a.ent.Pos()
	if dist(ex, ey, a.cx, a.cy) <= a.Range && a.ent.HasLos(a.cx, a.cy, 1, 1) {
		gl.Color4ub(255, 255, 255, 200)
	} else {
		gl.Color4ub(255, 64, 64, 200)
	}
	base.EnableShader("box")
	base.SetUniformF("box", "dx", 1)
	base.SetUniformF("box", "dy", 1)
	base.SetUniformI("box", "temp_invalid", 0)
	(&texture.Object{}).Data().Render(float64(a.cx), float64(a.cy), 1, 1)
	base.EnableShader("")
}
func (a *SummonAction) Cancel() {
	a.summonActionTempData = summonActionTempData{}
}
func (a *SummonAction) Maintain(dt int64, g *game.Game, ae game.ActionExec) game.MaintenanceStatus {
	if ae != nil {
		exec := ae.(*summonExec)
		ent := g.EntityById(exec.Ent)
		if ent == nil {
			base.Error().Printf("Got a summon action without a valid entity.")
			return game.Complete
		}
		a.ent = ent
		_, a.cx, a.cy = a.ent.Game().FromVertex(exec.Pos)
		a.ent.Stats.ApplyDamage(-a.Ap, 0, status.Unspecified)
		a.spawn = game.MakeEntity(a.Ent_name, a.ent.Game())
		if a.Current_ammo > 0 {
			a.Current_ammo--
		}
	}
	if a.ent.Sprite().State() == "ready" {
		a.ent.TurnToFace(a.cx, a.cy)
		a.ent.Sprite().Command(a.Animation)
		a.spawn.Stats.OnBegin()
		a.ent.Game().SpawnEntity(a.spawn, a.cx, a.cy)
		return game.Complete
	}
	return game.InProgress
}
func (a *SummonAction) Interrupt() bool {
	return true
}
