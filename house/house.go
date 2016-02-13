package house

import (
	"image"
	"math"
	"path/filepath"
	"unsafe"

	"github.com/MobRulesGames/glop/gin"
	"github.com/MobRulesGames/glop/gui"
	"github.com/MobRulesGames/glop/util/algorithm"
	gl "github.com/MobRulesGames/gogl/gl21"
	"github.com/MobRulesGames/haunts/base"
	"github.com/MobRulesGames/haunts/texture"
)

type Room struct {
	Defname string
	*roomDef

	// The placement of doors in this room
	Doors []*Door `registry:"loadfrom-doors"`

	// The offset of this room on this floor
	X, Y int

	temporary, invalid bool

	// whether or not to draw the walls transparent
	far_left struct {
		wall_alpha byte
	}
	far_right struct {
		wall_alpha byte
	}

	// opengl stuff
	// Vertex buffer storing the vertices of the room as well as the texture
	// coordinates for the los texture.
	vbuffer gl.Uint

	// index buffers
	left_buffer  gl.Uint
	right_buffer gl.Uint
	floor_buffer gl.Uint
	floor_count  gl.Int

	// we don't want to redo all of the vertex and index buffers unless we
	// need to, so we keep track of the position and size of the room when they
	// were made so we don't have to.
	gl struct {
		x, y, dx, dy             int
		wall_tex_dx, wall_tex_dy int
	}

	wall_texture_gl_map    map[*WallTexture]wallTextureGlIds
	wall_texture_state_map map[*WallTexture]wallTextureState
}

func (room *Room) Color() (r, g, b, a byte) {
	if room.temporary {
		if room.invalid {
			return 255, 127, 127, 200
		} else {
			return 127, 127, 255, 200
		}
	}
	return 255, 255, 255, 255
}

type WallFacing int

const (
	NearLeft WallFacing = iota
	NearRight
	FarLeft
	FarRight
)

func MakeDoor(name string) *Door {
	d := Door{Defname: name}
	base.GetObject("doors", &d)
	return &d
}

func GetAllDoorNames() []string {
	return base.GetAllNamesInRegistry("doors")
}

func LoadAllDoorsInDir(dir string) {
	base.RemoveRegistry("doors")
	base.RegisterRegistry("doors", make(map[string]*doorDef))
	base.RegisterAllObjectsInDir("doors", dir, ".json", "json")
}

func (d *Door) Load() {
	base.GetObject("doors", d)
}

type doorDef struct {
	// Name of this texture as it appears in the editor, should be unique among
	// all Doors
	Name string

	// Number of cells wide the door is
	Width int

	// If true then this door is always open, cannot be interacted with, and
	// never draws a threshold.
	Always_open bool

	Opened_texture texture.Object
	Closed_texture texture.Object

	Open_sound base.Path
	Shut_sound base.Path
}

type Door struct {
	Defname string
	*doorDef

	// Which wall the door is on
	Facing WallFacing

	// How far along this wall the door is located
	Pos int

	// Whether or not the door is opened - determines what texture to use
	Opened bool

	temporary, invalid bool

	highlight_threshold bool

	// gl stuff for drawing the threshold on the ground
	threshold_glids doorGlIds
	door_glids      doorGlIds
	state           doorState
}

func (d *Door) AlwaysOpen() bool {
	return d.doorDef.Always_open
}

func (d *Door) IsOpened() bool {
	return d.doorDef.Always_open || d.Opened
}

func (d *Door) SetOpened(opened bool) {
	d.Opened = opened
}

func (d *Door) HighlightThreshold(v bool) {
	d.highlight_threshold = v
}

type doorState struct {
	// for tracking whether the buffers are dirty
	facing WallFacing
	pos    int
	room   struct {
		x, y, dx, dy int
	}
}

type doorGlIds struct {
	vbuffer gl.Uint

	floor_buffer gl.Uint
	floor_count  gl.Sizei
}

