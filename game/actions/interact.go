package actions

import (
	"encoding/gob"
	"github.com/MobRulesGames/glop/gin"
	"github.com/MobRulesGames/glop/gui"
	"github.com/MobRulesGames/haunts/base"
	"github.com/MobRulesGames/haunts/game"
	"github.com/MobRulesGames/haunts/game/status"
	"github.com/MobRulesGames/haunts/house"
	"github.com/MobRulesGames/haunts/texture"
	lua "github.com/MobRulesGames/golua"
	"path/filepath"
)

func registerInteracts() map[string]func() game.Action {
	interact_actions := make(map[string]*InteractDef)
	base.RemoveRegistry("actions-interact_actions")
	base.RegisterRegistry("actions-interact_actions", interact_actions)
	base.RegisterAllObjectsInDir("actions-interact_actions", filepath.Join(base.GetDataDir(), "actions", "interacts"), ".json", "json")
	makers := make(map[string]func() game.Action)
	for name := range interact_actions {
		cname := name
		makers[cname] = func() game.Action {
			a := Interact{Defname: cname}
			base.GetObject("actions-interact_actions", &a)
			return &a
		}
	}
	return makers
}

func init() {
	game.RegisterActionMakers(registerInteracts)
	gob.Register(&Interact{})
	gob.Register(&interactExec{})
}

type Interact struct {
	Defname string
	*InteractDef
	interactInst
}
type InteractDef struct {
	Name         string // "Relic", "Mystery", or "Cleanse"
	Display_name string // The string actually displayed to the user
	Ap           int
	Range        int
	Animation    string
	Texture      texture.Object
}
type interactInst struct {
	ent *game.Entity

	// Potential targets
	targets []*game.Entity

	// Potential doors
	doors []*house.Door

	// The selected target for the attack
	target *game.Entity
}
type interactExec struct {
	id int
	game.BasicActionExec
	Target game.EntityId

	// If this interaction was to open or close a door then Toggle_door will be
	// true, otherwise it will be false.  If it is true then Target will be 0.
	Toggle_door       bool
	Floor, Room, Door int
}

func (exec interactExec) Push(L *lua.State, g *game.Game) {
	exec.BasicActionExec.Push(L, g)
	if L.IsNil(-1) {
		return
	}
	L.PushString("Toggle Door")
	L.PushBoolean(exec.Toggle_door)
	L.SetTable(-3)
	if exec.Toggle_door {
		L.PushString("Door")
		game.LuaPushDoor(L, g, exec.getDoor(g))
	} else {
		L.PushString("Target")
		game.LuaPushEntity(L, g.EntityById(exec.Target))
	}
	L.SetTable(-3)
}
func (exec interactExec) getDoor(g *game.Game) *house.Door {
	if exec.Floor < 0 || exec.Floor >= len(g.House.Floors) {
		return nil
	}
	floor := g.House.Floors[exec.Floor]
	if exec.Room < 0 || exec.Room >= len(floor.Rooms) {
		return nil
	}
	room := floor.Rooms[exec.Room]
	if exec.Door < 0 || exec.Door >= len(room.Doors) {
		return nil
	}
	return room.Doors[exec.Door]
}

func (a *Interact) SoundMap() map[string]string {
	return nil
}

func (a *Interact) makeDoorExec(ent *game.Entity, floor, room, door int) *interactExec {
	var exec interactExec
	exec.id = exec_id
	exec_id++
	exec.SetBasicData(ent, a)
	exec.Floor = floor
	exec.Room = room
	exec.Door = door
	exec.Toggle_door = true
	return &exec
}

func (a *Interact) Push(L *lua.State) {
	L.NewTable()
	L.PushString("Type")
	L.PushString("Interact")
	L.SetTable(-3)
	L.PushString("Ap")
	L.PushInteger(a.Ap)
	L.SetTable(-3)
	L.PushString("Range")
	L.PushInteger(a.Range)
	L.SetTable(-3)
}

func (a *Interact) AP() int {
	return a.Ap
}
func (a *Interact) Pos() (int, int) {
	return 0, 0
}
func (a *Interact) Dims() (int, int) {
	return 0, 0
}
func (a *Interact) String() string {
	return a.Display_name
}
func (a *Interact) Icon() *texture.Object {
	return &a.Texture
}
func (a *Interact) Readyable() bool {
	return false
}
func distBetweenEnts(e1, e2 *game.Entity) int {
	x1, y1 := e1.Pos()
	dx1, dy1 := e1.Dims()
	x2, y2 := e2.Pos()
	dx2, dy2 := e2.Dims()

	var xdist int
	switch {
	case x1 >= x2+dx2:
		xdist = x1 - (x2 + dx2)
	case x2 >= x1+dx1:
		xdist = x2 - (x1 + dx1)
	default:
		xdist = 0
	}

	var ydist int
	switch {
	case y1 >= y2+dy2:
		ydist = y1 - (y2 + dy2)
	case y2 >= y1+dy1:
		ydist = y2 - (y1 + dy1)
	default:
		ydist = 0
	}

	if xdist > ydist {
		return xdist
	}
	return ydist
}

