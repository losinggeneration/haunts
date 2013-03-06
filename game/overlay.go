package game

import (
	gl "github.com/MobRulesGames/gogl/gl21"
	"github.com/MobRulesGames/glop/gui"
	"github.com/MobRulesGames/haunts/base"
	"time"
)

type Overlay struct {
	region gui.Region
	game   *Game
}

func MakeOverlay(g *Game) gui.Widget {
	return &Overlay{game: g}
}

func (o *Overlay) Requested() gui.Dims {
	return gui.Dims{1024, 768}
}
func (o *Overlay) Expandable() (bool, bool) {
	return false, false
}
func (o *Overlay) Rendered() gui.Region {
	return o.region
}
func (o *Overlay) Respond(g *gui.Gui, group gui.EventGroup) bool {
	return false
}
func (o *Overlay) Think(g *gui.Gui, dt int64) {
	var side Side
	if o.game.viewer.Los_tex == o.game.los.intruders.tex {
		side = SideExplorers
	} else if o.game.viewer.Los_tex == o.game.los.denizens.tex {
		side = SideHaunt
	} else {
		side = SideNone
	}

	for i := range o.game.Waypoints {
		o.game.viewer.RemoveFloorDrawable(&o.game.Waypoints[i])
		o.game.viewer.AddFloorDrawable(&o.game.Waypoints[i])
		o.game.Waypoints[i].active = o.game.Waypoints[i].Side == side
		o.game.Waypoints[i].drawn = false
	}
}
func (o *Overlay) Draw(region gui.Region) {
	o.region = region
	switch o.game.Side {
	case SideHaunt:
		if o.game.los.denizens.mode == LosModeBlind {
			return
		}
	case SideExplorers:
		if o.game.los.intruders.mode == LosModeBlind {
			return
		}
	default:
		return
	}
	for _, wp := range o.game.Waypoints {
		if !wp.active || wp.drawn {
			continue
		}

		cx := float32(wp.X)
		cy := float32(wp.Y)
		r := float32(wp.Radius)
		cx1, cy1 := o.game.viewer.BoardToWindow(cx-r, cy-r)
		cx2, cy2 := o.game.viewer.BoardToWindow(cx-r, cy+r)
		cx3, cy3 := o.game.viewer.BoardToWindow(cx+r, cy+r)
		cx4, cy4 := o.game.viewer.BoardToWindow(cx+r, cy-r)
		gl.Color4ub(200, 0, 0, 128)

		base.EnableShader("waypoint")
		base.SetUniformF("waypoint", "radius", float32(wp.Radius))

		t := float32(time.Now().UnixNano()%1e15) / 1.0e9
		base.SetUniformF("waypoint", "time", t)

		gl.Begin(gl.QUADS)
		gl.TexCoord2i(0, 1)
		gl.Vertex2i(gl.Int(cx1), gl.Int(cy1))
		gl.TexCoord2i(0, 0)
		gl.Vertex2i(gl.Int(cx2), gl.Int(cy2))
		gl.TexCoord2i(1, 0)
		gl.Vertex2i(gl.Int(cx3), gl.Int(cy3))
		gl.TexCoord2i(1, 1)
		gl.Vertex2i(gl.Int(cx4), gl.Int(cy4))
		gl.End()

		base.EnableShader("")
	}
}
func (o *Overlay) DrawFocused(region gui.Region) {
	o.Draw(region)
}
func (o *Overlay) String() string {
	return "overlay"
}