func (d *Door) setupGlStuff(room *Room) {
	var state doorState
	state.facing = d.Facing
	state.pos = d.Pos
	state.room.x = room.X
	state.room.y = room.Y
	state.room.dx = room.roomDef.Size.Dx
	state.room.dy = room.roomDef.Size.Dy
	if state == d.state {
		return
	}
	if d.TextureData().Dy() == 0 {
		// Can't build this data until the texture is loaded, so we'll have to try
		// again later.
		return
	}
	d.state = state
	if d.threshold_glids.vbuffer != 0 {
		gl.DeleteBuffers(1, &d.threshold_glids.vbuffer)
		gl.DeleteBuffers(1, &d.threshold_glids.floor_buffer)
		d.threshold_glids.vbuffer = 0
		d.threshold_glids.floor_buffer = 0
	}
	if d.door_glids.vbuffer != 0 {
		gl.DeleteBuffers(1, &d.door_glids.vbuffer)
		gl.DeleteBuffers(1, &d.door_glids.floor_buffer)
		d.door_glids.vbuffer = 0
		d.door_glids.floor_buffer = 0
	}

	// far left, near right, do threshold
	// near left, far right, do threshold
	// far left, far right, do door
	var vs []roomVertex
	if d.Facing == FarLeft || d.Facing == NearRight {
		x1 := float32(d.Pos)
		x2 := float32(d.Pos + d.Width)
		var y1 float32 = -0.25
		var y2 float32 = 0.25
		if d.Facing == FarLeft {
			y1 = float32(room.roomDef.Size.Dy)
			y2 = float32(room.roomDef.Size.Dy) - 0.25
		}
		// los_x1 := (x1 + float32(room.X)) / LosTextureSize
		vs = append(vs, roomVertex{x: x1, y: y1})
		vs = append(vs, roomVertex{x: x1, y: y2})
		vs = append(vs, roomVertex{x: x2, y: y2})
		vs = append(vs, roomVertex{x: x2, y: y1})
		for i := 0; i < 4; i++ {
			vs[i].los_u = (y2 + float32(room.Y)) / LosTextureSize
			vs[i].los_v = (vs[i].x + float32(room.X)) / LosTextureSize
		}
	}
	if d.Facing == FarRight || d.Facing == NearLeft {
		y1 := float32(d.Pos)
		y2 := float32(d.Pos + d.Width)
		var x1 float32 = -0.25
		var x2 float32 = 0.25
		if d.Facing == FarRight {
			x1 = float32(room.roomDef.Size.Dx)
			x2 = float32(room.roomDef.Size.Dx) - 0.25
		}
		// los_y1 := (y1 + float32(room.Y)) / LosTextureSize
		vs = append(vs, roomVertex{x: x1, y: y1})
		vs = append(vs, roomVertex{x: x1, y: y2})
		vs = append(vs, roomVertex{x: x2, y: y2})
		vs = append(vs, roomVertex{x: x2, y: y1})
		for i := 0; i < 4; i++ {
			vs[i].los_u = (vs[i].y + float32(room.Y)) / LosTextureSize
			vs[i].los_v = (x2 + float32(room.X)) / LosTextureSize
		}
	}
	dz := -float32(d.Width*d.TextureData().Dy()) / float32(d.TextureData().Dx())
	if d.Facing == FarRight {
		x := float32(room.roomDef.Size.Dx)
		y1 := float32(d.Pos + d.Width)
		y2 := float32(d.Pos)
		los_v := (float32(room.X) + x - 0.5) / LosTextureSize
		los_u1 := (float32(room.Y) + y1) / LosTextureSize
		los_u2 := (float32(room.Y) + y2) / LosTextureSize
		vs = append(vs, roomVertex{
			x: x, y: y1, z: 0,
			u: 0, v: 1,
			los_u: los_u1,
			los_v: los_v,
		})
		vs = append(vs, roomVertex{
			x: x, y: y1, z: dz,
			u: 0, v: 0,
			los_u: los_u1,
			los_v: los_v,
		})
		vs = append(vs, roomVertex{
			x: x, y: y2, z: dz,
			u: 1, v: 0,
			los_u: los_u2,
			los_v: los_v,
		})
		vs = append(vs, roomVertex{
			x: x, y: y2, z: 0,
			u: 1, v: 1,
			los_u: los_u2,
			los_v: los_v,
		})
	}
	if d.Facing == FarLeft {
		x1 := float32(d.Pos)
		x2 := float32(d.Pos + d.Width)
		y := float32(room.roomDef.Size.Dy)
		los_v1 := (float32(room.X) + x1) / LosTextureSize
		los_v2 := (float32(room.X) + x2) / LosTextureSize
		los_u := (float32(room.Y) + y - 0.5) / LosTextureSize
		vs = append(vs, roomVertex{
			x: x1, y: y, z: 0,
			u: 0, v: 1,
			los_u: los_u,
			los_v: los_v1,
		})
		vs = append(vs, roomVertex{
			x: x1, y: y, z: dz,
			u: 0, v: 0,
			los_u: los_u,
			los_v: los_v1,
		})
		vs = append(vs, roomVertex{
			x: x2, y: y, z: dz,
			u: 1, v: 0,
			los_u: los_u,
			los_v: los_v2,
		})
		vs = append(vs, roomVertex{
			x: x2, y: y, z: 0,
			u: 1, v: 1,
			los_u: los_u,
			los_v: los_v2,
		})
	}
	gl.GenBuffers(1, &d.threshold_glids.vbuffer)
	gl.BindBuffer(gl.ARRAY_BUFFER, d.threshold_glids.vbuffer)
	size := int(unsafe.Sizeof(roomVertex{}))
	gl.BufferData(gl.ARRAY_BUFFER, gl.Sizeiptr(size*len(vs)), gl.Pointer(&vs[0].x), gl.STATIC_DRAW)

	is := []uint16{0, 1, 2, 0, 2, 3}
	gl.GenBuffers(1, &d.threshold_glids.floor_buffer)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, d.threshold_glids.floor_buffer)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, gl.Sizeiptr(int(unsafe.Sizeof(is[0]))*len(is)), gl.Pointer(&is[0]), gl.STATIC_DRAW)
	d.threshold_glids.floor_count = 6

	if d.Facing == FarLeft || d.Facing == FarRight {
		is2 := []uint16{4, 5, 6, 4, 6, 7}
		gl.GenBuffers(1, &d.door_glids.floor_buffer)
		gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, d.door_glids.floor_buffer)
		gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, gl.Sizeiptr(int(unsafe.Sizeof(is[0]))*len(is2)), gl.Pointer(&is2[0]), gl.STATIC_DRAW)
		d.door_glids.floor_count = 6
	}
}

func (d *Door) TextureData() *texture.Data {
	if d.IsOpened() {
		return d.Opened_texture.Data()
	}
	return d.Closed_texture.Data()
}

func (d *Door) Color() (r, g, b, a byte) {
	if d.temporary {
		if d.invalid {
			return 255, 127, 127, 200
		} else {
			return 127, 127, 255, 200
		}
	}
	return 255, 255, 255, 255
}

func (r *Room) Pos() (x, y int) {
	return r.X, r.Y
}

