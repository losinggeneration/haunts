package house

import (
  gl "github.com/MobRulesGames/gogl/gl21"
  "github.com/MobRulesGames/haunts/base"
  "github.com/MobRulesGames/haunts/texture"
  "github.com/MobRulesGames/mathgl"
  "unsafe"
)

func MakeWallTexture(name string) *WallTexture {
  wt := WallTexture{Defname: name}
  wt.Load()
  return &wt
}

func GetAllWallTextureNames() []string {
  return base.GetAllNamesInRegistry("wall_textures")
}

func LoadAllWallTexturesInDir(dir string) {
  base.RemoveRegistry("wall_textures")
  base.RegisterRegistry("wall_textures", make(map[string]*wallTextureDef))
  base.RegisterAllObjectsInDir("wall_textures", dir, ".json", "json")
}

func (wt *WallTexture) Load() {
  base.GetObject("wall_textures", wt)
}

type wallTextureGlIds struct {
  vbuffer gl.Uint

  left_buffer  gl.Uint
  left_count   gl.Sizei
  right_buffer gl.Uint
  right_count  gl.Sizei
  floor_buffer gl.Uint
  floor_count  gl.Sizei
}

type wallTextureState struct {
  // for tracking whether the buffers are dirty
  x, y, rot float32
  flip      bool
  room      struct {
    x, y, dx, dy int
  }
}

type WallTexture struct {
  Defname string
  *wallTextureDef

  // Position of the texture in floor coordinates.  If these coordinates exceed
  // either the dx or dy of the room, then this texture will be drawn, at least
  // partially, on the wall.  The coordinates should not both exceed the
  // dimensions of the room.
  X, Y float32
  Rot  float32

  // Whether or not to flip the texture about one of its axes
  Flip bool

  // If this is currently being dragged around it will be marked as temporary
  // so that it will be drawn differently
  temporary bool
}

type wallTextureDef struct {
  // Name of this texture as it appears in the editor, should be unique among
  // all WallTextures
  Name string

  Texture texture.Object
}

func (wt *WallTexture) Color() (r, g, b, a byte) {
  if wt.temporary {
    return 127, 127, 255, 200
  }
  return 255, 255, 255, 255
}

func (wt *WallTexture) Render() {
  dx2 := float32(wt.Texture.Data().Dx()) / 100 / 2
  dy2 := float32(wt.Texture.Data().Dy()) / 100 / 2
  wt.Texture.Data().RenderAdvanced(float64(wt.X-dx2), float64(wt.Y-dy2), float64(2*dx2), float64(2*dy2), float64(wt.Rot), wt.Flip)
}

