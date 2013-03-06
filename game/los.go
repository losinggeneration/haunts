package game

import (
	"bytes"
	"encoding/gob"
	"errors"
	gl "github.com/MobRulesGames/gogl/gl21"
	"github.com/MobRulesGames/cmwc"
	"github.com/MobRulesGames/glop/sprite"
	"github.com/MobRulesGames/glop/util/algorithm"
	"github.com/MobRulesGames/haunts/base"
	"github.com/MobRulesGames/haunts/house"
	"github.com/MobRulesGames/haunts/mrgnet"
	"reflect"
	"regexp"
	"time"
)

type Purpose int

const (
	PurposeNone Purpose = iota
	PurposeRelic
	PurposeMystery
	PurposeCleanse
)

type LosMode int

const (
	LosModeNone LosMode = iota
	LosModeBlind
	LosModeAll
	LosModeEntities
	LosModeRooms
)

type turnState int

const (
	// Waiting for the script to finish Init()
	turnStateInit turnState = iota

	// Waiting for the script to finish RoundStart()
	turnStateStart

	// Waiting for or running an Ai action
	turnStateAiAction

	// Waiting for the script to finish OnAction()
	turnStateScriptOnAction

	// Humans and Ai are done, now the script can run some actions if it wants
	turnStateMainPhaseOver

	// Waiting for the script to finish OnEnd()
	turnStateEnd
)

type sideLosData struct {
	mode LosMode
	tex  *house.LosTexture
}

type waypoint struct {
	Name   string
	Side   Side
	X, Y   float64
	Radius float64
	active bool
	drawn  bool
	// Color, maybe?
}

func (wp *waypoint) Dims() (int, int) {
	return int(2 * wp.Radius), int(2 * wp.Radius)
}
func (wp *waypoint) Pos() (int, int) {
	return int(wp.X), int(wp.Y)
}
func (wp *waypoint) RenderOnFloor() {
	if !wp.active {
		return
	}
	wp.drawn = true
	gl.Color4ub(200, 0, 0, 128)
	base.EnableShader("waypoint")
	base.SetUniformF("waypoint", "radius", float32(wp.Radius))

	t := float32(time.Now().UnixNano()%1e15) / 1.0e9
	base.SetUniformF("waypoint", "time", t)
	gl.Begin(gl.QUADS)
	gl.TexCoord2i(0, 1)
	gl.Vertex2i(gl.Int(wp.X-wp.Radius), gl.Int(wp.Y-wp.Radius))
	gl.TexCoord2i(0, 0)
	gl.Vertex2i(gl.Int(wp.X-wp.Radius), gl.Int(wp.Y+wp.Radius))
	gl.TexCoord2i(1, 0)
	gl.Vertex2i(gl.Int(wp.X+wp.Radius), gl.Int(wp.Y+wp.Radius))
	gl.TexCoord2i(1, 1)
	gl.Vertex2i(gl.Int(wp.X+wp.Radius), gl.Int(wp.Y-wp.Radius))
	gl.End()

	base.EnableShader("")

	// base.EnableShader("")
}

type gameDataTransient struct {
	los struct {
		denizens, intruders sideLosData

		// When merging the los from different entities we'll do it here, and we
		// keep it around to avoid reallocating it every time we need it.
		full_merger []bool
		merger      [][]bool
	}

	// Used to sync up with the script, the value passed is usually nil, but
	// whenever an action happens it will get passed along this channel too.
	comm struct {
		script_to_game chan interface{}
		game_to_script chan interface{}
	}

	script *gameScript

	// Indicates if we're waiting for a script to run or something
	Turn_state   turnState
	Action_state actionState

	net struct {
		key  mrgnet.GameKey
		game *mrgnet.Game
		side Side
	}
}

func (gdt *gameDataTransient) alloc() {
	if gdt.los.denizens.tex != nil {
		return
	}
	gdt.los.denizens.tex = house.MakeLosTexture()
	gdt.los.intruders.tex = house.MakeLosTexture()
	gdt.los.full_merger = make([]bool, house.LosTextureSizeSquared)
	gdt.los.merger = make([][]bool, house.LosTextureSize)
	for i := range gdt.los.merger {
		gdt.los.merger[i] = gdt.los.full_merger[i*house.LosTextureSize : (i+1)*house.LosTextureSize]
	}

	gdt.comm.script_to_game = make(chan interface{}, 1)
	gdt.comm.game_to_script = make(chan interface{}, 1)

	gdt.script = &gameScript{}
	base.Log().Printf("script = %p", gdt.script)
}