type Floor struct {
	Rooms  []*Room `registry:"loadfrom-rooms"`
	Spawns []*SpawnPoint
}

func (f *Floor) canAddRoom(add *Room) bool {
	for _, room := range f.Rooms {
		if room.temporary {
			continue
		}
		if roomOverlap(room, add) {
			return false
		}
	}
	return true
}

func (room *Room) canAddDoor(door *Door) bool {
	if door.Pos < 0 {
		return false
	}

	// Make sure that the door only occupies valid cells
	if door.Facing == FarLeft || door.Facing == NearRight {
		if door.Pos+door.Width >= room.Size.Dx {
			return false
		}
	}
	if door.Facing == FarRight || door.Facing == NearLeft {
		if door.Pos+door.Width >= room.Size.Dy {
			return false
		}
	}

	// Now make sure that the door doesn't overlap any other doors
	for _, other := range room.Doors {
		if other.Facing != door.Facing {
			continue
		}
		if other.temporary {
			continue
		}
		if other.Pos >= door.Pos && other.Pos-door.Pos < door.Width {
			return false
		}
		if door.Pos >= other.Pos && door.Pos-other.Pos < other.Width {
			return false
		}
	}

	return true
}

func (f *Floor) FindMatchingDoor(room *Room, door *Door) (*Room, *Door) {
	for _, other_room := range f.Rooms {
		if other_room == room {
			continue
		}
		for _, other_door := range other_room.Doors {
			if door.Facing == FarLeft && other_door.Facing != NearRight {
				continue
			}
			if door.Facing == FarRight && other_door.Facing != NearLeft {
				continue
			}
			if door.Facing == NearLeft && other_door.Facing != FarRight {
				continue
			}
			if door.Facing == NearRight && other_door.Facing != FarLeft {
				continue
			}
			if door.Facing == FarLeft && other_room.Y != room.Y+room.Size.Dy {
				continue
			}
			if door.Facing == NearRight && room.Y != other_room.Y+other_room.Size.Dy {
				continue
			}
			if door.Facing == FarRight && other_room.X != room.X+room.Size.Dx {
				continue
			}
			if door.Facing == NearLeft && room.X != other_room.X+other_room.Size.Dx {
				continue
			}
			if door.Facing == FarLeft || door.Facing == NearRight {
				if door.Pos == other_door.Pos-(room.X-other_room.X) {
					return other_room, other_door
				}
			}
			if door.Facing == FarRight || door.Facing == NearLeft {
				if door.Pos == other_door.Pos-(room.Y-other_room.Y) {
					return other_room, other_door
				}
			}
		}
	}
	return nil, nil
}

func (f *Floor) findRoomForDoor(target *Room, door *Door) (*Room, *Door) {
	if !target.canAddDoor(door) {
		return nil, nil
	}

	if door.Facing == FarLeft {
		for _, room := range f.Rooms {
			if room.Y == target.Y+target.Size.Dy {
				temp := MakeDoor(door.Defname)
				temp.Pos = door.Pos - (room.X - target.X)
				temp.Facing = NearRight
				if room.canAddDoor(temp) {
					return room, temp
				}
			}
		}
	} else if door.Facing == FarRight {
		for _, room := range f.Rooms {
			if room.X == target.X+target.Size.Dx {
				temp := MakeDoor(door.Defname)
				temp.Pos = door.Pos - (room.Y - target.Y)
				temp.Facing = NearLeft
				if room.canAddDoor(temp) {
					return room, temp
				}
			}
		}
	}
	return nil, nil
}

func (f *Floor) canAddDoor(target *Room, door *Door) bool {
	r, _ := f.findRoomForDoor(target, door)
	return r != nil
}

func (f *Floor) removeInvalidDoors() {
	for _, room := range f.Rooms {
		algorithm.Choose(room.Doors, func(a interface{}) bool {
			_, other_door := f.FindMatchingDoor(room, a.(*Door))
			return other_door != nil && !other_door.temporary
		})
	}
}

func (f *Floor) RoomFurnSpawnAtPos(x, y int) (room *Room, furn *Furniture, spawn *SpawnPoint) {
	for _, croom := range f.Rooms {
		rx, ry := croom.Pos()
		rdx, rdy := croom.Dims()
		if x < rx || y < ry || x >= rx+rdx || y >= ry+rdy {
			continue
		}
		room = croom
		for _, furniture := range room.Furniture {
			tx := x - rx
			ty := y - ry
			fx, fy := furniture.Pos()
			fdx, fdy := furniture.Dims()
			if tx < fx || ty < fy || tx >= fx+fdx || ty >= fy+fdy {
				continue
			}
			furn = furniture
			break
		}
		for _, sp := range f.Spawns {
			if sp.temporary {
				continue
			}
			if x >= sp.X && x < sp.X+sp.Dx && y >= sp.Y && y < sp.Y+sp.Dy {
				spawn = sp
				break
			}
		}
		return
	}
	return
}

