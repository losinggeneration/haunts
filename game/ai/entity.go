package ai

import (
	"fmt"
	"github.com/runningwild/glop/util/algorithm"
	"../../base"
	"../../game"
	"../../game/actions"
	"../../house"
	lua "github.com/xenith-studios/golua"
	"sort"
)

func (a *Ai) addEntityContext() {
	a.loadUtils("entity")

	game.LuaPushEntity(a.L, a.ent)
	a.L.SetGlobal("Me")

	a.L.NewTable()
	game.LuaPushSmartFunctionTable(a.L, game.FunctionTable{
		"BasicAttack":        func() { a.L.PushGoFunction(DoBasicAttackFunc(a)) },
		"AoeAttack":          func() { a.L.PushGoFunction(DoAoeAttackFunc(a)) },
		"Move":               func() { a.L.PushGoFunction(DoMoveFunc(a)) },
		"DoorToggle":         func() { a.L.PushGoFunction(DoDoorToggleFunc(a)) },
		"InteractWithObject": func() { a.L.PushGoFunction(DoInteractWithObjectFunc(a)) },
	})
	a.L.SetMetaTable(-2)
	a.L.SetGlobal("Do")

	a.L.NewTable()
	game.LuaPushSmartFunctionTable(a.L, game.FunctionTable{
		"AllPathablePoints":          func() { a.L.PushGoFunction(AllPathablePointsFunc(a)) },
		"RangedDistBetweenPositions": func() { a.L.PushGoFunction(RangedDistBetweenPositionsFunc(a)) },
		"RangedDistBetweenEntities":  func() { a.L.PushGoFunction(RangedDistBetweenEntitiesFunc(a)) },
		"NearestNEntities":           func() { a.L.PushGoFunction(NearestNEntitiesFunc(a.ent)) },
		"Waypoints":                  func() { a.L.PushGoFunction(WaypointsFunc(a.ent)) },
		"Exists":                     func() { a.L.PushGoFunction(ExistsFunc(a)) },
		"BestAoeAttackPos":           func() { a.L.PushGoFunction(BestAoeAttackPosFunc(a)) },
		"NearbyUnexploredRooms":      func() { a.L.PushGoFunction(NearbyUnexploredRoomsFunc(a)) },
		"RoomPath":                   func() { a.L.PushGoFunction(RoomPathFunc(a)) },
		"RoomContaining":             func() { a.L.PushGoFunction(RoomContainingFunc(a)) },
		"RoomsAreEqual":              func() { a.L.PushGoFunction(RoomAreEqualFunc(a)) },
		"AllDoorsBetween":            func() { a.L.PushGoFunction(AllDoorsBetween(a)) },
		"AllDoorsOn":                 func() { a.L.PushGoFunction(AllDoorsOn(a)) },
		"DoorPositions":              func() { a.L.PushGoFunction(DoorPositionsFunc(a)) },
		"DoorIsOpen":                 func() { a.L.PushGoFunction(DoorIsOpenFunc(a)) },
		"RoomPositions":              func() { a.L.PushGoFunction(RoomPositionsFunc(a)) },
		"Rand":                       func() { a.L.PushGoFunction(randFunc(a)) },
	})
	a.L.SetMetaTable(-2)
	a.L.SetGlobal("Utils")

	a.L.NewTable()
	game.LuaPushSmartFunctionTable(a.L, game.FunctionTable{
		"GetEntsByName": func() { a.L.PushGoFunction(GetEntsByName(a)) },
	})
	a.L.SetMetaTable(-2)
	a.L.SetGlobal("Cheat")
}

type entityDist struct {
	dist int
	ent  *game.Entity
}
type entityDistSlice []entityDist

func (e entityDistSlice) Len() int {
	return len(e)
}
func (e entityDistSlice) Less(i, j int) bool {
	return e[i].dist < e[j].dist
}
func (e entityDistSlice) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func getActionByName(e *game.Entity, name string) game.Action {
	for _, action := range e.Actions {
		if action.String() == name {
			return action
		}
	}
	return nil
}

