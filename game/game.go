package game

import (
	"github.com/MobRulesGames/glop/gin"
	"github.com/MobRulesGames/glop/gui"
	"github.com/MobRulesGames/haunts/base"
	"github.com/MobRulesGames/haunts/house"
	"github.com/MobRulesGames/haunts/mrgnet"
	"math/rand"
	"sort"
)

type GamePanel struct {
	*gui.AnchorBox

	main_bar *MainBar

	// Keep track of this so we know how much time has passed between
	// calls to Think()
	last_think int64

	complete gui.Widget

	script *gameScript
	game   *Game
}

func MakeGamePanel(script string, p *Player, data map[string]string, game_key mrgnet.GameKey) *GamePanel {
	var gp GamePanel
	gp.AnchorBox = gui.MakeAnchorBox(gui.Dims{1024, 768})
	if p == nil {
		p = &Player{}
	}
	base.Log().Printf("Script path: %s / %s", script, p.Script_path)
	if script == "" {
		script = p.Script_path
	}
	startGameScript(&gp, script, p, data, game_key)
	return &gp
}

// Returns  true iff the game panel has an active game with a viewer already
// installed.
func (gp *GamePanel) Active() bool {
	return gp.game != nil && gp.game.House != nil && gp.game.viewer != nil
}

func (gp *GamePanel) Think(ui *gui.Gui, t int64) {
	gp.scriptThinkOnce()
	gp.AnchorBox.Think(ui, t)
	if !gp.Active() {
		return
	}
	if gp.game != nil {
		gp.game.modal = (ui.FocusWidget() != nil)
	}

	if gp.last_think == 0 {
		gp.last_think = t
	}
	dt := t - gp.last_think
	gp.last_think = t
	gp.game.Think(dt)

	if gp.main_bar != nil {
		if gp.game.selected_ent != nil {
			gp.main_bar.SelectEnt(gp.game.selected_ent)
		} else {
			gp.main_bar.SelectEnt(gp.game.hovered_ent)
		}
	}
}

func (gp *GamePanel) Draw(region gui.Region) {
	gp.AnchorBox.Draw(region)
	region.PushClipPlanes()
	defer region.PopClipPlanes()
}

func (g *Game) SpawnEntity(spawn *Entity, x, y int) bool {
	for i := range g.Ents {
		cx, cy := g.Ents[i].Pos()
		if cx == x && cy == y {
			base.Warn().Printf("Can't spawn entity at (%d, %d) - already occupied by '%s'.", x, y, g.Ents[i].Name)
			return false
		}
	}
	spawn.X = float64(x)
	spawn.Y = float64(y)
	spawn.Info.RoomsExplored[spawn.CurrentRoom()] = true
	g.Ents = append(g.Ents, spawn)
	return true
}

// Returns true iff the action was set
// This function will return false if there is no selected entity, if the
// action cannot be selected (because it is invalid or the entity has
// insufficient Ap), or if there is an action currently executing.
func (g *Game) SetCurrentAction(action Action) bool {
	if g.Action_state != noAction && g.Action_state != preppingAction {
		return false
	}
	// the action should be one that belongs to the current entity, if not then
	// we need to bail out immediately
	if g.selected_ent == nil {
		base.Warn().Printf("Tried to SetCurrentAction() without a selected entity.")
		return action == nil
	}
	if action != nil {
		valid := false
		for _, a := range g.selected_ent.Actions {
			if a == action {
				valid = true
				break
			}
		}
		if !valid {
			base.Warn().Printf("Tried to SetCurrentAction() with an action that did not belong to the selected entity.")
			return action == nil
		}
	}
	if g.current_action != nil {
		g.current_action.Cancel()
	}
	if action == nil {
		g.Action_state = noAction
	} else {
		g.Action_state = preppingAction
	}
	g.viewer.RemoveFloorDrawable(g.current_action)
	g.current_action = action
	if g.current_action != nil {
		g.viewer.AddFloorDrawable(g.current_action)
	}
	return true
}