func (f *Floor) render(region gui.Region, focusx, focusy, angle, zoom float32, drawables []Drawable, los_tex *LosTexture, floor_drawers []FloorDrawer) {
	var ros []RectObject
	algorithm.Map2(f.Rooms, &ros, func(r *Room) RectObject { return r })
	// Do not include temporary objects in the ordering, since they will likely
	// overlap with other objects and make it difficult to determine the proper
	// ordering.  Just draw the temporary ones last.
	num_temp := 0
	for i := range ros {
		if ros[i].(*Room).temporary {
			ros[num_temp], ros[i] = ros[i], ros[num_temp]
			num_temp++
		}
	}
	placed := OrderRectObjects(ros[num_temp:])
	ros = ros[0:num_temp]
	for i := range placed {
		ros = append(ros, placed[i])
	}
	alpha_map := make(map[*Room]byte)
	los_map := make(map[*Room]byte)

	// First pass over the rooms - this will determine at what alpha the rooms
	// should be draw.  We will use this data later to determine the alpha for
	// the doors of adjacent rooms.
	for i := len(ros) - 1; i >= 0; i-- {
		room := ros[i].(*Room)
		los_alpha := room.getMaxLosAlpha(los_tex)
		room.setupGlStuff()
		tx := (focusx + 3) - float32(room.X+room.Size.Dx)
		if tx < 0 {
			tx = 0
		}
		ty := (focusy + 3) - float32(room.Y+room.Size.Dy)
		if ty < 0 {
			ty = 0
		}
		if tx < ty {
			tx = ty
		}
		// z := math.Log10(float64(zoom))
		z := float64(zoom) / 10
		v := math.Pow(z, float64(2*tx)/3)
		if v > 255 {
			v = 255
		}
		bv := 255 - byte(v)
		alpha_map[room] = byte((int(bv) * int(los_alpha)) >> 8)
		los_map[room] = los_alpha
		// room.render(floor, left, right, , 255)
	}

	// Second pass - this time we fill in the alpha that we should use for the
	// doors, using the values we've already calculated in the first pass.
	for _, r1 := range f.Rooms {
		r1.far_right.wall_alpha = 255
		r1.far_left.wall_alpha = 255
		for _, r2 := range f.Rooms {
			if r1 == r2 {
				continue
			}
			left, right := r2.getNearWallAlpha(los_tex)
			r1_rect := image.Rect(r1.X, r1.Y+r1.Size.Dy, r1.X+r1.Size.Dx, r1.Y+r1.Size.Dy+1)
			r2_rect := image.Rect(r2.X, r2.Y, r2.X+r2.Size.Dx, r2.Y+r2.Size.Dy)
			if r1_rect.Overlaps(r2_rect) {
				// If there is an open door between the two then we'll tone down the
				// alpha, otherwise we won't treat it any differently
				for _, d1 := range r1.Doors {
					for _, d2 := range r2.Doors {
						if d1 == d2 {
							r1.far_left.wall_alpha = byte((int(left) * 200) >> 8)
						}
					}
				}
			}
			r1_rect = image.Rect(r1.X+r1.Size.Dx, r1.Y, r1.X+r1.Size.Dx+1, r1.Y+r1.Size.Dy)
			if r1_rect.Overlaps(r2_rect) {
				for _, d1 := range r1.Doors {
					for _, d2 := range r2.Doors {
						if d1 == d2 {
							r1.far_right.wall_alpha = byte((int(right) * 200) >> 8)
						}
					}
				}
			}
		}
	}

	// Third pass - now that we know what alpha to use on the rooms, walls, and
	// doors we can actually render everything.  We still need to go back to
	// front though.
	for i := len(ros) - 1; i >= 0; i-- {
		room := ros[i].(*Room)
		fx := focusx - float32(room.X)
		fy := focusy - float32(room.Y)
		floor, _, left, _, right, _ := makeRoomMats(room.roomDef, region, fx, fy, angle, zoom)
		v := alpha_map[room]
		if los_map[room] > 5 {
			room.render(floor, left, right, zoom, v, drawables, los_tex, floor_drawers)
		}
	}
}

type HouseDef struct {
	Name string

	Icon texture.Object

	Floors []*Floor
}

func MakeHouseDef() *HouseDef {
	var h HouseDef
	h.Name = "name"
	h.Floors = append(h.Floors, &Floor{})
	return &h
}

// Shifts the rooms in all floors such that the coordinates of all rooms are
// as low on each axis as possible without being zero or negative.
func (h *HouseDef) Normalize() {
	for i := range h.Floors {
		if len(h.Floors[i].Rooms) == 0 {
			continue
		}
		var minx, miny int
		minx, miny = h.Floors[i].Rooms[0].Pos()
		for j := range h.Floors[i].Rooms {
			x, y := h.Floors[i].Rooms[j].Pos()
			if x < minx {
				minx = x
			}
			if y < miny {
				miny = y
			}
		}
		for j := range h.Floors[i].Rooms {
			h.Floors[i].Rooms[j].X -= minx - 1
			h.Floors[i].Rooms[j].Y -= miny - 1
		}
		for _, sp := range h.Floors[0].Spawns {
			sp.X -= minx - 1
			sp.Y -= miny - 1
		}
	}
}

type HouseEditor struct {
	*gui.HorizontalTable
	tab     *gui.TabFrame
	widgets []tabWidget

	house  HouseDef
	viewer *HouseViewer
}

func (he *HouseEditor) GetViewer() Viewer {
	return he.viewer
}

func (w *HouseEditor) SelectTab(n int) {
	if n < 0 || n >= len(w.widgets) {
		return
	}
	if n != w.tab.SelectedTab() {
		w.widgets[w.tab.SelectedTab()].Collapse()
		w.tab.SelectTab(n)
		// w.viewer.SetEditMode(editNothing)
		w.widgets[n].Expand()
	}
}

type houseDataTab struct {
	*gui.VerticalTable

	name       *gui.TextEditLine
	num_floors *gui.ComboBox
	icon       *gui.FileWidget

	house  *HouseDef
	viewer *HouseViewer

	// Distance from the mouse to the center of the object, in board coordinates
	drag_anchor struct{ x, y float32 }

	// Which floor we are viewing and editing
	current_floor int

	temp_room, prev_room *Room

	temp_spawns []*SpawnPoint
}