type frect struct {
	x, y, x2, y2 float64
}

func makeIntFrect(x, y, x2, y2 int) frect {
	return frect{float64(x), float64(y), float64(x2), float64(y2)}
}
func (f frect) overlapX(f2 frect) bool {
	if f2.x >= f.x && f2.x <= f.x2 {
		return true
	}
	if f2.x2 >= f.x && f2.x2 <= f.x2 {
		return true
	}
	return false
}
func (f frect) overlapY(f2 frect) bool {
	if f2.y >= f.y && f2.y <= f.y2 {
		return true
	}
	if f2.y2 >= f.y && f2.y2 <= f.y2 {
		return true
	}
	return false
}
func (f frect) Overlaps(f2 frect) bool {
	return (f.overlapX(f2) || f2.overlapX(f)) && (f.overlapY(f2) || f2.overlapY(f))
}
func (f frect) Contains(x, y float64) bool {
	return f.Overlaps(frect{x, y, x, y})
}

func makeRectForDoor(room *house.Room, door *house.Door) frect {
	var dr frect
	switch door.Facing {
	case house.FarLeft:
		dr = makeIntFrect(door.Pos, room.Size.Dy-1, door.Pos+door.Width, room.Size.Dy)
		dr.y += 0.5
		dr.y2 += 0.5
	case house.FarRight:
		dr = makeIntFrect(room.Size.Dx-1, door.Pos, room.Size.Dx, door.Pos+door.Width)
		dr.x += 0.5
		dr.x2 += 0.5
	case house.NearLeft:
		dr = makeIntFrect(0, door.Pos, 1, door.Pos+door.Width)
		dr.x -= 0.5
		dr.x2 -= 0.5
	case house.NearRight:
		dr = makeIntFrect(door.Pos, 0, door.Pos+door.Width, 1)
		dr.y -= 0.5
		dr.y2 -= 0.5
	}
	dr.x += float64(room.X)
	dr.y += float64(room.Y)
	dr.x2 += float64(room.X)
	dr.y2 += float64(room.Y)
	return dr
}

func (a *Interact) AiInteractWithObject(ent, object *game.Entity) game.ActionExec {
	if ent.Stats.ApCur() < a.Ap {
		return nil
	}
	if distBetweenEnts(ent, object) > a.Range {
		return nil
	}
	x, y := object.Pos()
	dx, dy := object.Dims()
	if !ent.HasLos(x, y, dx, dy) {
		return nil
	}
	var exec interactExec
	exec.SetBasicData(ent, a)
	exec.Target = object.Id
	return &exec
}

func (a *Interact) AiToggleDoor(ent *game.Entity, door *house.Door) game.ActionExec {
	if door.AlwaysOpen() {
		return nil
	}
	for fi, f := range ent.Game().House.Floors {
		for ri, r := range f.Rooms {
			for di, d := range r.Doors {
				if d == door {
					return a.makeDoorExec(ent, fi, ri, di)
				}
			}
		}
	}
	return nil
}