type gameDataPrivate struct {
	// Hacky - but gives us a way to prevent selecting ents and whatnot while
	// any kind of modal dialog box is up.
	modal bool
}
type spawnLos struct {
	Pattern string
	r       *regexp.Regexp
}
type gameDataGobbable struct {
	// TODO: No idea if this thing can be loaded from the registry - should
	// probably figure that out at some point
	House *house.HouseDef
	Ents  []*Entity

	// True for online games
	Net bool

	// Set of all Entities that are still resident.  This is so we can safely
	// clean things up since they will all have ais running in the background
	// preventing them from getting GCed.
	all_ents_in_game   map[*Entity]bool
	all_ents_in_memory map[*Entity]bool

	// Regexps.  Any spawn points with names matching this pattern will grant
	// los to the appropriate side.
	Los_spawns struct {
		Denizens, Intruders spawnLos
	}

	// Next unique EntityId to be assigned
	Entity_id EntityId

	// Current player
	Side Side

	// Current turn number - incremented on each OnRound() so every two
	// indicates that a complete round has happened.
	Turn int

	// PRNG, need it here so that we serialize it along with everything
	// else so that replays work properly.
	Rand *cmwc.Cmwc

	// Waypoints, used for signaling things to the player on the map
	Waypoints []waypoint

	// Transient data - none of the following are exported

	player_inactive bool

	viewer *house.HouseViewer

	// If the user is dragging around a new Entity to place, this is it
	new_ent *Entity

	selected_ent *Entity
	hovered_ent  *Entity

	// Stores the current acting entity - if it is an Ai controlled entity
	ai_ent *Entity

	Ai struct {
		Path struct {
			Minions, Denizens, Intruders string
		}
		minions, denizens, intruders Ai
	}

	// If an Ai is executing currently it is referenced here
	active_ai Ai

	current_exec   ActionExec
	current_action Action
}

type Game struct {
	gameDataTransient
	gameDataPrivate
	gameDataGobbable
}

func (g *Game) GobDecode(data []byte) error {
	g.gameDataPrivate = gameDataPrivate{}
	for ent := range g.all_ents_in_memory {
		ent.Release()
	}
	if g.Ai.intruders != nil {
		g.Ai.intruders.Terminate()
	}
	if g.Ai.minions != nil {
		g.Ai.minions.Terminate()
	}
	if g.Ai.denizens != nil {
		g.Ai.denizens.Terminate()
	}

	g.gameDataGobbable = gameDataGobbable{}

	dec := gob.NewDecoder(bytes.NewBuffer(data))
	if err := dec.Decode(&g.gameDataGobbable); err != nil {
		return err
	}

	base.ProcessObject(reflect.ValueOf(g.House), "")
	g.House.Normalize()
	g.viewer = house.MakeHouseViewer(g.House, 62)
	g.viewer.Edit_mode = true
	for _, ent := range g.Ents {
		base.GetObject("entities", ent)
	}

	g.setup()
	for _, ent := range g.Ents {
		ent.Load(g)
		base.Log().Printf("Ungob, ent: %p, %s", ent, ent.Name)
		g.UpdateEntLos(ent, true)
	}
	g.mergeLos(SideHaunt)
	g.mergeLos(SideExplorers)

	var sss []sprite.SpriteState
	if err := dec.Decode(&sss); err != nil {
		return err
	}
	if len(sss) != len(g.Ents) {
		return errors.New("SpriteStates were not recorded properly.")
	}
	for i := range sss {
		g.Ents[i].Sprite().SetSpriteState(sss[i])
	}

	// If Ais were bound then their paths will be listed here and we have to
	// reload them
	if g.Ai.Path.Denizens != "" {
		ai_maker(g.Ai.Path.Denizens, g, nil, &g.Ai.denizens, DenizensAi)
	}
	if g.Ai.denizens == nil {
		g.Ai.denizens = inactiveAi{}
	}
	if g.Ai.Path.Intruders != "" {
		ai_maker(g.Ai.Path.Intruders, g, nil, &g.Ai.intruders, IntrudersAi)
	}
	if g.Ai.intruders == nil {
		g.Ai.intruders = inactiveAi{}
	}
	if g.Ai.Path.Minions != "" {
		ai_maker(g.Ai.Path.Minions, g, nil, &g.Ai.minions, MinionsAi)
	}
	if g.Ai.minions == nil {
		g.Ai.minions = inactiveAi{}
	}

	return nil
}

func (g *Game) GobEncode() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(buf)
	if err := enc.Encode(g.gameDataGobbable); err != nil {
		return nil, err
	}
	var sss []sprite.SpriteState
	for i := range g.Ents {
		sss = append(sss, g.Ents[i].Sprite().GetSpriteState())
	}
	if err := enc.Encode(sss); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (g *Game) EntityById(id EntityId) *Entity {
	for i := range g.Ents {
		if g.Ents[i].Id == id {
			return g.Ents[i]
		}
	}
	return nil
}

func (g *Game) HoveredEnt() *Entity {
	return g.hovered_ent
}