// Used for doing los computation in the ai, so we don't have to allocate
// and deallocate lots of these.  Only one ai is ever running at a time so
// this should be ok.
var grid [][]bool

func init() {
	raw := make([]bool, house.LosTextureSizeSquared)
	grid = make([][]bool, house.LosTextureSize)
	for i := range grid {
		grid[i] = raw[i*house.LosTextureSize : (i+1)*house.LosTextureSize]
	}
}

// Returns an array of all points that can be reached by walking from a
// specific location that end in a certain general area.  Assumes that a 1x1
// unit is doing the walking.
//    Format:
//    points = AllPathablePoints(src, dst, min, max)
//
//    Inputs:
//    src - table[x,y] - Where the path starts.
//    dst - table[x,y] - Another point near where the path should go.
//    min - integer    - Minimum distance from dst that the path should end at.
//    max - integer    - Maximum distance from dst that the path should end at.
//
//    Outputs:
//    points - array[table[x,y]]
func AllPathablePointsFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "AllPathablePoints", game.LuaPoint, game.LuaPoint, game.LuaInteger, game.LuaInteger) {
			return 0
		}
		min := L.ToInteger(-2)
		max := L.ToInteger(-1)
		x1, y1 := game.LuaToPoint(L, -4)
		x2, y2 := game.LuaToPoint(L, -3)

		a.ent.Game().DetermineLos(x2, y2, max, grid)
		var dst []int
		for x := x2 - max; x <= x2+max; x++ {
			for y := y2 - max; y <= y2+max; y++ {
				if x > x2-min && x < x2+min && y > y2-min && y < y2+min {
					continue
				}
				if x < 0 || y < 0 || x >= len(grid) || y >= len(grid[0]) {
					continue
				}
				if !grid[x][y] {
					continue
				}
				dst = append(dst, a.ent.Game().ToVertex(x, y))
			}
		}
		vis := 0
		for i := range grid {
			for j := range grid[i] {
				if grid[i][j] {
					vis++
				}
			}
		}
		base.Log().Printf("Visible: %d", vis)
		graph := a.ent.Game().Graph(a.ent.Side(), true, nil)
		src := []int{a.ent.Game().ToVertex(x1, y1)}
		reachable := algorithm.ReachableDestinations(graph, src, dst)
		L.NewTable()
		base.Log().Printf("%d/%d reachable from (%d, %d) -> (%d, %d)", len(reachable), len(dst), x1, y1, x2, y2)
		for i, v := range reachable {
			_, x, y := a.ent.Game().FromVertex(v)
			L.PushInteger(i + 1)
			game.LuaPushPoint(L, x, y)
			L.SetTable(-3)
		}
		return 1
	}
}

// Performs a basic attack against the specifed target.
//    Format:
//    res = DoBasicAttack(attack, target)
//
//    Inputs:
//    attack - string  - Name of the attack to use.
//    target - integer - Entity id of the target of this attack.
//
//    Outputs:
//    res - table - Table containing the following values:
//                  hit (boolean) - true iff the attack hit its target.
//                  If the attack was invalid for some reason res will be nil.
func DoBasicAttackFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "DoBasicAttack", game.LuaString, game.LuaEntity) {
			return 0
		}
		me := a.ent
		name := L.ToString(-2)
		action := getActionByName(me, name)
		if action == nil {
			game.LuaDoError(L, fmt.Sprintf("Entity '%s' (id=%d) has no action named '%s'.", me.Name, me.Id, name))
			return 0
		}
		target := game.LuaToEntity(L, a.ent.Game(), -1)
		if action == nil {
			game.LuaDoError(L, fmt.Sprintf("Tried to target an entity who doesn't exist."))
			return 0
		}
		attack, ok := action.(*actions.BasicAttack)
		if !ok {
			game.LuaDoError(L, fmt.Sprintf("Action '%s' is not a basic attack.", name))
			return 0
		}
		exec := attack.AiAttackTarget(me, target)
		if exec != nil {
			a.execs <- exec
			<-a.pause
			result := actions.GetBasicAttackResult(exec)
			if result == nil {
				L.PushNil()
			} else {
				L.NewTable()
				L.PushString("hit")
				L.PushBoolean(result.Hit)
				L.SetTable(-3)
			}
		} else {
			L.PushNil()
		}
		return 1
	}
}