func makeHouseDataTab(house *HouseDef, viewer *HouseViewer) *houseDataTab {
	var hdt houseDataTab
	hdt.VerticalTable = gui.MakeVerticalTable()
	hdt.house = house
	hdt.viewer = viewer

	hdt.name = gui.MakeTextEditLine("standard", "name", 300, 1, 1, 1, 1)
	num_floors_options := []string{"1 Floor", "2 Floors", "3 Floors", "4 Floors"}
	hdt.num_floors = gui.MakeComboTextBox(num_floors_options, 300)
	if hdt.house.Icon.Path == "" {
		hdt.house.Icon.Path = base.Path(filepath.Join(datadir, "houses", "icons"))
	}
	hdt.icon = gui.MakeFileWidget(string(hdt.house.Icon.Path), imagePathFilter)

	hdt.VerticalTable.AddChild(hdt.name)
	hdt.VerticalTable.AddChild(hdt.num_floors)
	hdt.VerticalTable.AddChild(hdt.icon)

	names := GetAllRoomNames()
	room_buttons := gui.MakeVerticalTable()
	for _, name := range names {
		n := name
		room_buttons.AddChild(gui.MakeButton("standard", name, 300, 1, 1, 1, 1, func(int64) {
			if hdt.temp_room != nil {
				return
			}
			hdt.temp_room = &Room{Defname: n}
			base.GetObject("rooms", hdt.temp_room)
			hdt.temp_room.temporary = true
			hdt.temp_room.invalid = true
			hdt.house.Floors[0].Rooms = append(hdt.house.Floors[0].Rooms, hdt.temp_room)
			hdt.drag_anchor.x = float32(hdt.temp_room.Size.Dx / 2)
			hdt.drag_anchor.y = float32(hdt.temp_room.Size.Dy / 2)
		}))
	}
	scroller := gui.MakeScrollFrame(room_buttons, 300, 700)
	hdt.VerticalTable.AddChild(scroller)
	return &hdt
}
func (hdt *houseDataTab) Think(ui *gui.Gui, t int64) {
	if hdt.temp_room != nil {
		mx, my := gin.In().GetCursor("Mouse").Point()
		bx, by := hdt.viewer.WindowToBoard(mx, my)
		cx, cy := hdt.temp_room.Pos()
		hdt.temp_room.X = int(bx - hdt.drag_anchor.x)
		hdt.temp_room.Y = int(by - hdt.drag_anchor.y)
		dx := hdt.temp_room.X - cx
		dy := hdt.temp_room.Y - cy
		for i := range hdt.temp_spawns {
			hdt.temp_spawns[i].X += dx
			hdt.temp_spawns[i].Y += dy
		}
		hdt.temp_room.invalid = !hdt.house.Floors[0].canAddRoom(hdt.temp_room)
	}
	hdt.VerticalTable.Think(ui, t)
	num_floors := hdt.num_floors.GetComboedIndex() + 1
	if len(hdt.house.Floors) != num_floors {
		for len(hdt.house.Floors) < num_floors {
			hdt.house.Floors = append(hdt.house.Floors, &Floor{})
		}
		if len(hdt.house.Floors) > num_floors {
			hdt.house.Floors = hdt.house.Floors[0:num_floors]
		}
	}
	hdt.house.Name = hdt.name.GetText()
	hdt.house.Icon.Path = base.Path(hdt.icon.GetPath())
}

func (hdt *houseDataTab) onEscape() {
	if hdt.prev_room != nil {
		dx := hdt.prev_room.X - hdt.temp_room.X
		dy := hdt.prev_room.Y - hdt.temp_room.Y
		for i := range hdt.temp_spawns {
			hdt.temp_spawns[i].X += dx
			hdt.temp_spawns[i].Y += dy
		}
		*hdt.temp_room = *hdt.prev_room
		hdt.prev_room = nil
	} else {
		algorithm.Choose(&hdt.house.Floors[0].Rooms, func(r *Room) bool {
			return r != hdt.temp_room
		})
	}
	hdt.temp_room = nil
}

func (hdt *houseDataTab) Respond(ui *gui.Gui, group gui.EventGroup) bool {
	if hdt.VerticalTable.Respond(ui, group) {
		return true
	}

	if found, event := group.FindEvent(gin.Escape); found && event.Type == gin.Press {
		hdt.onEscape()
		return true
	}

	if found, event := group.FindEvent(gin.DeleteOrBackspace); found && event.Type == gin.Press {
		if hdt.temp_room != nil {
			spawns := make(map[*SpawnPoint]bool)
			for i := range hdt.temp_spawns {
				spawns[hdt.temp_spawns[i]] = true
			}
			algorithm.Choose(&hdt.house.Floors[0].Spawns, func(s *SpawnPoint) bool {
				return !spawns[s]
			})
			algorithm.Choose(&hdt.house.Floors[0].Rooms, func(r *Room) bool {
				return r != hdt.temp_room
			})
			hdt.temp_room = nil
			hdt.prev_room = nil
			hdt.viewer.SetBounds()
		}
		return true
	}

	floor := hdt.house.Floors[hdt.current_floor]
	if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
		if hdt.temp_room != nil {
			if !hdt.temp_room.invalid {
				hdt.temp_room.temporary = false
				floor.removeInvalidDoors()
				hdt.temp_room = nil
				hdt.prev_room = nil
				hdt.viewer.SetBounds()
			}
		} else {
			cx, cy := event.Key.Cursor().Point()
			bx, by := hdt.viewer.WindowToBoard(cx, cy)
			for i := range floor.Rooms {
				x, y := floor.Rooms[i].Pos()
				dx, dy := floor.Rooms[i].Dims()
				if int(bx) >= x && int(bx) < x+dx && int(by) >= y && int(by) < y+dy {
					hdt.temp_room = floor.Rooms[i]
					hdt.prev_room = new(Room)
					*hdt.prev_room = *hdt.temp_room
					hdt.temp_room.temporary = true
					hdt.drag_anchor.x = bx - float32(x)
					hdt.drag_anchor.y = by - float32(y)
					break
				}
			}
			if hdt.temp_room != nil {
				hdt.temp_spawns = hdt.temp_spawns[0:0]
				for _, sp := range hdt.house.Floors[0].Spawns {
					x, y := sp.Pos()
					rx, ry := hdt.temp_room.Pos()
					rdx, rdy := hdt.temp_room.Dims()
					if x >= rx && x < rx+rdx && y >= ry && y < ry+rdy {
						hdt.temp_spawns = append(hdt.temp_spawns, sp)
					}
				}
			}
		}
		return true
	}

	return false
}
func (hdt *houseDataTab) Collapse() {}
func (hdt *houseDataTab) Expand()   {}
func (hdt *houseDataTab) Reload() {
	hdt.name.SetText(hdt.house.Name)
	hdt.icon.SetPath(string(hdt.house.Icon.Path))
}