func (g *Game) SelectEnt(ent *Entity) bool {
	if g.Action_state != noAction {
		return false
	}
	found := false
	for i := range g.Ents {
		if g.Ents[i] == ent {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	if g.selected_ent != nil {
		g.selected_ent.selected = false
		g.selected_ent.hovered = false
	}
	g.selected_ent = ent
	if g.selected_ent != nil {
		g.selected_ent.selected = true
	}
	g.viewer.Focus(ent.FPos())
	return true
}

func (g *Game) OnBegin() {
	for i := range g.Ents {
		if g.Ents[i].Stats != nil {
			g.Ents[i].Stats.OnBegin()
		}
	}
}

// TODO: DEPRECATED
func (g *Game) PlaceInitialExplorers(ents []*Entity) {
}

func (g *Game) checkWinConditions() {
	return
	// Check for explorer win conditions
	explorer_win := false

	if explorer_win {
		base.Log().Printf("Explorers won - kaboom")
	}

	// Check for haunt win condition - all intruders dead
	haunts_win := true
	for i := range g.Ents {
		if g.Ents[i].Side() == SideExplorers {
			haunts_win = false
		}
	}
	if haunts_win {
		base.Log().Printf("Haunts won - kaboom")
	}
}

func (g *Game) SetVisibility(side Side) {
	switch side {
	case SideHaunt:
		g.viewer.Los_tex = g.los.denizens.tex
	case SideExplorers:
		g.viewer.Los_tex = g.los.intruders.tex
	default:
		base.Error().Printf("Unable to SetVisibility for side == %d.", side)
		return
	}
}

// This is called if the player is ready to end the turn, if the turn ends
// then the following things happen:
// 1. The game script gets to run its OnRound() function
// 2. Entities with stats and HpCur() <= 0 are removed.
// 3. Entities all have their OnRound() function called.
func (g *Game) OnRound(do_scripts bool) {
	// Don't end the round if any of the following are true
	// An action is currently executing
	if g.Action_state != noAction {
		base.Log().Printf("No OnRound - have action")
		return
	}
	// Any master ai is still active
	if g.Side == SideHaunt && (g.Ai.minions.Active() || g.Ai.denizens.Active()) {
		base.Log().Printf("No OnRound - waiting on ais")
		return
	}

	if do_scripts {
		g.Turn++
		if g.Side == SideExplorers {
			base.Log().Printf("OnRound from %d Intruders to %d Denizens", g.Turn-1, g.Turn)
			g.Side = SideHaunt
		} else {
			base.Log().Printf("OnRound from %d Denizens to %d Intruders", g.Turn-1, g.Turn)
			g.Side = SideExplorers
		}
		g.viewer.Los_tex.Remap()
	}

	for i := range g.Ents {
		if g.Ents[i].Side() == g.Side {
			g.Ents[i].OnRound()
		}
	}

	// The entity ais must be activated before the master ais, otherwise the
	// masters might be running with stale data if one of the entities has been
	// reloaded.
	if do_scripts {
		for i := range g.Ents {
			g.Ents[i].Ai.Activate()
			base.Log().Printf("EntityActive '%s': %t", g.Ents[i].Name, g.Ents[i].Ai.Active())
		}

		if g.Side == SideHaunt {
			g.Ai.minions.Activate()
			g.Ai.denizens.Activate()
			g.player_inactive = g.Ai.denizens.Active()
		} else {
			g.Ai.intruders.Activate()
			g.player_inactive = g.Ai.intruders.Active()
		}
	}

	for i := range g.Ents {
		if g.Ents[i].Stats != nil && g.Ents[i].Stats.HpCur() <= 0 {
			g.viewer.RemoveDrawable(g.Ents[i])
		}
	}
	algorithm.Choose(&g.Ents, func(ent *Entity) bool {
		return ent.Stats == nil || ent.Stats.HpCur() > 0
	})

	if do_scripts {
		g.script.OnRound(g)
	}

	if g.selected_ent != nil {
		g.selected_ent.hovered = false
		g.selected_ent.selected = false
	}
	if g.hovered_ent != nil {
		g.hovered_ent.hovered = false
		g.hovered_ent.selected = false
	}
	g.hovered_ent = nil
}

type actionState int

const (
	noAction actionState = iota

	// The Ai is running and determining the next action to run
	waitingAction

	// The player has selected an action and is determining whether or not to
	// use it, and how.
	preppingAction

	// Check the scripts to see if the action should be modified or cancelled.
	verifyingAction

	// An action is currently running, everything should pause while this runs.
	doingAction
)

func (g *Game) GetViewer() *house.HouseViewer {
	return g.viewer
}

func (g *Game) numVertex() int {
	total := 0
	for _, room := range g.House.Floors[0].Rooms {
		total += room.Size.Dx * room.Size.Dy
	}
	return total
}
func (g *Game) FromVertex(v int) (room *house.Room, x, y int) {
	for _, room := range g.House.Floors[0].Rooms {
		size := room.Size.Dx * room.Size.Dy
		if v >= size {
			v -= size
			continue
		}
		return room, room.X + (v % room.Size.Dx), room.Y + (v / room.Size.Dx)
	}
	return nil, 0, 0
}
func (g *Game) ToVertex(x, y int) int {
	v := 0
	for _, room := range g.House.Floors[0].Rooms {
		if x >= room.X && y >= room.Y && x < room.X+room.Size.Dx && y < room.Y+room.Size.Dy {
			x -= room.X
			y -= room.Y
			v += x + y*room.Size.Dx
			break
		}
		v += room.Size.Dx * room.Size.Dy
	}
	return v
}

// x and y are given in room coordinates
func furnitureAt(room *house.Room, x, y int) *house.Furniture {
	for _, f := range room.Furniture {
		fx, fy := f.Pos()
		fdx, fdy := f.Dims()
		if x >= fx && x < fx+fdx && y >= fy && y < fy+fdy {
			return f
		}
	}
	return nil
}

// x and y are given in floor coordinates
func roomAt(floor *house.Floor, x, y int) *house.Room {
	for _, room := range floor.Rooms {
		rx, ry := room.Pos()
		rdx, rdy := room.Dims()
		if x >= rx && x < rx+rdx && y >= ry && y < ry+rdy {
			return room
		}
	}
	return nil
}

func connected(r, r2 *house.Room, x, y, x2, y2 int) bool {
	if r == r2 {
		return true
	}
	x -= r.X
	y -= r.Y
	x2 -= r2.X
	y2 -= r2.Y
	var facing house.WallFacing
	if x == 0 && x2 != 0 {
		facing = house.NearLeft
	} else if y == 0 && y2 != 0 {
		facing = house.NearRight
	} else if x != 0 && x2 == 0 {
		facing = house.FarRight
	} else if y != 0 && y2 == 0 {
		facing = house.FarLeft
	} else {
		// This shouldn't happen, but in case it does we certainly shouldn't treat
		// it as an open door
		return false
	}
	for _, door := range r.Doors {
		if door.Facing != facing {
			continue
		}
		var pos int
		switch facing {
		case house.NearLeft:
			fallthrough
		case house.FarRight:
			pos = y

		case house.NearRight:
			fallthrough
		case house.FarLeft:
			pos = x
		}
		if pos >= door.Pos && pos < door.Pos+door.Width {
			return door.IsOpened()
		}
	}
	return false
}

func (g *Game) IsCellOccupied(x, y int) bool {
	r := roomAt(g.House.Floors[0], x, y)
	if r == nil {
		return true
	}
	f := furnitureAt(r, x-r.X, y-r.Y)
	if f != nil {
		return true
	}
	for _, ent := range g.Ents {
		ex, ey := ent.Pos()
		if x == ex && y == ey {
			return true
		}
	}
	return false
}

type exclusionGraph struct {
	side Side
	los  bool
	ex   map[*Entity]bool
	g    *Game
}

func (eg *exclusionGraph) Adjacent(v int) ([]int, []float64) {
	return eg.g.adjacent(v, eg.los, eg.side, eg.ex)
}
func (eg *exclusionGraph) NumVertex() int {
	return eg.g.numVertex()
}

func (g *Game) RecalcLos() {
	for i := range g.Ents {
		if g.Ents[i].los != nil {
			g.Ents[i].los.x = -1
		}
	}
}

type roomGraph struct {
	g *Game
}

func (g *Game) RoomGraph() algorithm.Graph {
	return &roomGraph{g}
}

func (rg *roomGraph) NumVertex() int {
	return len(rg.g.House.Floors[0].Rooms)
}

func (rg *roomGraph) Adjacent(n int) ([]int, []float64) {
	room := rg.g.House.Floors[0].Rooms[n]
	var adj []int
	var cost []float64
	for _, door := range room.Doors {
		other_room, _ := rg.g.House.Floors[0].FindMatchingDoor(room, door)
		if other_room != nil {
			for i := range rg.g.House.Floors[0].Rooms {
				if other_room == rg.g.House.Floors[0].Rooms[i] {
					adj = append(adj, i)
					cost = append(cost, 1)
					break
				}
			}
		}
	}
	return adj, cost
}

func (g *Game) Graph(side Side, los bool, exclude []*Entity) algorithm.Graph {
	ex := make(map[*Entity]bool, len(exclude))
	for i := range exclude {
		ex[exclude[i]] = true
	}
	return &exclusionGraph{side, los, ex, g}
}

func (g *Game) adjacent(v int, los bool, side Side, ex map[*Entity]bool) ([]int, []float64) {
	room, x, y := g.FromVertex(v)
	var adj []int
	var weight []float64
	var moves [3][3]float64
	ent_occupied := make(map[[2]int]bool)
	for _, ent := range g.Ents {
		if ex[ent] {
			continue
		}
		x, y := ent.Pos()
		dx, dy := ent.Dims()
		for i := x; i < x+dx; i++ {
			for j := y; j < y+dy; j++ {
				ent_occupied[[2]int{i, j}] = true
			}
		}
	}
	var data *sideLosData
	if los {
		switch side {
		case SideHaunt:
			data = &g.los.denizens
		case SideExplorers:
			data = &g.los.intruders
		default:
			base.Error().Printf("Unable to SetLosMode for side == %d.", side)
			return nil, nil
		}
	}
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			// Only run this loop if exactly one of dx and dy is non-zero
			if (dx == 0) == (dy == 0) {
				continue
			}
			tx := x + dx
			ty := y + dy
			if ent_occupied[[2]int{tx, ty}] {
				continue
			}
			if data != nil && data.tex.Pix()[tx][ty] < house.LosVisibilityThreshold {
				continue
			}
			// TODO: This is obviously inefficient
			troom, _, _ := g.FromVertex(g.ToVertex(tx, ty))
			if troom == nil {
				continue
			}
			if furnitureAt(troom, tx-troom.X, ty-troom.Y) != nil {
				continue
			}
			if !connected(room, troom, x, y, tx, ty) {
				continue
			}
			adj = append(adj, g.ToVertex(tx, ty))
			moves[dx+1][dy+1] = 1
			weight = append(weight, 1)
		}
	}
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			// Only run this loop if both dx and dy are non-zero (moving diagonal)
			if (dx == 0) != (dy == 0) {
				continue
			}
			tx := x + dx
			ty := y + dy
			if ent_occupied[[2]int{tx, ty}] {
				continue
			}
			if data != nil && data.tex.Pix()[tx][ty] < house.LosVisibilityThreshold {
				continue
			}
			// TODO: This is obviously inefficient
			troom, _, _ := g.FromVertex(g.ToVertex(tx, ty))
			if troom == nil {
				continue
			}
			if furnitureAt(troom, tx-troom.X, ty-troom.Y) != nil {
				continue
			}
			if !connected(room, troom, x, y, tx, ty) {
				continue
			}
			if !connected(troom, room, tx, ty, x, y) {
				continue
			}
			if moves[dx+1][1] == 0 || moves[1][dy+1] == 0 {
				continue
			}
			adj = append(adj, g.ToVertex(tx, ty))
			w := (moves[dx+1][1] + moves[1][dy+1]) / 2
			moves[dx+1][dy+1] = w
			weight = append(weight, w)
		}
	}
	return adj, weight
}

