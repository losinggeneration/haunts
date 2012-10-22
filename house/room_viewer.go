package house

import (
	"github.com/runningwild/glop/gin"
	"github.com/runningwild/glop/gui"
	"github.com/runningwild/haunts/base"
	"github.com/runningwild/mathgl"
	"github.com/runningwild/opengl/gl"
	"math"
)

type RectObject interface {
	// Position in board coordinates
	Pos() (int, int)

	// Dimensions in board coordinates
	Dims() (int, int)
}

type FloorDrawer interface {
	// Draws stuff on the floor.  This will be called after the floor and all
	// textures on it have been drawn, but before furniture has been drawn.
	RenderOnFloor()

	RectObject
}

type Drawable interface {
	RectObject
	FPos() (float64, float64)
	Render(pos mathgl.Vec2, width float32)
	Color() (r, g, b, a byte)
}

type editMode int

const (
	editNothing editMode = iota
	editFurniture
	editWallTextures
	editCells
)

type RoomViewer struct {
	gui.Childless
	gui.EmbeddedWidget
	gui.BasicZone
	gui.NonFocuser
	gui.NonResponder
	gui.NonThinker

	room *Room

	// In case the size of the room changes we will need to update the matrices
	size RoomSize

	// All events received by the viewer are passed to the handler
	handler gin.EventHandler

	// Focus, in map coordinates
	fx, fy float32

	// Mouse position, in board coordinates
	mx, my int

	// The viewing angle, 0 means the map is viewed head-on, 90 means the map is viewed
	// on its edge (i.e. it would not be visible)
	angle float32

	// Zoom factor, 1.0 is standard
	zoom float32

	// The modelview matrix that is sent to opengl.  Updated any time focus, zoom, or viewing
	// angle changes
	mat            mathgl.Mat4
	left_wall_mat  mathgl.Mat4
	right_wall_mat mathgl.Mat4

	// Inverse of mat
	imat            mathgl.Mat4
	left_wall_imat  mathgl.Mat4
	right_wall_imat mathgl.Mat4

	Temp struct {
		Furniture   *Furniture
		WallTexture *WallTexture
	}

	// This tells us what to highlight based on the mouse position
	edit_mode editMode

	// Keeping some things here to avoid unnecessary allocations elsewhere
	cstack base.ColorStack
}

func (rv *RoomViewer) SetEditMode(mode editMode) {
	rv.edit_mode = mode
}

func (rv *RoomViewer) String() string {
	return "viewer"
}

func MakeRoomViewer(room *roomDef, angle float32) *RoomViewer {
	var rv RoomViewer
	rv.EmbeddedWidget = &gui.BasicWidget{CoreWidget: &rv}
	rv.room = &Room{roomDef: room}
	rv.angle = angle
	rv.fx = float32(rv.room.Size.Dx / 2)
	rv.fy = float32(rv.room.Size.Dy / 2)
	rv.Zoom(1)
	rv.size = rv.room.Size
	rv.makeMat()
	rv.Request_dims.Dx = 100
	rv.Request_dims.Dy = 100
	rv.Ex = true
	rv.Ey = true

	return &rv
}

func (rv *RoomViewer) AdjAngle(ang float32) {
	rv.angle = ang
	rv.makeMat()
}

func (rv *RoomViewer) Drag(dx, dy float64) {
	v := mathgl.Vec3{X: rv.fx, Y: rv.fy}
	vx := mathgl.Vec3{1, -1, 0}
	vx.Normalize()
	vy := mathgl.Vec3{1, 1, 0}
	vy.Normalize()
	vx.Scale(float32(dx) / rv.zoom * 2)
	vy.Scale(float32(dy) / rv.zoom * 2)
	v.Add(&vx)
	v.Add(&vy)
	rv.fx, rv.fy = v.X, v.Y
	rv.makeMat()
}

func (rv *RoomViewer) makeMat() {
	rv.mat, rv.imat, rv.left_wall_mat, rv.left_wall_imat, rv.right_wall_mat, rv.right_wall_imat = makeRoomMats(rv.room.roomDef, rv.Render_region, rv.fx, rv.fy, rv.angle, rv.zoom)
}