type houseDoorTab struct {
	*gui.VerticalTable

	num_floors *gui.ComboBox

	house  *HouseDef
	viewer *HouseViewer

	// Distance from the mouse to the center of the object, in board coordinates
	drag_anchor struct{ x, y float32 }

	// Which floor we are viewing and editing
	current_floor int

	temp_room, prev_room *Room
	temp_door, prev_door *Door
}

func makeHouseDoorTab(house *HouseDef, viewer *HouseViewer) *houseDoorTab {
	var hdt houseDoorTab
	hdt.VerticalTable = gui.MakeVerticalTable()
	hdt.house = house
	hdt.viewer = viewer

	names := GetAllDoorNames()
	door_buttons := gui.MakeVerticalTable()
	for _, name := range names {
		n := name
		door_buttons.AddChild(gui.MakeButton("standard", name, 300, 1, 1, 1, 1, func(int64) {
			if len(hdt.house.Floors[0].Rooms) < 2 || hdt.temp_door != nil {
				return
			}
			hdt.temp_door = MakeDoor(n)
			hdt.temp_door.temporary = true
			hdt.temp_door.invalid = true
			hdt.temp_room = hdt.house.Floors[0].Rooms[0]
		}))
	}
	scroller := gui.MakeScrollFrame(door_buttons, 300, 700)
	hdt.VerticalTable.AddChild(scroller)
	return &hdt
}
func (hdt *houseDoorTab) Think(ui *gui.Gui, t int64) {
	hdt.VerticalTable.Think(ui, t)
}
func (hdt *houseDoorTab) onEscape() {
	if hdt.temp_door != nil {
		if hdt.temp_room != nil {
			algorithm.Choose(&hdt.temp_room.Doors, func(d *Door) bool {
				return d != hdt.temp_door
			})
		}
		if hdt.prev_door != nil {
			hdt.prev_room.Doors = append(hdt.prev_room.Doors, hdt.prev_door)
			hdt.prev_door.state.pos = -1 // forces it to redo its gl data
			hdt.prev_door = nil
			hdt.prev_room = nil
		}
		hdt.temp_door = nil
		hdt.temp_room = nil
	}
}
func (hdt *houseDoorTab) Respond(ui *gui.Gui, group gui.EventGroup) bool {
	if hdt.VerticalTable.Respond(ui, group) {
		return true
	}

	if found, event := group.FindEvent(gin.Escape); found && event.Type == gin.Press {
		hdt.onEscape()
		return true
	}

	if found, event := group.FindEvent(gin.DeleteOrBackspace); found && event.Type == gin.Press {
		algorithm.Choose(&hdt.temp_room.Doors, func(d *Door) bool {
			return d != hdt.temp_door
		})
		hdt.temp_room = nil
		hdt.temp_door = nil
		hdt.prev_room = nil
		hdt.prev_door = nil
		return true
	}

	cursor := group.Events[0].Key.Cursor()
	var bx, by float32
	if cursor != nil {
		bx, by = hdt.viewer.WindowToBoard(cursor.Point())
	}
	if cursor != nil && hdt.temp_door != nil {
		room := hdt.viewer.FindClosestDoorPos(hdt.temp_door, bx, by)
		if room != hdt.temp_room {
			algorithm.Choose(&hdt.temp_room.Doors, func(d *Door) bool {
				return d != hdt.temp_door
			})
			hdt.temp_room = room
			hdt.temp_door.invalid = (hdt.temp_room == nil)
			hdt.temp_room.Doors = append(hdt.temp_room.Doors, hdt.temp_door)
		}
		if hdt.temp_room == nil {
			hdt.temp_door.invalid = true
		} else {
			other_room, _ := hdt.house.Floors[0].findRoomForDoor(hdt.temp_room, hdt.temp_door)
			hdt.temp_door.invalid = (other_room == nil)
		}
	}

	floor := hdt.house.Floors[hdt.current_floor]
	if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
		if hdt.temp_door != nil {
			other_room, other_door := floor.findRoomForDoor(hdt.temp_room, hdt.temp_door)
			if other_room != nil {
				other_room.Doors = append(other_room.Doors, other_door)
				hdt.temp_door.temporary = false
				hdt.temp_door = nil
				hdt.prev_door = nil
			}
		} else {
			hdt.temp_room, hdt.temp_door = hdt.viewer.FindClosestExistingDoor(bx, by)
			if hdt.temp_door != nil {
				hdt.prev_door = new(Door)
				*hdt.prev_door = *hdt.temp_door
				hdt.prev_room = hdt.temp_room
				hdt.temp_door.temporary = true
				room, door := hdt.house.Floors[0].FindMatchingDoor(hdt.temp_room, hdt.temp_door)
				if room != nil {
					algorithm.Choose(&room.Doors, func(d *Door) bool {
						return d != door
					})
				}
			}
		}
		return true
	}

	return false
}
func (hdt *houseDoorTab) Collapse() {
	hdt.onEscape()
}
func (hdt *houseDoorTab) Expand() {
}
func (hdt *houseDoorTab) Reload() {
	hdt.onEscape()
}