// Performs an aoe attack against centered at the specified position.
//    Format:
//    res = DoAoeAttack(attack, pos)
//
//    Inputs:
//    attack - string     - Name of the attack to use.
//    pos    - table[x,y] - Position to center the aoe around.
//
//    Outputs:
//    res - boolean - true if the action performed, nil otherwise.
func DoAoeAttackFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "DoAoeAttack", game.LuaString, game.LuaPoint) {
			return 0
		}
		me := a.ent
		name := L.ToString(-2)
		action := getActionByName(me, name)
		if action == nil {
			game.LuaDoError(L, fmt.Sprintf("Entity '%s' (id=%d) has no action named '%s'.", me.Name, me.Id, name))
			return 0
		}
		attack, ok := action.(*actions.AoeAttack)
		if !ok {
			game.LuaDoError(L, fmt.Sprintf("Action '%s' is not an aoe attack.", name))
			return 0
		}
		tx, ty := game.LuaToPoint(L, -1)
		exec := attack.AiAttackPosition(me, tx, ty)
		if exec != nil {
			a.execs <- exec
			<-a.pause
			L.PushBoolean(true)
		} else {
			L.PushNil()
		}
		return 1
	}
}

// Performs an aoe attack against centered at the specified position.
//    Format:
//    target = BestAoeAttackPos(attack, extra_dist, spec)
//
//    Inputs:
//    attack     - string  - Name of the attack to use.
//    extra_dist - integer - Available distance to move before attacking.
//    spec       - string  - One of the following values:
//                           "allies ok", "minions ok", "enemies only"
//
//    Outputs:
//    pos  - table[x,y] - Position to place aoe for maximum results.
//    hits - array[ents] - Visible entities that will be in the aoe
func BestAoeAttackPosFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "BestAoeAttackPos", game.LuaString, game.LuaInteger, game.LuaString) {
			return 0
		}
		me := a.ent
		name := L.ToString(-3)
		action := getActionByName(me, name)
		if action == nil {
			game.LuaDoError(L, fmt.Sprintf("Entity '%s' (id=%d) has no action named '%s'.", me.Name, me.Id, name))
			return 0
		}
		attack, ok := action.(*actions.AoeAttack)
		if !ok {
			game.LuaDoError(L, fmt.Sprintf("Action '%s' is not an aoe attack.", name))
			return 0
		}
		var spec actions.AiAoeTarget
		switch L.ToString(-1) {
		case "allies ok":
			spec = actions.AiAoeHitAlliesOk
		case "minions ok":
			spec = actions.AiAoeHitMinionsOk
		case "enemies only":
			spec = actions.AiAoeHitNoAllies
		default:
			game.LuaDoError(L, fmt.Sprintf("'%s' is not a valid value of spec for BestAoeAttackPos().", L.ToString(-1)))
			return 0
		}
		x, y, hits := attack.AiBestTarget(me, L.ToInteger(-2), spec)
		game.LuaPushPoint(L, x, y)
		L.NewTable()
		for i := range hits {
			L.PushInteger(i + 1)
			game.LuaPushEntity(L, hits[i])
			L.SetTable(-3)
		}
		return 2
	}
}