func (g *Game) setup() {
	g.gameDataTransient.alloc()
	g.all_ents_in_game = make(map[*Entity]bool)
	g.all_ents_in_memory = make(map[*Entity]bool)
	if g.Side == SideHaunt {
		g.viewer.Los_tex = g.los.intruders.tex
	} else {
		g.viewer.Los_tex = g.los.denizens.tex
	}

	g.Ai.minions = inactiveAi{}
	g.Ai.denizens = inactiveAi{}
	g.Ai.intruders = inactiveAi{}
}

func makeGame(h *house.HouseDef) *Game {
	var g Game
	g.Side = SideExplorers
	g.House = h
	g.House.Normalize()
	g.viewer = house.MakeHouseViewer(g.House, 62)
	g.Rand = cmwc.MakeCmwc(4285415527, 3)
	g.Rand.SeedWithDevRand()

	// This way an unset id will be invalid
	g.Entity_id = 1

	g.Turn = 1
	g.Side = SideHaunt

	g.setup()

	return &g
}

func (g *Game) SetLosMode(side Side, mode LosMode, rooms []*house.Room) {
	var data *sideLosData
	switch side {
	case SideHaunt:
		data = &g.los.denizens
	case SideExplorers:
		data = &g.los.intruders
	default:
		base.Error().Printf("Unable to SetLosMode for side == %d.", side)
		return
	}
	data.mode = mode
	pix := data.tex.Pix()

	switch data.mode {
	case LosModeNone:
		for i := range pix {
			for j := range pix[i] {
				if pix[i][j] >= house.LosVisibilityThreshold {
					pix[i][j] = house.LosVisibilityThreshold - 1
				}
			}
		}

	case LosModeBlind:
		for i := range pix {
			for j := range pix[i] {
				if pix[i][j] >= house.LosVisibilityThreshold {
					pix[i][j] = 0
				}
			}
		}

	case LosModeAll:
		for i := range pix {
			for j := range pix[i] {
				if pix[i][j] < house.LosVisibilityThreshold {
					pix[i][j] = house.LosVisibilityThreshold
				}
			}
		}

	case LosModeEntities:
		// Don't need to do anything here - it's handled on every think

	case LosModeRooms:
		in_room := make(map[int]bool)
		for _, room := range rooms {
			for x := room.X; x < room.X+room.Size.Dx; x++ {
				for y := room.Y; y < room.Y+room.Size.Dy; y++ {
					in_room[g.ToVertex(x, y)] = true
				}
			}
		}
		for i := range pix {
			for j := range pix[i] {
				if in_room[g.ToVertex(i, j)] {
					if pix[i][j] < house.LosVisibilityThreshold {
						pix[i][j] = house.LosVisibilityThreshold
					}
				} else {
					if pix[i][j] >= house.LosVisibilityThreshold {
						pix[i][j] = house.LosVisibilityThreshold - 1
					}
				}

			}
		}
	}
	data.tex.Remap()
}