type houseRelicsTab struct {
	*gui.VerticalTable

	spawn_name *gui.TextEditLine
	make_spawn *gui.Button
	typed_name string

	house  *HouseDef
	viewer *HouseViewer

	// Which floor we are viewing and editing
	current_floor int

	temp_relic, prev_relic *SpawnPoint

	drag_anchor struct{ x, y float32 }
}

func (hdt *houseRelicsTab) newSpawn() {
	hdt.temp_relic = new(SpawnPoint)
	hdt.temp_relic.Name = hdt.spawn_name.GetText()
	hdt.temp_relic.X = 10000
	hdt.temp_relic.Dx = 2
	hdt.temp_relic.Dy = 2
	hdt.temp_relic.temporary = true
	hdt.temp_relic.invalid = true
	hdt.house.Floors[0].Spawns = append(hdt.house.Floors[0].Spawns, hdt.temp_relic)
}

func makeHouseRelicsTab(house *HouseDef, viewer *HouseViewer) *houseRelicsTab {
	var hdt houseRelicsTab
	hdt.VerticalTable = gui.MakeVerticalTable()
	hdt.house = house
	hdt.viewer = viewer

	hdt.VerticalTable.AddChild(gui.MakeTextLine("standard", "Spawns", 300, 1, 1, 1, 1))
	hdt.spawn_name = gui.MakeTextEditLine("standard", "", 300, 1, 1, 1, 1)
	hdt.VerticalTable.AddChild(hdt.spawn_name)

	hdt.make_spawn = gui.MakeButton("standard", "New Spawn Point", 300, 1, 1, 1, 1, func(int64) {
		hdt.newSpawn()
	})
	hdt.VerticalTable.AddChild(hdt.make_spawn)

	return &hdt
}

func (hdt *houseRelicsTab) onEscape() {
	if hdt.temp_relic != nil {
		if hdt.prev_relic != nil {
			*hdt.temp_relic = *hdt.prev_relic
			hdt.prev_relic = nil
		} else {
			algorithm.Choose(&hdt.house.Floors[0].Spawns, func(s *SpawnPoint) bool {
				return s != hdt.temp_relic
			})
		}
		hdt.temp_relic = nil
	}
}

func (hdt *houseRelicsTab) markTempSpawnValidity() {
	hdt.temp_relic.invalid = false
	floor := hdt.house.Floors[0]
	var room *Room
	x, y := hdt.temp_relic.Pos()
	for ix := 0; ix < hdt.temp_relic.Dx; ix++ {
		for iy := 0; iy < hdt.temp_relic.Dy; iy++ {
			room_at, furn_at, _ := floor.RoomFurnSpawnAtPos(x+ix, y+iy)
			if room == nil {
				room = room_at
			}
			if room_at == nil || room_at != room || furn_at != nil {
				hdt.temp_relic.invalid = true
				return
			}
		}
	}
}

func (hdt *houseRelicsTab) Think(ui *gui.Gui, t int64) {
	defer hdt.VerticalTable.Think(ui, t)
	rbx, rby := hdt.viewer.WindowToBoard(gin.In().GetCursor("Mouse").Point())
	bx := roundDown(rbx - hdt.drag_anchor.x + 0.5)
	by := roundDown(rby - hdt.drag_anchor.y + 0.5)
	if hdt.temp_relic != nil {
		hdt.temp_relic.X = bx
		hdt.temp_relic.Y = by
		hdt.temp_relic.Dx += gin.In().GetKey(gin.Right).FramePressCount()
		hdt.temp_relic.Dx -= gin.In().GetKey(gin.Left).FramePressCount()
		if hdt.temp_relic.Dx < 1 {
			hdt.temp_relic.Dx = 1
		}
		if hdt.temp_relic.Dx > 10 {
			hdt.temp_relic.Dx = 10
		}
		hdt.temp_relic.Dy += gin.In().GetKey(gin.Up).FramePressCount()
		hdt.temp_relic.Dy -= gin.In().GetKey(gin.Down).FramePressCount()
		if hdt.temp_relic.Dy < 1 {
			hdt.temp_relic.Dy = 1
		}
		if hdt.temp_relic.Dy > 10 {
			hdt.temp_relic.Dy = 10
		}
		hdt.markTempSpawnValidity()
	} else {
		_, _, spawn_at := hdt.house.Floors[0].RoomFurnSpawnAtPos(roundDown(rbx), roundDown(rby))
		if spawn_at != nil {
			hdt.spawn_name.SetText(spawn_at.Name)
		} else if hdt.spawn_name.IsBeingEdited() {
			hdt.typed_name = hdt.spawn_name.GetText()
		} else {
			hdt.spawn_name.SetText(hdt.typed_name)
		}
	}

	if hdt.temp_relic == nil && gin.In().GetKey('n').FramePressCount() > 0 && ui.FocusWidget() == nil {
		hdt.newSpawn()
	}
}