func (a *Interact) findDoors(ent *game.Entity, g *game.Game) []*house.Door {
	room_num := ent.CurrentRoom()
	room := g.House.Floors[0].Rooms[room_num]
	x, y := ent.Pos()
	dx, dy := ent.Dims()
	ent_rect := makeIntFrect(x, y, x+dx, y+dy)
	var valid []*house.Door
	for _, door := range room.Doors {
		if door.AlwaysOpen() {
			continue
		}
		if ent_rect.Overlaps(makeRectForDoor(room, door)) {
			valid = append(valid, door)
		}
	}
	return valid
}
func (a *Interact) findTargets(ent *game.Entity, g *game.Game) []*game.Entity {
	var targets []*game.Entity
	for _, e := range g.Ents {
		x, y := e.Pos()
		dx, dy := e.Dims()
		if e == ent {
			continue
		}
		if e.ObjectEnt == nil {
			continue
		}
		if e.Sprite().State() != "ready" {
			continue
		}
		if distBetweenEnts(e, ent) > a.Range {
			continue
		}
		if !ent.HasLos(x, y, dx, dy) {
			continue
		}

		// Make sure it's still active:
		active := false
		active = true
		if !active {
			continue
		}

		targets = append(targets, e)
	}
	return targets
}
func (a *Interact) Preppable(ent *game.Entity, g *game.Game) bool {
	if a.Ap > ent.Stats.ApCur() {
		return false
	}
	a.targets = a.findTargets(ent, g)
	a.doors = a.findDoors(ent, g)
	return len(a.targets) > 0 || len(a.doors) > 0
}
func (a *Interact) Prep(ent *game.Entity, g *game.Game) bool {
	if a.Preppable(ent, g) {
		a.ent = ent
		room := g.House.Floors[0].Rooms[ent.CurrentRoom()]
		for _, door := range a.doors {
			_, other_door := g.House.Floors[0].FindMatchingDoor(room, door)
			if other_door != nil {
				door.HighlightThreshold(true)
				other_door.HighlightThreshold(true)
			}
		}
		return true
	}
	return false
}
func (a *Interact) HandleInput(group gui.EventGroup, g *game.Game) (bool, game.ActionExec) {
	if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
		bx, by := g.GetViewer().WindowToBoard(gin.In().GetCursor("Mouse").Point())
		room_num := a.ent.CurrentRoom()
		room := g.House.Floors[0].Rooms[room_num]
		for door_num, door := range room.Doors {
			rect := makeRectForDoor(room, door)
			if rect.Contains(float64(bx), float64(by)) {
				var exec interactExec
				exec.Toggle_door = true
				exec.SetBasicData(a.ent, a)
				exec.Room = room_num
				exec.Door = door_num
				return true, &exec
			}
		}
	}
	target := g.HoveredEnt()
	if target == nil {
		return false, nil
	}
	if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
		for i := range a.targets {
			if a.targets[i] == target && distBetweenEnts(a.ent, target) <= a.Range {
				var exec interactExec
				exec.SetBasicData(a.ent, a)
				exec.Target = target.Id
				return true, &exec
			}
		}
		return true, nil
	}
	return false, nil
}
func (a *Interact) RenderOnFloor() {
}
func (a *Interact) Cancel() {
	room := a.ent.Game().House.Floors[0].Rooms[a.ent.CurrentRoom()]
	for _, door := range a.doors {
		_, other_door := a.ent.Game().House.Floors[0].FindMatchingDoor(room, door)
		if other_door != nil {
			door.HighlightThreshold(false)
			other_door.HighlightThreshold(false)
		}
	}
	for _, door := range a.doors {
		door.HighlightThreshold(false)
	}
	a.interactInst = interactInst{}
}
func (a *Interact) Maintain(dt int64, g *game.Game, ae game.ActionExec) game.MaintenanceStatus {
	if ae != nil {
		exec := ae.(*interactExec)
		a.ent = g.EntityById(ae.EntityId())
		if (exec.Target != 0) == (exec.Toggle_door) {
			base.Error().Printf("Got an interact that tried to target a door and an entity: %v", exec)
			return game.Complete
		}
		if exec.Target != 0 {
			target := g.EntityById(exec.Target)
			if target == nil {
				base.Error().Printf("Tried to interact with an entity that doesn't exist: %v", exec)
				return game.Complete
			}
			if target.ObjectEnt == nil {
				base.Error().Printf("Tried to interact with an entity that wasn't an object: %v", exec)
				return game.Complete
			}
			if target.Sprite().State() != "ready" {
				base.Error().Printf("Tried to interact with an object that wasn't in its ready state: %v", exec)
				return game.Complete
			}
			if distBetweenEnts(a.ent, target) > a.Range {
				base.Error().Printf("Tried to interact with an object that was out of range: %v", exec)
				return game.Complete
			}
			x, y := target.Pos()
			dx, dy := target.Dims()
			if !a.ent.HasLos(x, y, dx, dy) {
				base.Error().Printf("Tried to interact with an object without having los: %v", exec)
				return game.Complete
			}
			a.ent.Stats.ApplyDamage(-a.Ap, 0, status.Unspecified)
			target.Sprite().Command("inspect")
			return game.Complete
		} else {
			// We're interacting with a door here
			if exec.Floor < 0 || exec.Floor >= len(g.House.Floors) {
				base.Error().Printf("Specified an unknown floor %v", exec)
				return game.Complete
			}
			floor := g.House.Floors[exec.Floor]
			if exec.Room < 0 || exec.Room >= len(floor.Rooms) {
				base.Error().Printf("Specified an unknown room %v", exec)
				return game.Complete
			}
			room := floor.Rooms[exec.Room]
			if exec.Door < 0 || exec.Door >= len(room.Doors) {
				base.Error().Printf("Specified an unknown door %v", exec)
				return game.Complete
			}
			door := room.Doors[exec.Door]

			x, y := a.ent.Pos()
			dx, dy := a.ent.Dims()
			ent_rect := makeIntFrect(x, y, x+dx, y+dy)
			if !ent_rect.Overlaps(makeRectForDoor(room, door)) {
				base.Error().Printf("Tried to open a door that was out of range: %v", exec)
				return game.Complete
			}

			_, other_door := floor.FindMatchingDoor(room, door)
			if other_door != nil {
				door.SetOpened(!door.IsOpened())
				other_door.SetOpened(door.IsOpened())
				// if door.IsOpened() {
				//   sound.PlaySound(door.Open_sound)
				// } else {
				//   sound.PlaySound(door.Shut_sound)
				// }
				g.RecalcLos()
				a.ent.Stats.ApplyDamage(-a.Ap, 0, status.Unspecified)
			} else {
				base.Error().Printf("Couldn't find matching door: %v", exec)
				return game.Complete
			}
		}
	}
	return game.Complete
}
func (a *Interact) Interrupt() bool {
	return true
}
