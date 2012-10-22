package game

import (
	"github.com/runningwild/glop/gui"
)

// SUPER simple scrolling region
type ScrollingRegion struct {
	X, Y, Dx, Dy int
	Height       int
	pos          float64
	target       float64
}

func (sr *ScrollingRegion) Up() {
	sr.target -= float64(sr.Dy)
}
func (sr *ScrollingRegion) Down() {
	sr.target += float64(sr.Dy)
}
func (sr *ScrollingRegion) Top() int {
	return int(sr.pos) + sr.Y + sr.Dy
}
func (sr *ScrollingRegion) Think(dt int64) {
	if sr.target > float64(sr.Height-sr.Dy) {
		sr.target = float64(sr.Height - sr.Dy)
	}
	if sr.target < 0 {
		sr.target = 0
	}
	sr.pos = doApproach(sr.pos, sr.target, dt)
}
func (sr *ScrollingRegion) Region() gui.Region {
	return gui.Region{gui.Point{sr.X, sr.Y}, gui.Dims{sr.Dx, sr.Dy}}
}