// Performs a move action to the closest one of any of the specifed inputs
// points.  The movement can be restricted to not spend more than a certain
// amount of ap.
//    Format:
//    success, p = DoMove(dsts, max_ap)
//
//    Input:
//    dsts  - array[table[x,y]] - Array of all points that are acceptable
//                                destinations.
//    max_ap - integer - Maxmium ap to spend while doing this move, if the
//                       required ap exceeds this the entity will still move
//                       as far as possible towards a destination.
//
//    Output:
//    success = bool - True iff the move made it to a position in dsts.
//    p - table[x,y] - New position of this entity, or nil if the move failed.
func DoMoveFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "DoMove", game.LuaArray, game.LuaInteger) {
			return 0
		}
		me := a.ent
		max_ap := L.ToInteger(-1)
		L.Pop(1)
		cur_ap := me.Stats.ApCur()
		if max_ap > cur_ap {
			max_ap = cur_ap
		}
		n := int(L.ObjLen(-1))
		dsts := make([]int, n)[0:0]
		for i := 1; i <= n; i++ {
			L.PushInteger(i)
			L.GetTable(-2)
			x, y := game.LuaToPoint(L, -1)
			dsts = append(dsts, me.Game().ToVertex(x, y))
			L.Pop(1)
		}
		var move *actions.Move
		var ok bool
		for i := range me.Actions {
			move, ok = me.Actions[i].(*actions.Move)
			if ok {
				break
			}
		}
		if !ok {
			// TODO: what to do here?  This poor guy didn't have a move action :(
			L.PushNil()
			L.PushNil()
			return 2
		}
		exec := move.AiMoveToPos(me, dsts, max_ap)
		if exec != nil {
			a.execs <- exec
			<-a.pause
			// TODO: Need to get a resolution
			x, y := me.Pos()
			v := me.Game().ToVertex(x, y)
			complete := false
			for i := range dsts {
				if v == dsts[i] {
					complete = true
					break
				}
			}
			L.PushBoolean(complete)
			game.LuaPushPoint(L, x, y)
			base.Log().Printf("Finished move")
		} else {
			base.Log().Printf("Didn't bother moving")
			L.PushBoolean(true)
			L.PushNil()
		}
		return 2
	}
}

// Computes the ranged distance between two points.
//    Format:
//    dist = RangedDistBetweenPositions(p1, p2)
//
//    Input:
//    p1 - table[x,y]
//    p2 - table[x,y]
//
//    Output:
//    dist - integer - The ranged distance between the two positions.
func RangedDistBetweenPositionsFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "RangedDistBetweenPositions", game.LuaPoint, game.LuaPoint) {
			return 0
		}
		x1, y1 := game.LuaToPoint(L, -2)
		x2, y2 := game.LuaToPoint(L, -1)
		dx := x2 - x1
		if dx < 0 {
			dx = -dx
		}
		dy := y2 - y1
		if dy < 0 {
			dy = -dy
		}
		if dx > dy {
			L.PushInteger(dx)
		} else {
			L.PushInteger(dy)
		}
		return 1
	}
}

// Computes the ranged distance between two entities.
//    Format:
//    dist = RangedDistBetweenEntities(e1, e2)
//
//    Input:
//    e1 - integer - An entity id.
//    e2 - integer - Another entity id.
//
//    Output:
//    dist - integer - The ranged distance between the two specified entities,
//                     this will not necessarily be the same as
//                     RangedDistBetweenPositions(pos(e1), pos(e2)) if at
//                     least one of the entities isn't 1x1.
func RangedDistBetweenEntitiesFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "RangedDistBetweenEntities", game.LuaEntity, game.LuaEntity) {
			return 0
		}
		e1 := game.LuaToEntity(L, a.ent.Game(), -2)
		e2 := game.LuaToEntity(L, a.ent.Game(), -1)
		for _, e := range []*game.Entity{e1, e2} {
			if e == nil {
				L.PushNil()
				return 1
			}
			x, y := e.Pos()
			dx, dy := e.Dims()
			if !a.ent.HasLos(x, y, dx, dy) {
				L.PushNil()
				return 1
			}
		}

		L.PushInteger(rangedDistBetween(e1, e2))
		return 1
	}
}

// Queries whether or not an entity still exists.  An entity existing implies
// that it currently alive.
//    Format:
//    e = exists(id)
//
//    Input:
//    id - integer - Entity id of the entity whose existence we are querying.
//
//    Output:
//    e - boolean - True if the entity exists and has positive hp.
func ExistsFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		// if !game.LuaCheckParamsOk(L, "exists", game.LuaTable) {
		//   return 0
		// }
		if L.IsNil(-1) {
			return 0
		}
		ent := game.LuaToEntity(L, a.ent.Game(), -1)
		L.PushBoolean(ent != nil && ent.Stats != nil && ent.Stats.HpCur() > 0)
		return 1
	}
}