func (wt *WallTexture) setupGlStuff(x, y, dx, dy int, gl_ids *wallTextureGlIds) {
  if gl_ids.vbuffer != 0 {
    gl.DeleteBuffers(1, (*gl.Uint)(&gl_ids.vbuffer))
    gl.DeleteBuffers(1, (*gl.Uint)(&gl_ids.left_buffer))
    gl.DeleteBuffers(1, (*gl.Uint)(&gl_ids.right_buffer))
    gl.DeleteBuffers(1, (*gl.Uint)(&gl_ids.floor_buffer))
    gl_ids.vbuffer = 0
    gl_ids.left_buffer = 0
    gl_ids.right_buffer = 0
    gl_ids.floor_buffer = 0
  }

  // All vertices for both walls and the floor will go here and get sent to
  // opengl all at once
  var vs []roomVertex

  // Conveniently casted values
  frx := float32(x)
  fry := float32(y)
  frdx := float32(dx)
  frdy := float32(dy)
  tdx := float32(wt.Texture.Data().Dx()) / 100
  tdy := float32(wt.Texture.Data().Dy()) / 100

  wtx := wt.X
  wty := wt.Y
  wtr := wt.Rot

  if wtx > frdx {
    wtr -= 3.1415926535 / 2
  }

  // Floor
  verts := []mathgl.Vec2{
    {-tdx / 2, -tdy / 2},
    {-tdx / 2, tdy / 2},
    {tdx / 2, tdy / 2},
    {tdx / 2, -tdy / 2},
  }
  var m, run mathgl.Mat3
  run.Identity()
  m.Translation(wtx, wty)
  run.Multiply(&m)
  m.RotationZ(wtr)
  run.Multiply(&m)
  if wt.Flip {
    m.Scaling(-1, 1)
    run.Multiply(&m)
  }
  for i := range verts {
    verts[i].Transform(&run)
  }
  p := mathgl.Poly(verts)
  p.Clip(&mathgl.Seg2{A: mathgl.Vec2{0, 0}, B: mathgl.Vec2{0, frdy}})
  p.Clip(&mathgl.Seg2{A: mathgl.Vec2{0, frdy}, B: mathgl.Vec2{frdx, frdy}})
  p.Clip(&mathgl.Seg2{A: mathgl.Vec2{frdx, frdy}, B: mathgl.Vec2{frdx, 0}})
  p.Clip(&mathgl.Seg2{A: mathgl.Vec2{frdx, 0}, B: mathgl.Vec2{0, 0}})
  if len(p) >= 3 {
    // floor indices
    var is []uint16
    for i := 1; i < len(p)-1; i++ {
      is = append(is, uint16(len(vs)+0))
      is = append(is, uint16(len(vs)+i))
      is = append(is, uint16(len(vs)+i+1))
    }
    gl.GenBuffers(1, (*gl.Uint)(&gl_ids.floor_buffer))
    gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, gl.Uint(gl_ids.floor_buffer))
    gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, gl.Sizeiptr(int(unsafe.Sizeof(is[0]))*len(is)), gl.Pointer(&is[0]), gl.STATIC_DRAW)
    gl_ids.floor_count = gl.Sizei(len(is))

    run.Inverse()
    for i := range p {
      v := mathgl.Vec2{p[i].X, p[i].Y}
      v.Transform(&run)
      vs = append(vs, roomVertex{
        x:     p[i].X,
        y:     p[i].Y,
        u:     v.X/tdx + 0.5,
        v:     -(v.Y/tdy + 0.5),
        los_u: (fry + p[i].Y) / LosTextureSize,
        los_v: (frx + p[i].X) / LosTextureSize,
      })
    }
  }

  // Left Wall
  verts = []mathgl.Vec2{
    {-tdx / 2, -tdy / 2},
    {-tdx / 2, tdy / 2},
    {tdx / 2, tdy / 2},
    {tdx / 2, -tdy / 2},
  }
  run.Identity()
  m.Translation(wtx, wty)
  run.Multiply(&m)
  m.RotationZ(wtr)
  run.Multiply(&m)
  if wt.Flip {
    m.Scaling(-1, 1)
    run.Multiply(&m)
  }
  for i := range verts {
    verts[i].Transform(&run)
  }
  p = mathgl.Poly(verts)
  p.Clip(&mathgl.Seg2{A: mathgl.Vec2{0, 0}, B: mathgl.Vec2{0, frdy}})
  p.Clip(&mathgl.Seg2{B: mathgl.Vec2{0, frdy}, A: mathgl.Vec2{frdx, frdy}})
  p.Clip(&mathgl.Seg2{A: mathgl.Vec2{frdx, frdy}, B: mathgl.Vec2{frdx, 0}})
  if len(p) >= 3 {
    // floor indices
    var is []uint16
    for i := 1; i < len(p)-1; i++ {
      is = append(is, uint16(len(vs)+0))
      is = append(is, uint16(len(vs)+i))
      is = append(is, uint16(len(vs)+i+1))
    }
    gl.GenBuffers(1, (*gl.Uint)(&gl_ids.left_buffer))
    gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, gl.Uint(gl_ids.left_buffer))
    gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, gl.Sizeiptr(int(unsafe.Sizeof(is[0]))*len(is)), gl.Pointer(&is[0]), gl.STATIC_DRAW)
    gl_ids.left_count = gl.Sizei(len(is))

    run.Inverse()
    for i := range p {
      v := mathgl.Vec2{p[i].X, p[i].Y}
      v.Transform(&run)
      vs = append(vs, roomVertex{
        x:     p[i].X,
        y:     frdy,
        z:     frdy - p[i].Y,
        u:     v.X/tdx + 0.5,
        v:     -(v.Y/tdy + 0.5),
        los_u: (fry + frdy - 0.5) / LosTextureSize,
        los_v: (frx + p[i].X) / LosTextureSize,
      })
    }
  }

  // Right Wall
  verts = []mathgl.Vec2{
    {-tdx / 2, -tdy / 2},
    {-tdx / 2, tdy / 2},
    {tdx / 2, tdy / 2},
    {tdx / 2, -tdy / 2},
  }
  run.Identity()
  m.Translation(wtx, wty)
  run.Multiply(&m)
  m.RotationZ(wtr)
  run.Multiply(&m)
  if wt.Flip {
    m.Scaling(-1, 1)
    run.Multiply(&m)
  }
  for i := range verts {
    verts[i].Transform(&run)
  }
  p = mathgl.Poly(verts)
  p.Clip(&mathgl.Seg2{A: mathgl.Vec2{0, frdy}, B: mathgl.Vec2{frdx, frdy}})
  p.Clip(&mathgl.Seg2{B: mathgl.Vec2{frdx, frdy}, A: mathgl.Vec2{frdx, 0}})
  p.Clip(&mathgl.Seg2{A: mathgl.Vec2{frdx, 0}, B: mathgl.Vec2{0, 0}})
  if len(p) >= 3 {
    // floor indices
    var is []uint16
    for i := 1; i < len(p)-1; i++ {
      is = append(is, uint16(len(vs)+0))
      is = append(is, uint16(len(vs)+i))
      is = append(is, uint16(len(vs)+i+1))
    }
    gl.GenBuffers(1, (*gl.Uint)(&gl_ids.right_buffer))
    gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, gl.Uint(gl_ids.right_buffer))
    gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, gl.Sizeiptr(int(unsafe.Sizeof(is[0]))*len(is)), gl.Pointer(&is[0]), gl.STATIC_DRAW)
    gl_ids.right_count = gl.Sizei(len(is))

    run.Inverse()
    for i := range p {
      v := mathgl.Vec2{p[i].X, p[i].Y}
      v.Transform(&run)
      vs = append(vs, roomVertex{
        x:     frdx,
        y:     p[i].Y,
        z:     frdx - p[i].X,
        u:     v.X/tdx + 0.5,
        v:     -(v.Y/tdy + 0.5),
        los_u: (fry + p[i].Y) / LosTextureSize,
        los_v: (frx + frdx - 0.5) / LosTextureSize,
      })
    }
  }

  if len(vs) > 0 {
    gl.GenBuffers(1, (*gl.Uint)(&gl_ids.vbuffer))
    gl.BindBuffer(gl.ARRAY_BUFFER, gl.Uint(gl_ids.vbuffer))
    size := int(unsafe.Sizeof(roomVertex{}))
    gl.BufferData(gl.ARRAY_BUFFER, gl.Sizeiptr(size*len(vs)), gl.Pointer(&vs[0].x), gl.STATIC_DRAW)
  }
}