func makeRoomMats(room *roomDef, region gui.Region, focusx, focusy, angle, zoom float32) (floor, ifloor, left, ileft, right, iright mathgl.Mat4) {
	var m mathgl.Mat4
	floor.Translation(float32(region.Dx/2+region.X), float32(region.Dy/2+region.Y), 0)

	// NOTE: If we want to change 45 to *anything* else then we need to do the
	// appropriate math for rendering quads for furniture
	m.RotationZ(45 * math.Pi / 180)
	floor.Multiply(&m)
	m.RotationAxisAngle(mathgl.Vec3{X: -1, Y: 1}, -float32(angle)*math.Pi/180)
	floor.Multiply(&m)

	s := float32(zoom)
	m.Scaling(s, s, s)
	floor.Multiply(&m)

	// Move the viewer so that the focus is at the origin, and hence becomes centered
	// in the window
	m.Translation(-focusx, -focusy, 0)
	floor.Multiply(&m)

	ifloor.Assign(&floor)
	ifloor.Inverse()

	// Also make the mats for the left and right walls based on this mat
	left.Assign(&floor)
	m.RotationX(-math.Pi / 2)
	left.Multiply(&m)
	m.Translation(0, 0, float32(room.Size.Dy))
	left.Multiply(&m)
	ileft.Assign(&left)
	ileft.Inverse()

	right.Assign(&floor)
	m.RotationX(-math.Pi / 2)
	right.Multiply(&m)
	m.RotationY(-math.Pi / 2)
	right.Multiply(&m)
	m.Scaling(1, 1, 1)
	right.Multiply(&m)
	m.Translation(0, 0, -float32(room.Size.Dx))
	right.Multiply(&m)
	swap_x_y := mathgl.Mat4{
		0, 1, 0, 0,
		1, 0, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
	right.Multiply(&swap_x_y)
	iright.Assign(&right)
	iright.Inverse()

	return floor, ifloor, left, ileft, right, iright
}

// Transforms a cursor position in window coordinates to board coordinates.
func (rv *RoomViewer) WindowToBoard(wx, wy int) (float32, float32) {
	return rv.WindowToBoardf(float32(wx), float32(wy))
}
func (rv *RoomViewer) WindowToBoardf(wx, wy float32) (float32, float32) {
	fx, fy, fdist := rv.modelviewToBoard(wx, wy)
	lbx, lby, ldist := rv.modelviewToLeftWall(wx, wy)
	rbx, rby, rdist := rv.modelviewToRightWall(wx, wy)
	if fdist < ldist && fdist < rdist {
		if fx > float32(rv.room.Size.Dx) {
			fx = float32(rv.room.Size.Dx)
		}
		if fy > float32(rv.room.Size.Dy) {
			fy = float32(rv.room.Size.Dy)
		}
		return fx, fy
	}
	if ldist < rdist {
		return lbx, lby
	}
	return rbx, rby
}

func (rv *RoomViewer) BoardToWindow(bx, by float32) (int, int) {
	x, y := rv.BoardToWindowf(bx, by)
	return int(x), int(y)
}
func (rv *RoomViewer) BoardToWindowf(bx, by float32) (float32, float32) {
	fx, fy, fz := rv.boardToModelview(float32(bx), float32(by))
	lbx, lby, lz := rv.leftWallToModelview(float32(bx), float32(by))
	rbx, rby, rz := rv.rightWallToModelview(float32(bx), float32(by))
	if fz < lz && fz < rz {
		return fx, fy
	}
	if lz < rz {
		return lbx, lby
	}
	return rbx, rby
}

func (rv *RoomViewer) modelviewToLeftWall(mx, my float32) (x, y, dist float32) {
	mz := d2p(rv.left_wall_mat, mathgl.Vec3{mx, my, 0}, mathgl.Vec3{0, 0, 1})
	v := mathgl.Vec4{X: mx, Y: my, Z: mz, W: 1}
	v.Transform(&rv.left_wall_imat)
	if v.X > float32(rv.room.Size.Dx) {
		v.X = float32(rv.room.Size.Dx)
	}
	return v.X, v.Y + float32(rv.room.Size.Dy), mz
}

func (rv *RoomViewer) modelviewToRightWall(mx, my float32) (x, y, dist float32) {
	mz := d2p(rv.right_wall_mat, mathgl.Vec3{mx, my, 0}, mathgl.Vec3{0, 0, 1})
	v := mathgl.Vec4{X: mx, Y: my, Z: mz, W: 1}
	v.Transform(&rv.right_wall_imat)
	if v.Y > float32(rv.room.Size.Dy) {
		v.Y = float32(rv.room.Size.Dy)
	}
	return v.X + float32(rv.room.Size.Dx), v.Y, mz
}

func (rv *RoomViewer) leftWallToModelview(bx, by float32) (x, y, z float32) {
	v := mathgl.Vec4{X: bx, Y: by - float32(rv.room.Size.Dy), W: 1}
	v.Transform(&rv.left_wall_mat)
	return v.X, v.Y, v.Z
}

func (rv *RoomViewer) rightWallToModelview(bx, by float32) (x, y, z float32) {
	v := mathgl.Vec4{X: bx - float32(rv.room.Size.Dx), Y: by, W: 1}
	v.Transform(&rv.right_wall_mat)
	return v.X, v.Y, v.Z
}

// Distance to Plane(Point?)?  WTF IS THIS!?
func d2p(tmat mathgl.Mat4, point, ray mathgl.Vec3) float32 {
	var mat mathgl.Mat4
	mat.Assign(&tmat)
	var sub mathgl.Vec3
	sub.X = mat[12]
	sub.Y = mat[13]
	sub.Z = mat[14]
	mat[12], mat[13], mat[14] = 0, 0, 0
	point.Subtract(&sub)
	point.Scale(-1)
	ray.Normalize()
	dist := point.Dot(mat.GetForwardVec3())

	var forward mathgl.Vec3
	forward.Assign(mat.GetForwardVec3())
	cos := float64(forward.Dot(&ray))
	return dist / float32(cos)
}

func (rv *RoomViewer) modelviewToBoard(mx, my float32) (x, y, dist float32) {
	mz := d2p(rv.mat, mathgl.Vec3{mx, my, 0}, mathgl.Vec3{0, 0, 1})
	//  mz := (my - float32(rv.Render_region.Y+rv.Render_region.Dy/2)) * float32(math.Tan(float64(rv.angle*math.Pi/180)))
	v := mathgl.Vec4{X: mx, Y: my, Z: mz, W: 1}
	v.Transform(&rv.imat)
	return v.X, v.Y, mz
}

func (rv *RoomViewer) boardToModelview(mx, my float32) (x, y, z float32) {
	v := mathgl.Vec4{X: mx, Y: my, W: 1}
	v.Transform(&rv.mat)
	x, y, z = v.X, v.Y, v.Z
	return
}

func clamp(f, min, max float32) float32 {
	if f < min {
		return min
	}
	if f > max {
		return max
	}
	return f
}

// Changes the current zoom from e^(zoom) to e^(zoom+dz)
func (rv *RoomViewer) Zoom(dz float64) {
	if dz == 0 {
		return
	}
	exp := math.Log(float64(rv.zoom)) + dz
	exp = float64(clamp(float32(exp), 2.5, 5.0))
	rv.zoom = float32(math.Exp(exp))
	rv.makeMat()
}

func drawPrep() {
	gl.Disable(gl.DEPTH_TEST)
	gl.Disable(gl.TEXTURE_2D)
	gl.PolygonMode(gl.FRONT_AND_BACK, gl.FILL)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.ClearStencil(0)
	gl.Clear(gl.STENCIL_BUFFER_BIT)
}

// A single slice of WallTextures that can be reused again and again so we can
// avoid reallocations.  Since only one drawWall or drawFloor function will
// ever execute at a time this is safe.
var g_texs []WallTexture

// Same kind of thing as g_texs but for doors
var g_doors []*Door

// And one for RectObjects
var g_stuff []RectObject

// room: the wall to draw
// wall: the texture to render on the wall
// temp: an additional texture to render along with the other detail textures
// specified in room
// left,right: the xy planes of the left and right walls
func drawWall(room *Room, floor, left, right mathgl.Mat4, temp_tex *WallTexture, temp_door doorInfo, cstack base.ColorStack, los_tex *LosTexture, los_alpha float64) {
	gl.Enable(gl.STENCIL_TEST)
	defer gl.Disable(gl.STENCIL_TEST)

	gl.MatrixMode(gl.MODELVIEW)
	gl.PushMatrix()
	defer gl.PopMatrix()

	var dz int
	if room.Wall.Data().Dx() > 0 {
		dz = room.Wall.Data().Dy() * (room.Size.Dx + room.Size.Dy) / room.Wall.Data().Dx()
	}
	corner := float32(room.Size.Dx) / float32(room.Size.Dx+room.Size.Dy)
	gl.LoadIdentity()
	gl.MultMatrixf(&floor[0])

	g_texs = g_texs[0:0]
	if temp_tex != nil {
		g_texs = append(g_texs, *temp_tex)
	}
	for _, tex := range room.WallTextures {
		g_texs = append(g_texs, *tex)
	}

	do_right_wall := func() {
		gl.Begin(gl.QUADS)
		gl.TexCoord2f(1, 0)
		gl.Vertex3i(room.Size.Dx, 0, 0)
		gl.TexCoord2f(1, -1)
		gl.Vertex3i(room.Size.Dx, 0, -dz)
		gl.TexCoord2f(corner, -1)
		gl.Vertex3i(room.Size.Dx, room.Size.Dy, -dz)
		gl.TexCoord2f(corner, 0)
		gl.Vertex3i(room.Size.Dx, room.Size.Dy, 0)
		gl.End()
	}

	g_doors = g_doors[0:0]
	for _, door := range room.Doors {
		g_doors = append(g_doors, door)
	}
	if temp_door.Door != nil {
		g_doors = append(g_doors, temp_door.Door)
	}

	alpha := 0.2

	do_right_doors := func(opened bool) {
		for _, door := range g_doors {
			if door.Facing != FarRight {
				continue
			}
			if door.IsOpened() != opened {
				continue
			}
			door.TextureData().Bind()
			if door == temp_door.Door {
				if temp_door.Valid {
					cstack.Push(0, 0, 1, alpha)
				} else {
					cstack.Push(1, 0, 0, alpha)
				}
			}
			cstack.ApplyWithAlpha(alpha * los_alpha)
			gl.Begin(gl.QUADS)
			height := float64(door.Width*door.TextureData().Dy()) / float64(door.TextureData().Dx())
			gl.TexCoord2f(1, 0)
			gl.Vertex3d(float64(room.Size.Dx), float64(door.Pos), 0)
			gl.TexCoord2f(1, -1)
			gl.Vertex3d(float64(room.Size.Dx), float64(door.Pos), -height)
			gl.TexCoord2f(0, -1)
			gl.Vertex3d(float64(room.Size.Dx), float64(door.Pos+door.Width), -height)
			gl.TexCoord2f(0, 0)
			gl.Vertex3d(float64(room.Size.Dx), float64(door.Pos+door.Width), 0)
			gl.End()
			if door == temp_door.Door {
				cstack.Pop()
			}
		}
	}

	// Right wall
	gl.StencilFunc(gl.NOTEQUAL, 8, 7)
	gl.StencilOp(gl.DECR_WRAP, gl.REPLACE, gl.REPLACE)
	gl.Color4d(0, 0, 0, 0)
	do_right_wall()
	gl.Enable(gl.TEXTURE_2D)
	cstack.ApplyWithAlpha(alpha * los_alpha)
	gl.StencilFunc(gl.EQUAL, 8, 15)
	gl.StencilOp(gl.KEEP, gl.ZERO, gl.ZERO)
	do_right_doors(true)
	cstack.ApplyWithAlpha(1.0 * los_alpha)
	gl.StencilFunc(gl.EQUAL, 15, 15)
	gl.StencilOp(gl.KEEP, gl.ZERO, gl.ZERO)
	do_right_doors(true)
	for _, alpha := range []float64{alpha, 1.0} {
		cstack.ApplyWithAlpha(alpha * los_alpha)
		if alpha == 1.0 {
			gl.StencilFunc(gl.EQUAL, 15, 15)
			gl.StencilOp(gl.KEEP, gl.KEEP, gl.KEEP)
		} else {
			gl.StencilFunc(gl.EQUAL, 8, 15)
			gl.StencilOp(gl.KEEP, gl.KEEP, gl.KEEP)
		}
		room.Wall.Data().Bind()

		do_right_wall()

		gl.PushMatrix()
		gl.LoadIdentity()
		gl.MultMatrixf(&right[0])
		for i, tex := range g_texs {
			dx, dy := float32(room.Size.Dx), float32(room.Size.Dy)
			if tex.Y > dy {
				tex.X, tex.Y = dx+tex.Y-dy, dy+dx-tex.X
			}
			if tex.X > dx {
				tex.Rot -= 3.1415926535 / 2
			}
			tex.X -= dx
			if temp_tex != nil && i == 0 {
				cstack.Push(1, 0.7, 0.7, 0.7)
			}
			cstack.ApplyWithAlpha(alpha * los_alpha)
			tex.Render()
			if temp_tex != nil && i == 0 {
				cstack.Pop()
			}
		}
		gl.PopMatrix()
	}
	cstack.ApplyWithAlpha(alpha * los_alpha)
	gl.StencilFunc(gl.EQUAL, 8, 15)
	do_right_doors(false)
	cstack.ApplyWithAlpha(1.0 * los_alpha)
	gl.StencilFunc(gl.EQUAL, 15, 15)
	do_right_doors(false)
	// Go back over the area we just drew on and replace it with all b0001
	gl.StencilFunc(gl.ALWAYS, 1, 1)
	gl.StencilOp(gl.REPLACE, gl.REPLACE, gl.REPLACE)
	gl.Color4d(0, 0, 0, 0)
	do_right_wall()

	// Now that the entire wall has been draw we can cast shadows on it if we've
	// got a los texture
	if los_tex != nil {
		los_tex.Bind()
		gl.BlendFunc(gl.SRC_ALPHA_SATURATE, gl.SRC_ALPHA)
		gl.Color4d(0, 0, 0, 1)

		tx := (float64(room.X+room.Size.Dx) - 0.5) / float64(los_tex.Size())
		ty := (float64(room.Y) + 0.5) / float64(los_tex.Size())
		ty2 := (float64(room.Y+room.Size.Dy) - 0.5) / float64(los_tex.Size())
		gl.Begin(gl.QUADS)
		gl.TexCoord2d(ty, tx)
		gl.Vertex3i(room.Size.Dx, 0, 0)
		gl.TexCoord2d(ty, tx)
		gl.Vertex3i(room.Size.Dx, 0, -dz)
		gl.TexCoord2d(ty2, tx)
		gl.Vertex3i(room.Size.Dx, room.Size.Dy, -dz)
		gl.TexCoord2d(ty2, tx)
		gl.Vertex3i(room.Size.Dx, room.Size.Dy, 0)
		gl.End()
		gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	}

	do_left_wall := func() {
		gl.Begin(gl.QUADS)
		gl.TexCoord2f(corner, 0)
		gl.Vertex3i(room.Size.Dx, room.Size.Dy, 0)
		gl.TexCoord2f(corner, -1)
		gl.Vertex3i(room.Size.Dx, room.Size.Dy, -dz)
		gl.TexCoord2f(0, -1)
		gl.Vertex3i(0, room.Size.Dy, -dz)
		gl.TexCoord2f(0, 0)
		gl.Vertex3i(0, room.Size.Dy, 0)
		gl.End()
	}

	do_left_doors := func(opened bool) {
		for _, door := range g_doors {
			if door.Facing != FarLeft {
				continue
			}
			if door.IsOpened() != opened {
				continue
			}
			door.TextureData().Bind()
			if door == temp_door.Door {
				if temp_door.Valid {
					cstack.Push(0, 0, 1, alpha)
				} else {
					cstack.Push(1, 0, 0, alpha)
				}
			}
			cstack.ApplyWithAlpha(alpha * los_alpha)
			gl.Begin(gl.QUADS)
			height := float64(door.Width*door.TextureData().Dy()) / float64(door.TextureData().Dx())
			gl.TexCoord2f(0, 0)
			gl.Vertex3d(float64(door.Pos), float64(room.Size.Dy), 0)
			gl.TexCoord2f(0, -1)
			gl.Vertex3d(float64(door.Pos), float64(room.Size.Dy), -height)
			gl.TexCoord2f(1, -1)
			gl.Vertex3d(float64(door.Pos+door.Width), float64(room.Size.Dy), -height)
			gl.TexCoord2f(1, 0)
			gl.Vertex3d(float64(door.Pos+door.Width), float64(room.Size.Dy), 0)
			gl.End()
			if door == temp_door.Door {
				cstack.Pop()
			}
		}
	}

	gl.StencilFunc(gl.NOTEQUAL, 8, 7)
	gl.StencilOp(gl.DECR_WRAP, gl.REPLACE, gl.REPLACE)
	gl.Color4d(0, 0, 0, 0)
	do_left_wall()
	gl.Enable(gl.TEXTURE_2D)
	cstack.ApplyWithAlpha(alpha * los_alpha)
	gl.StencilFunc(gl.EQUAL, 8, 15)
	gl.StencilOp(gl.KEEP, gl.ZERO, gl.ZERO)
	do_left_doors(true)
	cstack.ApplyWithAlpha(1.0 * los_alpha)
	gl.StencilFunc(gl.EQUAL, 15, 15)
	gl.StencilOp(gl.KEEP, gl.ZERO, gl.ZERO)
	do_left_doors(true)
	for _, alpha := range []float64{alpha, 1.0} {
		if alpha == 1.0 {
			gl.StencilFunc(gl.EQUAL, 15, 15)
			gl.StencilOp(gl.KEEP, gl.KEEP, gl.KEEP)
		} else {
			gl.StencilFunc(gl.EQUAL, 8, 15)
			gl.StencilOp(gl.KEEP, gl.KEEP, gl.KEEP)
		}
		room.Wall.Data().Bind()
		cstack.ApplyWithAlpha(alpha * los_alpha)
		do_left_wall()

		gl.PushMatrix()
		gl.LoadIdentity()
		gl.MultMatrixf(&left[0])
		for i, tex := range g_texs {
			dx, dy := float32(room.Size.Dx), float32(room.Size.Dy)
			if tex.X > dx {
				tex.X, tex.Y = dx+dy-tex.Y, dy+tex.X-dx
			}
			tex.Y -= dy
			if temp_tex != nil && i == 0 {
				cstack.Push(1, 0.7, 0.7, 0.7)
			}
			cstack.ApplyWithAlpha(alpha * los_alpha)
			tex.Render()
			if temp_tex != nil && i == 0 {
				cstack.Pop()
			}
		}
		gl.PopMatrix()
	}
	cstack.ApplyWithAlpha(alpha * los_alpha)
	gl.StencilFunc(gl.EQUAL, 8, 15)
	do_left_doors(false)
	cstack.ApplyWithAlpha(1.0 * los_alpha)
	gl.StencilFunc(gl.EQUAL, 15, 15)
	do_left_doors(false)
	// Go back over the area we just drew on and replace it with all b0010
	gl.StencilFunc(gl.ALWAYS, 2, 2)
	gl.StencilOp(gl.REPLACE, gl.REPLACE, gl.REPLACE)
	gl.Color4d(0, 0, 0, 0)
	do_left_wall()

	// Now that the entire wall has been draw we can cast shadows on it if we've
	// got a los texture
	if los_tex != nil {
		los_tex.Bind()
		gl.BlendFunc(gl.SRC_ALPHA_SATURATE, gl.SRC_ALPHA)
		gl.Color4d(0, 0, 0, 1)

		ty := (float64(room.Y+room.Size.Dy) - 0.5) / float64(los_tex.Size())
		tx := (float64(room.X) + 0.5) / float64(los_tex.Size())
		tx2 := (float64(room.X+room.Size.Dx) - 0.5) / float64(los_tex.Size())
		gl.Begin(gl.QUADS)
		gl.TexCoord2d(ty, tx)
		gl.Vertex3i(0, room.Size.Dy, 0)
		gl.TexCoord2d(ty, tx)
		gl.Vertex3i(0, room.Size.Dy, -dz)
		gl.TexCoord2d(ty, tx2)
		gl.Vertex3i(room.Size.Dx, room.Size.Dy, -dz)
		gl.TexCoord2d(ty, tx2)
		gl.Vertex3i(room.Size.Dx, room.Size.Dy, 0)
		gl.End()
		gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	}
}

func drawFloor(room *Room, floor mathgl.Mat4, temp *WallTexture, cstack base.ColorStack, los_tex *LosTexture, los_alpha float64, floor_drawer []FloorDrawer) {
	gl.MatrixMode(gl.MODELVIEW)
	gl.PushMatrix()
	gl.LoadIdentity()
	gl.MultMatrixf(&floor[0])
	defer gl.PopMatrix()

	gl.Enable(gl.STENCIL_TEST)
	defer gl.Disable(gl.STENCIL_TEST)

	gl.StencilFunc(gl.ALWAYS, 4, 4)
	gl.StencilOp(gl.REPLACE, gl.REPLACE, gl.REPLACE)
	gl.Disable(gl.TEXTURE_2D)
	gl.Begin(gl.QUADS)
	gl.Vertex2i(0, 0)
	gl.Vertex2i(0, room.Size.Dy)
	gl.Vertex2i(room.Size.Dx, room.Size.Dy)
	gl.Vertex2i(room.Size.Dx, 0)
	gl.End()
	gl.StencilFunc(gl.EQUAL, 4, 15)
	gl.StencilOp(gl.KEEP, gl.KEEP, gl.KEEP)

	// Draw the floor
	gl.Enable(gl.TEXTURE_2D)
	cstack.ApplyWithAlpha(los_alpha)
	room.Floor.Data().Render(0, 0, float64(room.Size.Dx), float64(room.Size.Dy))

	if los_tex != nil {
		los_tex.Bind()
		gl.BlendFunc(gl.SRC_ALPHA_SATURATE, gl.SRC_ALPHA)
		gl.Color4d(0, 0, 0, 1)
		gl.Begin(gl.QUADS)
		gl.TexCoord2i(0, 0)
		gl.Vertex2i(-room.X, -room.Y)
		gl.TexCoord2i(1, 0)
		gl.Vertex2i(-room.X, los_tex.Size()-room.Y)
		gl.TexCoord2i(1, 1)
		gl.Vertex2i(los_tex.Size()-room.X, los_tex.Size()-room.Y)
		gl.TexCoord2i(0, 1)
		gl.Vertex2i(los_tex.Size()-room.X, -room.Y)
		gl.End()
		gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	}
	cstack.ApplyWithAlpha(los_alpha)
	{
		g_texs = g_texs[0:0]
		if temp != nil {
			g_texs = append(g_texs, *temp)
		}
		for _, tex := range room.WallTextures {
			g_texs = append(g_texs, *tex)
		}
		for i, tex := range g_texs {
			if tex.X >= float32(room.Size.Dx) {
				tex.Rot -= 3.1415926535 / 2
			}
			if temp != nil && i == 0 {
				cstack.Push(1, 0.7, 0.7, 0.7)
			}
			cstack.ApplyWithAlpha(los_alpha)
			tex.Render()
			if temp != nil && i == 0 {
				cstack.Pop()
			}
		}
	}

	gl.PushMatrix()
	gl.Translated(-float64(room.X), -float64(room.Y), 0)
	for _, fd := range floor_drawer {
		fd.RenderOnFloor()
	}
	gl.PopMatrix()

	// Re-enable textures because floor_drawer.RenderOnFloor() might have
	// disabled them
	gl.Enable(gl.TEXTURE_2D)

	gl.StencilFunc(gl.ALWAYS, 5, 5)
	gl.StencilOp(gl.REPLACE, gl.REPLACE, gl.REPLACE)
	gl.Disable(gl.TEXTURE_2D)
	gl.Color4d(0, 0, 0, 0)
	gl.Begin(gl.QUADS)
	gl.Vertex2i(0, 0)
	gl.Vertex2i(0, room.Size.Dy)
	gl.Vertex2i(room.Size.Dx, room.Size.Dy)
	gl.Vertex2i(room.Size.Dx, 0)
	gl.End()
}

func (rv *RoomViewer) drawFloor() {
	gl.MatrixMode(gl.MODELVIEW)
	gl.PushMatrix()
	gl.LoadIdentity()
	gl.MultMatrixf(&rv.mat[0])
	defer gl.PopMatrix()

	gl.Disable(gl.TEXTURE_2D)
	gl.Color4f(1, 0, 1, 0.9)
	if rv.edit_mode == editCells {
		gl.LineWidth(0.02 * rv.zoom)
	} else {
		gl.LineWidth(0.05 * rv.zoom)
	}
	gl.Begin(gl.LINES)
	for i := float32(0); i < float32(rv.room.Size.Dx); i += 1.0 {
		gl.Vertex2f(i, 0)
		gl.Vertex2f(i, float32(rv.room.Size.Dy))
	}
	for j := float32(0); j < float32(rv.room.Size.Dy); j += 1.0 {
		gl.Vertex2f(0, j)
		gl.Vertex2f(float32(rv.room.Size.Dx), j)
	}
	gl.End()

	if rv.edit_mode == editCells {
		gl.Disable(gl.TEXTURE_2D)
		gl.Color4d(1, 0, 0, 1)
		gl.LineWidth(0.05 * rv.zoom)
		gl.Begin(gl.LINES)
		for _, f := range rv.room.Furniture {
			x, y := f.Pos()
			dx, dy := f.Dims()
			gl.Vertex2i(x, y)
			gl.Vertex2i(x, y+dy)

			gl.Vertex2i(x, y+dy)
			gl.Vertex2i(x+dx, y+dy)

			gl.Vertex2i(x+dx, y+dy)
			gl.Vertex2i(x+dx, y)

			gl.Vertex2i(x+dx, y)
			gl.Vertex2i(x, y)
		}
		gl.End()
	}

	gl.Disable(gl.STENCIL_TEST)
}

func drawFurniture(roomx, roomy int, mat mathgl.Mat4, zoom float32, furniture []*Furniture, temp_furniture *Furniture, extras []Drawable, cstack base.ColorStack, los_tex *LosTexture, los_alpha float64) {
	gl.Enable(gl.TEXTURE_2D)
	gl.Color4d(1, 1, 1, los_alpha)
	gl.PushMatrix()
	gl.LoadIdentity()

	board_to_window := func(mx, my float32) (x, y float32) {
		v := mathgl.Vec4{X: mx, Y: my, W: 1}
		v.Transform(&mat)
		x, y = v.X, v.Y
		return
	}

	g_stuff = g_stuff[0:0]
	for i := range furniture {
		g_stuff = append(g_stuff, furniture[i])
	}
	if temp_furniture != nil {
		g_stuff = append(g_stuff, temp_furniture)
	}
	for i := range extras {
		g_stuff = append(g_stuff, extras[i])
	}
	g_stuff = OrderRectObjects(g_stuff)

	for i := len(g_stuff) - 1; i >= 0; i-- {
		f := g_stuff[i]
		var near_x, near_y, dx, dy float32

		idx, idy := f.Dims()
		dx = float32(idx)
		dy = float32(idy)
		switch d := f.(type) {
		case *Furniture:
			ix, iy := d.Pos()
			near_x = float32(ix)
			near_y = float32(iy)

		case Drawable:
			fx, fy := d.FPos()
			near_x = float32(fx)
			near_y = float32(fy)
		}

		vis_tot := 1.0
		if los_tex != nil {
			vis_tot = 0.0

			// If we're looking at a piece of furniture that blocks Los then we
			// can't expect to have Los to all of it, so we will check the squares
			// around it.  Full visibility will mean that half of the surrounding
			// cells are visible.
			blocks_los := false
			// Also need to check if it is an enemy unit
			if _, ok := f.(*Furniture); ok {
				blocks_los = true
			}

			if blocks_los {
				for x := near_x - 1; x < near_x+dx+1; x++ {
					vis_tot += float64(los_tex.Pix()[int(x)+roomx][int(near_y-1)+roomy])
					vis_tot += float64(los_tex.Pix()[int(x)+roomx][int(near_y+dy+1)+roomy])
				}
				for y := near_y; y < near_y+dy; y++ {
					vis_tot += float64(los_tex.Pix()[int(near_x-1)+roomx][int(y)+roomy])
					vis_tot += float64(los_tex.Pix()[int(near_x+dx+1)+roomx][int(y)+roomy])
				}
				vis_tot /= float64((dx*2 + dy*2 + 4) * 255 / 2)
				if vis_tot > 1 {
					vis_tot = 1
				}
			} else {
				for x := near_x; x < near_x+dx; x++ {
					for y := near_y; y < near_y+dy; y++ {
						vis_tot += float64(los_tex.Pix()[int(x)+roomx][int(y)+roomy])
					}
				}
				vis_tot /= float64(dx * dy * 255)
			}
		}

		leftx, _ := board_to_window(near_x, near_y+dy)
		rightx, _ := board_to_window(near_x+dx, near_y)
		_, boty := board_to_window(near_x, near_y)
		if f == temp_furniture {
			cstack.Push(1, 0, 0, 0.4)
		} else {
			bot := (LosMinVisibility / 255.0)
			vis := (vis_tot - bot) / (1 - bot)
			vis = vis * vis
			vis = vis*(1-bot) + bot
			vis = vis * vis
			cstack.Push(vis, vis, vis, 1)
		}
		cstack.ApplyWithAlpha(los_alpha)
		cstack.Pop()
		switch d := f.(type) {
		case *Furniture:
			d.Render(mathgl.Vec2{leftx, boty}, rightx-leftx)

		case Drawable:
			gl.Enable(gl.TEXTURE_2D)
			x := (leftx + rightx) / 2
			d.Render(mathgl.Vec2{x, boty}, rightx-leftx)
		}
	}
	gl.PopMatrix()
}

func (rv *RoomViewer) Draw(region gui.Region) {
	region.PushClipPlanes()
	defer region.PopClipPlanes()

	if rv.Render_region.X != region.X || rv.Render_region.Y != region.Y || rv.Render_region.Dx != region.Dx || rv.Render_region.Dy != region.Dy {
		rv.Render_region = region
		rv.makeMat()
	}

	gl.MatrixMode(gl.MODELVIEW)
	gl.PushMatrix()
	defer gl.PopMatrix()
	gl.LoadIdentity()
	gl.MultMatrixf(&rv.mat[0])

	// rv.room.render(rv.mat, rv.left_wall_mat, rv.right_wall_mat)
	rv.room.setupGlStuff()
	rv.room.far_left.wall_alpha = 255
	rv.room.far_right.wall_alpha = 255
	rv.room.render(rv.mat, rv.left_wall_mat, rv.right_wall_mat, rv.zoom, 255, nil, nil, nil)
	return

	rv.cstack.Push(1, 1, 1, 1)
	defer rv.cstack.Pop()
	drawPrep()
	drawWall(rv.room, rv.mat, rv.left_wall_mat, rv.right_wall_mat, rv.Temp.WallTexture, doorInfo{}, rv.cstack, nil, 1.0)
	drawFloor(rv.room, rv.mat, rv.Temp.WallTexture, rv.cstack, nil, 1.0, nil)
	rv.drawFloor()
	if rv.edit_mode == editCells {
		rv.cstack.Pop()
		rv.cstack.Push(1, 1, 1, 0.1)
	} else {
		rv.cstack.Push(1, 1, 1, 1)
		defer rv.cstack.Pop()
	}
	drawFurniture(0, 0, rv.mat, rv.zoom, rv.room.Furniture, rv.Temp.Furniture, nil, rv.cstack, nil, 1.0)
}

func (rv *RoomViewer) SetEventHandler(handler gin.EventHandler) {
	rv.handler = handler
}

func (rv *RoomViewer) Think(*gui.Gui, int64) {
	if rv.size != rv.room.Size {
		rv.size = rv.room.Size
		rv.makeMat()
	}
	mx, my := rv.WindowToBoard(gin.In().GetCursor("Mouse").Point())
	rv.mx = int(mx)
	rv.my = int(my)
}