func (g *Game) Think(dt int64) {
	for _, ent := range g.Ents {
		if !g.all_ents_in_game[ent] {
			g.all_ents_in_game[ent] = true
			g.all_ents_in_memory[ent] = true
		}
	}
	var mark []*Entity
	for ent := range g.all_ents_in_memory {
		if !g.all_ents_in_game[ent] && ent != g.new_ent {
			mark = append(mark, ent)
		}
	}
	for _, ent := range mark {
		delete(g.all_ents_in_game, ent)
		delete(g.all_ents_in_memory, ent)
		ent.Release()
	}

	// Figure out if there are any entities that might be occluded be any
	// furniture, if so we'll want to make that furniture a little transparent.
	for _, floor := range g.House.Floors {
		for _, room := range floor.Rooms {
			for _, furn := range room.Furniture {
				if !furn.Blocks_los {
					continue
				}
				rx, ry := room.Pos()
				x, y2 := furn.Pos()
				x += rx
				y2 += ry
				dx, dy := furn.Dims()
				x2 := x + dx
				y := y2 + dy
				tex := furn.Orientations[furn.Rotation].Texture.Data()
				tex_dy := 2 * (tex.Dy() * ((x2 - y2) - (x - y))) / tex.Dx()
				v1 := y - x
				v2 := y2 - x2
				hit := false
				for _, ent := range g.Ents {
					ex, ey2 := ent.Pos()
					edx, edy := ent.Dims()
					ex2 := ex + edx
					ey := ey2 + edy
					if ex+ey2 < x+y2 || ex+ey2 > x+y2+tex_dy {
						continue
					}
					if ent.Side() != g.Side && !g.TeamLos(g.Side, ex, ey2, edx, edy) {
						continue
					}

					ev1 := ey - ex
					ev2 := ey2 - ex2
					if ev2 >= v1 || ev1 <= v2 {
						continue
					}
					hit = true
					break
				}
				alpha := furn.Alpha()
				if hit {
					furn.SetAlpha(doApproach(alpha, 0.3, dt))
				} else {
					furn.SetAlpha(doApproach(alpha, 1.0, dt))
				}
			}
		}
	}

	switch g.Turn_state {
	case turnStateInit:
		select {
		case <-g.comm.script_to_game:
			base.Log().Printf("ScriptComm: change to turnStateStart")
			g.Turn_state = turnStateStart
			g.script.OnRound(g)
			// g.OnRound()
		default:
		}
	case turnStateStart:
		select {
		case <-g.comm.script_to_game:
			base.Log().Printf("ScriptComm: change to turnStateAiAction")
			g.Turn_state = turnStateAiAction
		default:
		}
	case turnStateScriptOnAction:
		select {
		case exec := <-g.comm.script_to_game:
			if g.current_exec != nil {
				base.Error().Printf("Got an exec from the script when we already had one pending.")
			} else {
				if exec != nil {
					g.current_exec = exec.(ActionExec)
				} else {
					base.Log().Printf("ScriptComm: change to turnStateAiAction")
					g.Turn_state = turnStateAiAction
				}
			}
		default:
		}
	case turnStateMainPhaseOver:
		select {
		case exec := <-g.comm.script_to_game:
			if exec != nil {
				base.Log().Printf("ScriptComm: Got an exec: %v", exec)
				g.current_exec = exec.(ActionExec)
				g.Action_state = doingAction
				ent := g.EntityById(g.current_exec.EntityId())
				g.viewer.RemoveFloorDrawable(g.current_action)
				g.current_action = ent.Actions[g.current_exec.ActionIndex()]
				g.viewer.AddFloorDrawable(g.current_action)
				ent.current_action = g.current_action
			} else {
				g.Turn_state = turnStateEnd
				base.Log().Printf("ScriptComm: change to turnStateEnd for realzes")
			}
		default:
		}

	case turnStateEnd:
		select {
		case <-g.comm.script_to_game:
			g.Turn_state = turnStateStart
			base.Log().Printf("ScriptComm: change to turnStateStart")
			g.OnRound(true)
		default:
		}
	}

	if g.current_exec != nil && g.Action_state != verifyingAction && g.Turn_state != turnStateMainPhaseOver {
		ent := g.EntityById(g.current_exec.EntityId())
		g.viewer.RemoveFloorDrawable(g.current_action)
		g.current_action = ent.Actions[g.current_exec.ActionIndex()]
		g.viewer.AddFloorDrawable(g.current_action)
		ent.current_action = g.current_action
		g.Action_state = verifyingAction
		base.Log().Printf("ScriptComm: request exec verification")
		g.comm.game_to_script <- g.current_exec
	}

	if g.Action_state == verifyingAction {
		select {
		case <-g.comm.script_to_game:
			g.Action_state = doingAction
		default:
		}
	}

	// If there is an action that is currently executing we need to advance that
	// action.
	if g.Action_state == doingAction {
		res := g.current_action.Maintain(dt, g, g.current_exec)
		if g.current_exec != nil {
			base.Log().Printf("ScriptComm: sent action")
			g.current_exec = nil
		}
		switch res {
		case Complete:
			g.current_action.Cancel()
			g.viewer.RemoveFloorDrawable(g.current_action)
			g.current_action = nil
			g.Action_state = noAction
			if g.Turn_state != turnStateMainPhaseOver {
				g.Turn_state = turnStateScriptOnAction
			}
			base.Log().Printf("ScriptComm: Action complete")
			g.comm.game_to_script <- nil
			g.checkWinConditions()

		case InProgress:
		case CheckForInterrupts:
		}
	}

	for _, ent := range g.Ents {
		ent.Think(dt)
		s := ent.Sprite()
		if s != nil {
			if s.AnimState() == "ready" && s.Idle() && g.current_action == nil && ent.current_action != nil {
				g.viewer.RemoveFloorDrawable(g.current_action)
				ent.current_action = nil
			}
		}
	}
	if g.new_ent != nil {
		g.new_ent.Think(dt)
	}
	for i := range g.Ents {
		g.UpdateEntLos(g.Ents[i], false)
	}
	if g.los.denizens.mode == LosModeEntities {
		g.mergeLos(SideHaunt)
	}
	if g.los.intruders.mode == LosModeEntities {
		g.mergeLos(SideExplorers)
	}

	// Do spawn points los stuff
	for _, los := range []*spawnLos{&g.Los_spawns.Denizens, &g.Los_spawns.Intruders} {
		if los.r == nil || los.r.String() != los.Pattern {
			if los.Pattern == "" {
				los.r = nil
			} else {
				var err error
				los.r, err = regexp.Compile("^" + los.Pattern + "$")
				if err != nil {
					base.Warn().Printf("Unable to compile regexp: `%s`", los.Pattern)
					los.Pattern = ""
				}
			}
		}
	}
	for i := 0; i < 2; i++ {
		var los *spawnLos
		var pix [][]byte
		if i == 0 {
			if g.los.denizens.mode == LosModeBlind {
				continue
			}
			los = &g.Los_spawns.Denizens
			pix = g.los.denizens.tex.Pix()
		} else {
			if g.los.intruders.mode == LosModeBlind {
				continue
			}
			los = &g.Los_spawns.Intruders
			pix = g.los.intruders.tex.Pix()
		}
		if los.r == nil {
			continue
		}
		for _, spawn := range g.House.Floors[0].Spawns {
			if !los.r.MatchString(spawn.Name) {
				continue
			}
			sx, sy := spawn.Pos()
			dx, dy := spawn.Dims()
			for x := sx; x < sx+dx; x++ {
				for y := sy; y < sy+dy; y++ {
					if pix[x][y] < house.LosVisibilityThreshold {
						pix[x][y] = house.LosVisibilityThreshold
					}
				}
			}
		}
	}

	for _, tex := range []*house.LosTexture{g.los.denizens.tex, g.los.intruders.tex} {
		pix := tex.Pix()
		amt := dt/6 + 1
		mod := false
		for i := range pix {
			for j := range pix[i] {
				v := int64(pix[i][j])
				if v < house.LosVisibilityThreshold {
					v -= amt
				} else {
					v += amt
				}
				if v < house.LosMinVisibility {
					v = house.LosMinVisibility
				}
				if v < 0 {
					v = 0
				}
				if v > 255 {
					v = 255
				}
				mod = mod || (byte(v) != pix[i][j])
				pix[i][j] = byte(v)
			}
		}
		if mod {
			tex.Remap()
		}
	}

	// Don't do any ai stuff if there is a pending action
	if g.current_action != nil {
		return
	}

	// Also don't do an ai stuff if this isn't the appropriate state
	if g.Turn_state != turnStateAiAction {
		return
	}

	// If any entities are not either ready or dead let's wait until they are
	// before we do any of the ai stuff
	for _, ent := range g.Ents {
		if ent.Side() != SideHaunt && ent.Side() != SideExplorers {
			// Relics and cleanse points and whatnot matter here, and they might not
			// be in a 'ready' state.
			continue
		}
		state := ent.sprite.Sprite().AnimState()
		if state != "ready" && state != "killed" {
			return
		}
		if !ent.sprite.Sprite().Idle() {
			return
		}
	}
	// Do Ai - if there is any to do
	if g.Side == SideHaunt {
		if g.Ai.minions.Active() {
			g.active_ai = g.Ai.minions
			g.Action_state = waitingAction
		} else {
			if g.Ai.denizens.Active() {
				g.active_ai = g.Ai.denizens
				g.Action_state = waitingAction
			}
		}
	} else {
		if g.Ai.intruders.Active() {
			g.active_ai = g.Ai.intruders
			g.Action_state = waitingAction
		}
	}
	if g.Action_state == waitingAction {
		select {
		case exec := <-g.active_ai.ActionExecs():
			if exec != nil {
				g.current_exec = exec
			} else {
				g.Action_state = noAction
				// TODO: indicate that the master ai can go now
			}

		default:
		}
	}
	if g.player_inactive && g.Action_state == noAction && !g.Ai.intruders.Active() && !g.Ai.denizens.Active() && !g.Ai.minions.Active() {
		g.Turn_state = turnStateMainPhaseOver
		base.Log().Printf("ScriptComm: change to turnStateMainPhaseOver")
		g.comm.game_to_script <- nil
		base.Log().Printf("ScriptComm: sent nil")
	}
}