func (gp *GamePanel) Respond(ui *gui.Gui, group gui.EventGroup) bool {
	if gp.AnchorBox.Respond(ui, group) {
		return true
	}
	if !gp.Active() {
		return false
	}

	if group.Events[0].Type == gin.Release {
		return false
	}

	cursor := group.Events[0].Key.Cursor()
	if cursor != nil {
		if gp.game.hovered_ent != nil {
			gp.game.hovered_ent.hovered = false
		}
		gp.game.hovered_ent = nil
		mx, my := cursor.Point()
		for i := range gp.game.Ents {
			fx, fy := gp.game.Ents[i].FPos()
			wx, wy := gp.game.viewer.BoardToWindow(float32(fx), float32(fy))
			if gp.game.Ents[i].Stats != nil && gp.game.Ents[i].Stats.HpCur() <= 0 {
				continue // Don't bother showing dead units
			}
			x := wx - int(gp.game.Ents[i].last_render_width/2)
			y := wy
			x2 := wx + int(gp.game.Ents[i].last_render_width/2)
			y2 := wy + int(150*gp.game.Ents[i].last_render_width/100)
			if mx >= x && mx <= x2 && my >= y && my <= y2 {
				if gp.game.hovered_ent != nil {
					gp.game.hovered_ent.hovered = false
				}
				gp.game.hovered_ent = gp.game.Ents[i]
				gp.game.hovered_ent.hovered = true
			}
		}
	}

	if found, event := group.FindEvent(gin.Escape); found && event.Type == gin.Press {
		if gp.game.selected_ent != nil {
			switch gp.game.Action_state {
			case noAction:
				gp.game.selected_ent.selected = false
				gp.game.selected_ent.hovered = false
				gp.game.selected_ent = nil
				return true

			case preppingAction:
				gp.game.SetCurrentAction(nil)
				return true

			case doingAction:
				// Do nothing - we don't cancel an action that's in progress
			}
		}
	}

	if gp.game.Action_state == noAction {
		if found, _ := group.FindEvent(gin.MouseLButton); found {
			if gp.game.hovered_ent != nil && gp.game.hovered_ent.Side() == gp.game.Side {
				if gp.game.selected_ent != nil {
					gp.game.selected_ent.selected = false
				}
				gp.game.selected_ent = gp.game.hovered_ent
				gp.game.selected_ent.selected = true
			}
			return true
		}
	}

	if gp.game.Action_state == preppingAction {
		consumed, exec := gp.game.current_action.HandleInput(group, gp.game)
		if consumed {
			if exec != nil {
				gp.game.current_exec = exec
				// TODO: Should send the exec across the wire here
			}
			return true
		}
	}

	// After this point all events that we check for require that we have a
	// selected entity
	if gp.game.selected_ent == nil {
		return false
	}
	if gp.game.Action_state == noAction || gp.game.Action_state == preppingAction {
		if len(group.Events) == 1 && group.Events[0].Key.Id() >= '1' && group.Events[0].Key.Id() <= '9' {
			index := int(group.Events[0].Key.Id() - '1')
			if index >= 0 && index < len(gp.game.selected_ent.Actions) {
				action := gp.game.selected_ent.Actions[index]
				if action != gp.game.current_action && action.Prep(gp.game.selected_ent, gp.game) {
					gp.game.SetCurrentAction(action)
				}
			}
		}
	}

	return false
}

type orderEntsBigToSmall []*Entity

func (o orderEntsBigToSmall) Len() int {
	return len(o)
}
func (o orderEntsBigToSmall) Swap(i, j int) {
	o[i], o[j] = o[j], o[i]
}
func (o orderEntsBigToSmall) Less(i, j int) bool {
	return o[i].Dx*o[i].Dy > o[j].Dx*o[j].Dy
}

type orderSpawnsSmallToBig []*house.SpawnPoint

func (o orderSpawnsSmallToBig) Len() int {
	return len(o)
}
func (o orderSpawnsSmallToBig) Swap(i, j int) {
	o[i], o[j] = o[j], o[i]
}
func (o orderSpawnsSmallToBig) Less(i, j int) bool {
	return o[i].Dx*o[i].Dy < o[j].Dx*o[j].Dy
}

type entSpawnPair struct {
	ent   *Entity
	spawn *house.SpawnPoint
}

// Distributes the ents among the spawn points.  Since this is done randomly
// it might not work, so there is a very small chance that not all spawns will
// have an ent given to them, even if it is possible to distrbiute them
// properly.  Regardless, at least some will be spawned.
func spawnEnts(g *Game, ents []*Entity, spawns []*house.SpawnPoint) {
	sort.Sort(orderSpawnsSmallToBig(spawns))
	sanity := 100
	var places []entSpawnPair
	for sanity > 0 {
		sanity--
		places = places[0:0]
		sort.Sort(orderEntsBigToSmall(ents))
		//slightly shuffle the ents
		for i := range ents {
			j := i + rand.Intn(5) - 2
			if j >= 0 && j < len(ents) {
				ents[i], ents[j] = ents[j], ents[i]
			}
		}
		// Go through each ent and try to place it in an unused spawn point
		used_spawns := make(map[*house.SpawnPoint]bool)
		for _, ent := range ents {
			for _, spawn := range spawns {
				if used_spawns[spawn] {
					continue
				}
				if spawn.Dx < ent.Dx || spawn.Dy < ent.Dy {
					continue
				}
				used_spawns[spawn] = true
				places = append(places, entSpawnPair{ent, spawn})
				break
			}
		}
		if len(places) == len(spawns) {
			break
		}
	}
	if sanity > 0 {
		base.Log().Printf("Placed all objects with %d sanity remaining", sanity)
	} else {
		base.Warn().Printf("Only able to place %d out of %d objects", len(places), len(spawns))
	}
	for _, place := range places {
		place.ent.X = float64(place.spawn.X + rand.Intn(place.spawn.Dx-place.ent.Dx+1))
		place.ent.Y = float64(place.spawn.Y + rand.Intn(place.spawn.Dy-place.ent.Dy+1))
		g.viewer.AddDrawable(place.ent)
		g.Ents = append(g.Ents, place.ent)
		base.Log().Printf("Using object '%s' at (%.0f, %.0f)", place.ent.Name, place.ent.X, place.ent.Y)
	}
}

func (gp *GamePanel) GetViewer() house.Viewer {
	return gp.game.viewer
}