// Returns an array of all entities of a specified type that are in this
// entity's los.  The entities in the array will be sorted in ascending order
// of distance from this entity.
//    Format
//    ents = nearestNEntites(max, kind)
//
//    Input:
//    max  - integer - Maximum number of entities to return
//    kind - string  - One of "intruder" "denizen" "minion" "servitor"
//                     "master" "non-minion" "non-servitor" "non-master" and
//                     "all".  The "non-*" parameters indicate denizens only
//                     (i.e. will *not* include intruders) that are not of the
//                     type specified.
//
//    Output:
//    ents - array[integer] - Array of entity ids.
func NearestNEntitiesFunc(me *game.Entity) lua.GoFunction {
	valid_kinds := map[string]bool{
		"intruder": true,
		"denizen":  true,
		"minion":   true,
		"servitor": true,
		"master":   true,
		"object":   true,
	}
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "NearestNEntities", game.LuaInteger, game.LuaString) {
			return 0
		}
		g := me.Game()
		max := L.ToInteger(-2)
		kind := L.ToString(-1)
		if !valid_kinds[kind] {
			err_str := fmt.Sprintf("NearestNEntities expects kind in the set ['intruder' 'denizen' 'servitor' 'master' 'minion'], got %s.", kind)
			base.Warn().Printf(err_str)
			L.PushString(err_str)
			L.Error()
			return 0
		}
		var eds entityDistSlice
		for _, ent := range g.Ents {
			if ent.Stats != nil && ent.Stats.HpCur() <= 0 {
				continue
			}
			switch kind {
			case "intruder":
				if ent.Side() != game.SideExplorers {
					continue
				}
			case "denizen":
				if ent.Side() != game.SideHaunt {
					continue
				}
			case "minion":
				if ent.HauntEnt == nil || ent.HauntEnt.Level != game.LevelMinion {
					continue
				}
			case "servitor":
				if ent.HauntEnt == nil || ent.HauntEnt.Level != game.LevelServitor {
					continue
				}
			case "master":
				if ent.HauntEnt == nil || ent.HauntEnt.Level != game.LevelMaster {
					continue
				}
			case "object":
				if ent.ObjectEnt == nil {
					continue
				}
			}
			x, y := ent.Pos()
			dx, dy := ent.Dims()
			if !me.HasTeamLos(x, y, dx, dy) {
				continue
			}
			eds = append(eds, entityDist{rangedDistBetween(me, ent), ent})
		}
		// TODO: ONLY GUYS THAT EXIST
		sort.Sort(eds)
		if max > len(eds) {
			max = len(eds)
		}
		if max < 0 {
			max = 0
		}
		eds = eds[0:max]

		// eds contains the results, in order.  Now we make a lua table and
		// populate it with the entity ids of the results.
		L.NewTable()
		for i := range eds {
			L.PushInteger(i + 1)
			game.LuaPushEntity(L, eds[i].ent)
			L.SetTable(-3)
		}
		return 1
	}
}

func WaypointsFunc(me *game.Entity) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "Waypoints") {
			return 0
		}
		g := me.Game()
		L.NewTable()
		count := 0
		for _, wp := range g.Waypoints {
			if wp.Side != me.Side() {
				continue
			}
			count++
			L.PushInteger(count)
			L.NewTable()
			L.PushString("Name")
			L.PushString(wp.Name)
			L.SetTable(-3)
			L.PushString("Radius")
			L.PushNumber(wp.Radius)
			L.SetTable(-3)
			L.PushString("Pos")
			game.LuaPushPoint(L, int(wp.X), int(wp.Y))
			L.SetTable(-3)
			L.SetTable(-3)
		}
		return 1
	}
}

func checkFloorRoom(h *house.HouseDef, floor, room int) bool {
	if floor < 0 || room < 0 {
		return false
	}
	if floor >= len(h.Floors) {
		return false
	}
	if room >= len(h.Floors[floor].Rooms) {
		return false
	}
	return true
}