func (g *Game) doLos(dist int, line [][2]int, los [][]bool) {
	var x0, y0, x, y int
	var room0, room *house.Room
	x, y = line[0][0], line[0][1]
	if x < 0 || y < 0 || x >= len(los) || y >= len(los[x]) {
		return
	}
	los[x][y] = true
	room = roomAt(g.House.Floors[0], x, y)
	for _, p := range line[1:] {
		x0, y0 = x, y
		x, y = p[0], p[1]
		if x < 0 || y < 0 || x >= len(los) || y >= len(los[x]) {
			return
		}
		room0 = room
		room = roomAt(g.House.Floors[0], x, y)
		if room == nil {
			return
		}
		if x == x0 || y == y0 {
			if room0 != nil && room0 != room && !connected(room, room0, x, y, x0, y0) {
				return
			}
		} else {
			roomA := roomAt(g.House.Floors[0], x0, y0)
			roomB := roomAt(g.House.Floors[0], x, y0)
			roomC := roomAt(g.House.Floors[0], x0, y)
			if roomA != nil && roomB != nil && roomA != roomB && !connected(roomA, roomB, x0, y0, x, y0) {
				return
			}
			if roomA != nil && roomC != nil && roomA != roomC && !connected(roomA, roomC, x0, y0, x0, y) {
				return
			}
			if roomB != nil && room != roomB && !connected(room, roomB, x, y, x, y0) {
				return
			}
			if roomC != nil && room != roomC && !connected(room, roomC, x, y, x0, y) {
				return
			}
		}
		furn := furnitureAt(room, x-room.X, y-room.Y)
		if furn != nil && furn.Blocks_los {
			return
		}
		dist -= 1 // or whatever
		if dist < 0 {
			return
		}
		los[x][y] = true
	}
}

