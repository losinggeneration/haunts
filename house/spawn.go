package house

import (
	gl "github.com/MobRulesGames/gogl/gl21"
	"github.com/MobRulesGames/haunts/base"
	"github.com/MobRulesGames/haunts/texture"
	"github.com/MobRulesGames/mathgl"
	"hash/fnv"
	"regexp"
)

var spawn_regex []*regexp.Regexp

func PushSpawnRegexp(pattern string) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		base.Error().Printf("Unable to compile regexp: '%s': %v", pattern, err)
		// Just duplicate the top one, since this will probably come with an
		// associated pop.
		spawn_regex = append(spawn_regex, topSpawnRegexp())
		return
	}
	spawn_regex = append(spawn_regex, re)
}
func PopSpawnRegexp() {
	if len(spawn_regex) == 0 {
		base.Error().Printf("Tried to pop an empty stack.")
		return
	}
	spawn_regex = spawn_regex[0 : len(spawn_regex)-1]
}
func topSpawnRegexp() *regexp.Regexp {
	if len(spawn_regex) == 0 {
		return nil
	}
	return spawn_regex[len(spawn_regex)-1]
}

type SpawnPoint struct {
	Name   string
	Dx, Dy int
	X, Y   int

	// just for the shader
	temporary, invalid bool
}

func (sp *SpawnPoint) Dims() (int, int) {
	return sp.Dx, sp.Dy
}
func (sp *SpawnPoint) Pos() (int, int) {
	return sp.X, sp.Y
}
func (sp *SpawnPoint) FPos() (float64, float64) {
	return float64(sp.X), float64(sp.Y)
}
func (sp *SpawnPoint) Color() (r, g, b, a byte) {
	return 255, 255, 255, 255
}
func (sp *SpawnPoint) Render(pos mathgl.Vec2, width float32) {
	gl.Disable(gl.TEXTURE_2D)
	gl.Color4d(1, 1, 1, 0.1)
	gl.Begin(gl.QUADS)
	gl.Vertex2f(gl.Float(pos.X-width/2), gl.Float(pos.Y))
	gl.Vertex2f(gl.Float(pos.X-width/2), gl.Float(pos.Y+width))
	gl.Vertex2f(gl.Float(pos.X+width/2), gl.Float(pos.Y+width))
	gl.Vertex2f(gl.Float(pos.X+width/2), gl.Float(pos.Y))
	gl.End()
}
func (sp *SpawnPoint) RenderOnFloor() {
	re := topSpawnRegexp()
	if re == nil || !re.MatchString(sp.Name) {
		return
	}

	var rgba [4]float64
	gl.GetDoublev(gl.CURRENT_COLOR, (*gl.Double)(&rgba[0]))
	gl.PushAttrib(gl.CURRENT_BIT)
	gl.Disable(gl.TEXTURE_2D)

	// This just creates a color that is consistent among all spawn points whose
	// names match SpawnName-.*
	prefix := sp.Name
	for i := range prefix {
		if prefix[i] == '-' {
			prefix = prefix[0:i]
			break
		}
	}
	h := fnv.New32()
	h.Write([]byte(prefix))
	hs := h.Sum32()
	gl.Color4ub(gl.Ubyte(hs%256), gl.Ubyte((hs/256)%256), gl.Ubyte((hs/(256*256))%256), gl.Ubyte(255*rgba[3]))

	base.EnableShader("box")
	base.SetUniformF("box", "dx", float32(sp.Dx))
	base.SetUniformF("box", "dy", float32(sp.Dy))
	if !sp.temporary {
		base.SetUniformI("box", "temp_invalid", 0)
	} else if !sp.invalid {
		base.SetUniformI("box", "temp_invalid", 1)
	} else {
		base.SetUniformI("box", "temp_invalid", 2)
	}
	(&texture.Object{}).Data().Render(float64(sp.X), float64(sp.Y), float64(sp.Dx), float64(sp.Dy))
	base.EnableShader("")
	gl.PopAttrib()
}