// Rounds a float32 down, instead of towards zero
func roundDown(f float32) int {
	if f >= 0 {
		return int(f)
	}
	return int(f - 1)
}

func (hdt *houseRelicsTab) Respond(ui *gui.Gui, group gui.EventGroup) bool {
	if hdt.VerticalTable.Respond(ui, group) {
		return true
	}

	if found, event := group.FindEvent(gin.Escape); found && event.Type == gin.Press {
		hdt.onEscape()
		return true
	}

	if found, event := group.FindEvent(gin.DeleteOrBackspace); found && event.Type == gin.Press {
		algorithm.Choose(&hdt.house.Floors[0].Spawns, func(s *SpawnPoint) bool {
			return s != hdt.temp_relic
		})
		hdt.temp_relic = nil
		hdt.prev_relic = nil
		return true
	}

	cursor := group.Events[0].Key.Cursor()
	floor := hdt.house.Floors[hdt.current_floor]
	if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
		if hdt.temp_relic != nil {
			if !hdt.temp_relic.invalid {
				hdt.temp_relic.temporary = false
				hdt.temp_relic = nil
			}
		} else {
			for _, sp := range floor.Spawns {
				fbx, fby := hdt.viewer.WindowToBoard(cursor.Point())
				bx, by := roundDown(fbx), roundDown(fby)
				x, y := sp.Pos()
				dx, dy := sp.Dims()
				if bx >= x && bx < x+dx && by >= y && by < y+dy {
					hdt.temp_relic = sp
					hdt.prev_relic = new(SpawnPoint)
					*hdt.prev_relic = *hdt.temp_relic
					hdt.temp_relic.temporary = true
					hdt.drag_anchor.x = fbx - float32(hdt.temp_relic.X)
					hdt.drag_anchor.y = fby - float32(hdt.temp_relic.Y)
					break
				}
			}
		}
	}
	return false
}
func (hdt *houseRelicsTab) Collapse() {
	PopSpawnRegexp()
	hdt.onEscape()
}
func (hdt *houseRelicsTab) Expand() {
	PushSpawnRegexp(".*")
}
func (hdt *houseRelicsTab) Reload() {
	hdt.onEscape()
}

func (h *HouseDef) Save(path string) {
	base.SaveJson(path, h)
}

func LoadAllHousesInDir(dir string) {
	base.RemoveRegistry("houses")
	base.RegisterRegistry("houses", make(map[string]*HouseDef))
	base.RegisterAllObjectsInDir("houses", dir, ".house", "json")
}

func (h *HouseDef) setDoorsOpened(opened bool) {
	for _, floor := range h.Floors {
		for _, room := range floor.Rooms {
			for _, door := range room.Doors {
				door.Opened = opened
			}
		}
	}
}

type iamanidiotcontainer struct {
	Defname string
	*HouseDef
}

func MakeHouseFromName(name string) *HouseDef {
	var idiot iamanidiotcontainer
	idiot.Defname = name
	base.GetObject("houses", &idiot)
	idiot.HouseDef.setDoorsOpened(false)
	return idiot.HouseDef
}

func MakeHouseFromPath(path string) (*HouseDef, error) {
	var house HouseDef
	err := base.LoadAndProcessObject(path, "json", &house)
	if err != nil {
		return nil, err
	}
	house.Normalize()
	house.setDoorsOpened(false)
	return &house, nil
}

func MakeHouseEditorPanel() Editor {
	var he HouseEditor
	he.house = *MakeHouseDef()
	he.HorizontalTable = gui.MakeHorizontalTable()
	he.viewer = MakeHouseViewer(&he.house, 62)
	he.viewer.Edit_mode = true
	he.HorizontalTable.AddChild(he.viewer)

	he.widgets = append(he.widgets, makeHouseDataTab(&he.house, he.viewer))
	he.widgets = append(he.widgets, makeHouseDoorTab(&he.house, he.viewer))
	he.widgets = append(he.widgets, makeHouseRelicsTab(&he.house, he.viewer))
	var tabs []gui.Widget
	for _, w := range he.widgets {
		tabs = append(tabs, w.(gui.Widget))
	}
	he.tab = gui.MakeTabFrame(tabs)
	he.HorizontalTable.AddChild(he.tab)

	return &he
}

// Manually pass all events to the tabs, regardless of location, since the tabs
// need to know where the user clicks.
func (he *HouseEditor) Respond(ui *gui.Gui, group gui.EventGroup) bool {
	he.viewer.Respond(ui, group)
	return he.widgets[he.tab.SelectedTab()].Respond(ui, group)
}

func (he *HouseEditor) Load(path string) error {
	house, err := MakeHouseFromPath(path)
	if err != nil {
		return err
	}
	base.Log().Printf("Loaded %s\n", path)
	house.Normalize()
	he.house = *house
	he.viewer.SetBounds()
	for _, tab := range he.widgets {
		tab.Reload()
	}
	return err
}

func (he *HouseEditor) Save() (string, error) {
	path := filepath.Join(datadir, "houses", he.house.Name+".house")
	err := base.SaveJson(path, he.house)
	return path, err
}

func (he *HouseEditor) Reload() {
	for _, floor := range he.house.Floors {
		for i := range floor.Rooms {
			base.GetObject("rooms", floor.Rooms[i])
		}
	}
	for _, tab := range he.widgets {
		tab.Reload()
	}
}