func (g *Game) TeamLos(side Side, x, y, dx, dy int) bool {
	var team_los [][]byte
	if side == SideExplorers {
		team_los = g.los.intruders.tex.Pix()
	} else if side == SideHaunt {
		team_los = g.los.denizens.tex.Pix()
	} else {
		base.Warn().Printf("Can only ask for TeamLos for the intruders and denizens.")
		return false
	}
	if team_los == nil {
		return false
	}
	for i := x; i < x+dx; i++ {
		for j := y; j < y+dy; j++ {
			if i < 0 || j < 0 || i >= len(team_los) || j >= len(team_los[0]) {
				continue
			}
			if team_los[i][j] >= house.LosVisibilityThreshold {
				return true
			}
		}
	}
	return false
}

func (g *Game) ViewFrac(x, y, dx, dy int) float64 {
	if g == nil {
		return 1.0
	}
	team_los := g.viewer.Los_tex.Pix()
	for i := x; i < x+dx; i++ {
		for j := y; j < y+dy; j++ {
			if i < 0 || j < 0 || i >= len(team_los) || j >= len(team_los[0]) {
				continue
			}
			if team_los[i][j] >= house.LosVisibilityThreshold {
				return float64(team_los[i][j]-house.LosVisibilityThreshold) / float64(255-house.LosVisibilityThreshold)
			}
		}
	}
	return 0.0
}