func checkFloorRoomDoor(h *house.HouseDef, floor, room, door int) bool {
	if !checkFloorRoom(h, floor, room) {
		return false
	}
	if door < 0 || door >= len(h.Floors[floor].Rooms[room].Doors) {
		return false
	}
	return true
}

func NearbyUnexploredRoomsFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "NearbyUnexploredRooms") {
			return 0
		}

		me := a.ent
		g := me.Game()
		graph := g.RoomGraph()
		var unexplored []int
		for room_num, _ := range g.House.Floors[0].Rooms {
			if !me.Info.RoomsExplored[room_num] {
				adj, _ := graph.Adjacent(room_num)
				for i := range adj {
					if me.Info.RoomsExplored[adj[i]] || adj[i] == me.CurrentRoom() {
						unexplored = append(unexplored, room_num)
						break
					}
				}
			}
		}
		L.NewTable()
		for i := range unexplored {
			L.PushInteger(i + 1)
			game.LuaPushRoom(L, a.game, a.game.House.Floors[0].Rooms[unexplored[i]])
			L.SetTable(-3)
		}
		return 1
	}
}

// Returns a list of rooms representing a path from src to dst.  The path will
// not include src, but will include dst.  This function will return nil if
// the path requires going through more than a single unexplored room, this
// means that you can use this to path to an unexplored room, but you cannot
// use it to path to a room further in the house than that.
// rooms.
//    Format
//    path = roomPath(src, dst)
//
//    Input:
//    src - Room to start the path from.
//    dst - Room to end the path at.
//
//    Output:
//    path - array - A list of rooms that connect src to dst, excluding src
//    but including dst.
func RoomPathFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "roomPath", game.LuaRoom, game.LuaRoom) {
			return 0
		}

		me := a.ent
		g := me.Game()
		graph := g.RoomGraph()
		r1 := game.LuaToRoom(L, g, -2)
		r2 := game.LuaToRoom(L, g, -1)
		if r1 == nil || r2 == nil {
			game.LuaDoError(L, fmt.Sprintf("Referenced one or more invalid rooms."))
			return 0
		}

		L.PushString("room")
		L.GetTable(-3)
		r1_index := L.ToInteger(-1)
		L.Pop(1)

		L.PushString("room")
		L.GetTable(-2)
		r2_index := L.ToInteger(-1)
		L.Pop(1)

		cost, path := algorithm.Dijkstra(graph, []int{r1_index}, []int{r2_index})
		if cost == -1 {
			L.PushNil()
			return 1
		}
		num_unexplored := 0
		for _, v := range path {
			if !me.Info.RoomsExplored[v] {
				num_unexplored++
			}
		}
		if num_unexplored > 1 {
			L.PushNil()
			return 1
		}
		L.NewTable()
		for i, v := range path {
			if i == 0 {
				continue
			} // Skip this one because we're in it already
			L.PushInteger(i)
			game.LuaPushRoom(L, g, g.House.Floors[0].Rooms[v])
			L.SetTable(-3)
		}
		return 1
	}
}

// Returns the room that the specified entity is currently in.  The specified
// entity must be in los of a unit on the acting entity's team, or be on the
// acting entity's team, otherwise this function returns nil.
//    Format
//    r = roomContaining(id)
//
//    Input:
//    id - An entity id.
//
//    Output:
//    r - room - The room the specified entity is in, or nil if it can't be
//    seen right now.
func RoomContainingFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "roomContaining", game.LuaEntity) {
			return 0
		}
		ent := game.LuaToEntity(L, a.ent.Game(), -1)
		side := a.ent.Side()
		x, y := a.ent.Pos()
		dx, dy := a.ent.Dims()
		if ent == nil || (ent.Side() != side && !a.ent.Game().TeamLos(side, x, y, dx, dy)) {
			L.PushNil()
		} else {
			game.LuaPushRoom(L, ent.Game(), ent.Game().House.Floors[0].Rooms[ent.CurrentRoom()])
		}
		return 1
	}
}

func RoomAreEqualFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "RoomsAreEqual", game.LuaRoom, game.LuaRoom) {
			return 0
		}
		r1 := game.LuaToRoom(L, a.ent.Game(), -2)
		r2 := game.LuaToRoom(L, a.ent.Game(), -1)
		L.PushBoolean(r1 == r2)
		return 1
	}
}

// Returns a list of all doors between two rooms.
//    Format
//    doors = allDoorsBetween(r1, r2)
//
//    Input:
//    r1 - room - A room.
//    r2 - room - Another room.
//
//    Output:
//    doors - array[door] - List of all doors connecting r1 and r2.
func AllDoorsBetween(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "allDoorsBetween", game.LuaRoom, game.LuaRoom) {
			return 0
		}
		room1 := game.LuaToRoom(L, a.ent.Game(), -2)
		room2 := game.LuaToRoom(L, a.ent.Game(), -1)
		if room1 == nil || room2 == nil {
			game.LuaDoError(L, "AllDoorsBetween: Specified an invalid door.")
			return 0
		}

		// TODO: Check for floors!
		// if f1 != f2 {
		//   // Rooms on different floors can theoretically be connected in the
		//   // future by a stairway, but right now that doesn't happen.
		//   L.NewTable()
		//   return 1
		// }

		L.NewTable()
		count := 1
		for _, door1 := range room1.Doors {
			for _, door2 := range room2.Doors {
				_, d := a.ent.Game().House.Floors[0].FindMatchingDoor(room1, door1)
				if d == door2 {
					L.PushInteger(count)
					count++
					game.LuaPushDoor(L, a.ent.Game(), door1)
					L.SetTable(-3)
				}
			}
		}
		return 1
	}
}

// Returns a list of all doors attached to the specified room.
//    Format
//    room = allDoorsOn(r)
//
//    Input:
//    r - room - A room.
//
//    Output:
//    doors - array[door] - List of all doors attached to the specified room.
func AllDoorsOn(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "allDoorsOn", game.LuaRoom) {
			return 0
		}
		room := game.LuaToRoom(L, a.ent.Game(), -1)
		if room == nil {
			game.LuaDoError(L, "Specified an invalid room.")
			return 0
		}

		L.NewTable()
		for i := range room.Doors {
			L.PushInteger(i + 1)
			game.LuaPushDoor(L, a.ent.Game(), room.Doors[i])
			L.SetTable(-3)
		}
		return 1
	}
}

// Returns a list of all positions that the specified door can be opened and
// closed from.
//    Format
//    ps = doorPositions(d)
//
//    Input:
//    d - door - A door.
//
//    Output:
//    ps - array[table[x,y]] - List of all position this door can be opened
//    and closed from.
func DoorPositionsFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "DoorPositions", game.LuaDoor) {
			return 0
		}
		room := game.LuaToRoom(L, a.ent.Game(), -1)
		door := game.LuaToDoor(L, a.ent.Game(), -1)
		if door == nil || room == nil {
			game.LuaDoError(L, "DoorPositions: Specified an invalid door.")
			return 0
		}

		var x, y, dx, dy int
		switch door.Facing {
		case house.FarLeft:
			x = door.Pos
			y = room.Size.Dy - 1
			dx = 1
		case house.FarRight:
			x = room.Size.Dx - 1
			y = door.Pos
			dy = 1
		case house.NearLeft:
			x = -1
			y = door.Pos
			dy = 1
		case house.NearRight:
			x = door.Pos
			y = -1
			dx = 1
		default:
			game.LuaDoError(L, fmt.Sprintf("Found a door with a bad facing."))
		}
		L.NewTable()
		count := 1
		for i := 0; i < door.Width; i++ {
			L.PushInteger(count*2 - 1)
			game.LuaPushPoint(L, room.X+x+dx*i, room.Y+y+dy*i)
			L.SetTable(-3)
			L.PushInteger(count * 2)
			game.LuaPushPoint(L, room.X+x+dx*i+dy, room.Y+y+dy*i+dx)
			L.SetTable(-3)
			count++
		}
		return 1
	}
}