func (g *Game) mergeLos(side Side) {
	var pix [][]byte
	switch side {
	case SideHaunt:
		pix = g.los.denizens.tex.Pix()
	case SideExplorers:
		pix = g.los.intruders.tex.Pix()
	default:
		base.Error().Printf("Unable to mergeLos on side %d.", side)
		return
	}
	for i := range g.los.full_merger {
		g.los.full_merger[i] = false
	}
	for _, ent := range g.Ents {
		if ent.Side() != side && !ent.Enemy_los {
			continue
		}
		if ent.los == nil {
			continue
		}
		for i := ent.los.minx; i <= ent.los.maxx; i++ {
			for j := ent.los.miny; j <= ent.los.maxy; j++ {
				if ent.los.grid[i][j] {
					g.los.merger[i][j] = true
				}
			}
		}
	}
	for i := 0; i < len(pix); i++ {
		for j := 0; j < len(pix); j++ {
			if g.los.merger[i][j] {
				continue
			}
			if pix[i][j] >= house.LosVisibilityThreshold {
				pix[i][j] = house.LosVisibilityThreshold - 1
			}
		}
	}
	for i := range g.los.merger {
		for j := range g.los.merger[i] {
			if g.los.merger[i][j] {
				if pix[i][j] < house.LosVisibilityThreshold {
					pix[i][j] = house.LosVisibilityThreshold
				}
			}
		}
	}
}

// This is the function used to determine LoS.  Nothing else should try to
// make any attempts at doing so.  Eventually this should be replaced with
// something more sensible and faster, so everyone needs to use this so that
// everything stays in sync.
func (g *Game) DetermineLos(x, y, los_dist int, grid [][]bool) {
	for i := range grid {
		for j := range grid[i] {
			grid[i][j] = false
		}
	}
	minx := x - los_dist
	miny := y - los_dist
	maxx := x + los_dist
	maxy := y + los_dist
	line := make([][2]int, los_dist)
	for vx := minx; vx <= maxx; vx++ {
		line = line[0:0]
		bresenham(x, y, vx, miny, &line)
		g.doLos(los_dist, line, grid)
		line = line[0:0]
		bresenham(x, y, vx, maxy, &line)
		g.doLos(los_dist, line, grid)
	}
	for vy := miny; vy <= maxy; vy++ {
		line = line[0:0]
		bresenham(x, y, minx, vy, &line)
		g.doLos(los_dist, line, grid)
		line = line[0:0]
		bresenham(x, y, maxx, vy, &line)
		g.doLos(los_dist, line, grid)
	}
}

func (g *Game) UpdateEntLos(ent *Entity, force bool) {
	if ent.los == nil || ent.Stats == nil {
		return
	}
	ex, ey := ent.Pos()
	if !force && ex == ent.los.x && ey == ent.los.y {
		return
	}
	base.Log().Printf("UpdateEntLos(%s): %t (%d, %d) -> (%d, %d)", ent.Name, force, ent.los.x, ent.los.y, ex, ey)
	ent.los.x = ex
	ent.los.y = ey

	g.DetermineLos(ex, ey, ent.Stats.Sight(), ent.los.grid)

	ent.los.minx = len(ent.los.grid)
	ent.los.miny = len(ent.los.grid)
	ent.los.maxx = 0
	ent.los.maxy = 0
	for i := range ent.los.grid {
		for j := range ent.los.grid[i] {
			if ent.los.grid[i][j] {
				if i < ent.los.minx {
					ent.los.minx = i
				}
				if j < ent.los.miny {
					ent.los.miny = j
				}
				if i > ent.los.maxx {
					ent.los.maxx = i
				}
				if j > ent.los.maxy {
					ent.los.maxy = j
				}
			}
		}
	}
}

// Uses Bresenham's alogirthm to determine the points to rasterize a line from
// x,y to x2,y2.
func bresenham(x, y, x2, y2 int, res *[][2]int) {
	dx := x2 - x
	if dx < 0 {
		dx = -dx
	}
	dy := y2 - y
	if dy < 0 {
		dy = -dy
	}

	steep := dy > dx
	if steep {
		x, y = y, x
		x2, y2 = y2, x2
		dx, dy = dy, dx
	}

	err := dx >> 1
	cy := y

	xstep := 1
	if x2 < x {
		xstep = -1
	}
	ystep := 1
	if y2 < y {
		ystep = -1
	}
	for cx := x; cx != x2; cx += xstep {
		if !steep {
			*res = append(*res, [2]int{cx, cy})
		} else {
			*res = append(*res, [2]int{cy, cx})
		}
		err -= dy
		if err < 0 {
			cy += ystep
			err += dx
		}
	}
	if !steep {
		*res = append(*res, [2]int{x2, cy})
	} else {
		*res = append(*res, [2]int{cy, x2})
	}
}