// Queries whether a door is currently open.
//    Format
//    open = doorIsOpen(d)
//
//    Input:
//    d - door - A door.
//
//    Output:
//    open - boolean - True if the door is open, false otherwise.
func DoorIsOpenFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "doorIsOpen", game.LuaDoor) {
			return 0
		}
		door := game.LuaToDoor(L, a.ent.Game(), -1)
		if door == nil {
			game.LuaDoError(L, "DoorIsOpen: Specified an invalid door.")
			return 0
		}
		L.PushBoolean(door.IsOpened())
		return 1
	}
}

// Performs an Interact action to toggle the opened/closed state of a door.
//    Format
//    res = doDoorToggle(d)
//
//    Input:
//    d - door - A door.
//
//    Output:
//    res - boolean - True if the door was opened, false if it was closed.
//    res will be nil if the action could not be performed for some reason.
func DoDoorToggleFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "doDoorToggle", game.LuaDoor) {
			return 0
		}
		door := game.LuaToDoor(L, a.ent.Game(), -1)
		if door == nil {
			game.LuaDoError(L, "DoDoorToggle: Specified an invalid door.")
			return 0
		}

		var interact *actions.Interact
		for _, action := range a.ent.Actions {
			var ok bool
			interact, ok = action.(*actions.Interact)
			if ok {
				break
			}
		}
		if interact == nil {
			game.LuaDoError(L, fmt.Sprintf("Tried to toggle a door, but don't have an interact action."))
			L.PushNil()
			return 1
		}
		exec := interact.AiToggleDoor(a.ent, door)
		if exec != nil {
			a.execs <- exec
			<-a.pause
			L.PushBoolean(door.IsOpened())
		} else {
			L.PushNil()
		}
		return 1
	}
}

func DoInteractWithObjectFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "DoInteractWithObject", game.LuaEntity) {
			return 0
		}
		object := game.LuaToEntity(L, a.ent.Game(), -1)
		var interact *actions.Interact
		for _, action := range a.ent.Actions {
			var ok bool
			interact, ok = action.(*actions.Interact)
			if ok {
				break
			}
		}
		if interact == nil {
			game.LuaDoError(L, "Tried to interact with an object, but don't have an interact action.")
			L.PushNil()
			return 1
		}
		exec := interact.AiInteractWithObject(a.ent, object)
		if exec != nil {
			a.execs <- exec
			<-a.pause
			L.PushBoolean(true)
		} else {
			L.PushNil()
		}
		return 1
	}
}

// Returns a list of all positions inside the specified room.
//    Format
//    ps = roomPositions(r)
//
//    Input:
//    r - room - A room.
//
//    Output:
//    ps - array[table[x,y]] - List of all position inside the specified room.
func RoomPositionsFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "roomPositions", game.LuaRoom) {
			return 0
		}
		room := game.LuaToRoom(L, a.ent.Game(), -1)
		if room == nil {
			game.LuaDoError(L, "RoomPositions: Specified an invalid room.")
			return 0
		}

		L.NewTable()
		count := 1
		for x := room.X; x < room.X+room.Size.Dx; x++ {
			for y := room.Y; y < room.Y+room.Size.Dy; y++ {
				L.PushInteger(count)
				count++
				game.LuaPushPoint(L, x, y)
				L.SetTable(-3)
			}
		}
		return 1
	}
}

func randFunc(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "Rand", game.LuaInteger) {
			return 0
		}
		n := L.ToInteger(-1)
		L.PushInteger(int(a.game.Rand.Int63()%int64(n)) + 1)
		return 1
	}
}

func GetEntsByName(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "GetEntsByName", game.LuaString) {
			return 0
		}
		name := L.ToString(-1)
		count := 1
		L.NewTable()
		for _, ent := range a.game.Ents {
			if ent.Name == name {
				L.PushInteger(count)
				count++
				game.LuaPushEntity(L, ent)
				L.SetTable(-3)
			}
		}
		return 1
	}
}
