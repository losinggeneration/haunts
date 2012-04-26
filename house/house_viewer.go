package house

import (
  "math"
  "github.com/runningwild/glop/gui"
  "github.com/runningwild/glop/util/algorithm"
  "github.com/runningwild/haunts/base"
  "github.com/runningwild/mathgl"
)


// This structure is used for temporary doors (that are being dragged around in
// the editor) since the normal Door struct only handles one door out of a pair
// and doesn't know what rooms it connects.
type doorInfo struct {
  Door  *Door
  Valid bool
}

type HouseViewer struct {
  gui.Childless
  gui.BasicZone
  gui.NonFocuser

  house *HouseDef

  zoom,angle,fx,fy float32
  floor,ifloor mathgl.Mat4

  // target[xy] are the values that f[xy] approach, this gives us a nice way
  // to change what the camera is looking at
  targetx, targety float32
  target_on bool

  // Need to keep track of time so we can measure time between thinks
  last_timestamp int64


  drawables    []Drawable
  Los_tex      *LosTexture
  Floor_drawer FloorDrawer

  Temp struct {
    Room *Room

    // If we have a temporary door then this is the room it is attached to
    Door_room *Room
    Door_info doorInfo

    Spawn *SpawnPoint
  }

  // Keeping a variety of slices here so that we don't keep allocating new
  // ones every time we render everything
  rooms           []RectObject
  temp_drawables  []Drawable
  all_furn        []*Furniture
  spawns          []*SpawnPoint
  floor_drawers   []FloorDrawer
}

func MakeHouseViewer(house *HouseDef, angle float32) *HouseViewer {
  var hv HouseViewer
  hv.Request_dims.Dx = 100
  hv.Request_dims.Dy = 100
  hv.Ex = true
  hv.Ey = true
  hv.house = house
  hv.angle = angle
  hv.Zoom(1)
  return &hv
}

func (hv *HouseViewer) Respond(g *gui.Gui, group gui.EventGroup) bool {
  return false
}

func (hv *HouseViewer) Think(g *gui.Gui, t int64) {
  dt := t - hv.last_timestamp
  if hv.last_timestamp == 0 {
    dt = 0
  }
  hv.last_timestamp = t
  if hv.target_on {
    f := mathgl.Vec2{hv.fx, hv.fy}
    v := mathgl.Vec2{hv.targetx, hv.targety}
    v.Subtract(&f)
    scale := 1 - float32(math.Pow(0.005, float64(dt)/1000))
    v.Scale(scale)
    f.Add(&v)
    hv.fx = f.X
    hv.fy = f.Y
  }
}

func (hv *HouseViewer) AddDrawable(d Drawable) {
  hv.drawables = append(hv.drawables, d)
}
func (hv *HouseViewer) RemoveDrawable(d Drawable) {
  algorithm.Choose2(&hv.drawables, func(t Drawable) bool {
    return t != d
  })
}

func (hv *HouseViewer) modelviewToBoard(mx, my float32) (x,y,dist float32) {
  mz := d2p(hv.floor, mathgl.Vec3{mx, my, 0}, mathgl.Vec3{0,0,1})
  v := mathgl.Vec4{X: mx, Y: my, Z: mz, W: 1}
  v.Transform(&hv.ifloor)
  return v.X, v.Y, mz
}

func (hv *HouseViewer) boardToModelview(mx, my float32) (x, y, z float32) {
  v := mathgl.Vec4{X: mx, Y: my, W: 1}
  v.Transform(&hv.floor)
  x, y, z = v.X, v.Y, v.Z
  return
}

func (hv *HouseViewer) WindowToBoard(wx, wy int) (float32, float32) {
  hv.floor, hv.ifloor, _, _, _, _ = makeRoomMats(&roomDef{}, hv.Render_region, hv.fx, hv.fy, hv.angle, hv.zoom)

  fx,fy,_ := hv.modelviewToBoard(float32(wx), float32(wy))
  return fx, fy
}

func (hv *HouseViewer) BoardToWindow(bx, by float32) (int, int) {
  hv.floor, hv.ifloor, _, _, _, _ = makeRoomMats(&roomDef{}, hv.Render_region, hv.fx, hv.fy, hv.angle, hv.zoom)

  fx,fy,_ := hv.boardToModelview(bx, by)
  return int(fx), int(fy)
}

// Changes the current zoom from e^(zoom) to e^(zoom+dz)
func (hv *HouseViewer) Zoom(dz float64) {
  if dz == 0 {
    return
  }
  exp := math.Log(float64(hv.zoom)) + dz
  exp = float64(clamp(float32(exp), 2.5, 5.0))
  hv.zoom = float32(math.Exp(exp))
}

func (hv *HouseViewer) Drag(dx, dy float64) {
  hv.floor, hv.ifloor, _, _, _, _ = makeRoomMats(&roomDef{}, hv.Render_region, hv.fx, hv.fy, hv.angle, hv.zoom)

  v := mathgl.Vec4{X: hv.fx, Y: hv.fy, W: 1}
  v.Transform(&hv.floor)
  v.X += float32(dx)
  v.Y += float32(dy)

  v.Z = d2p(hv.floor, mathgl.Vec3{v.X, v.Y, 0}, mathgl.Vec3{0,0,1})
  v.Transform(&hv.ifloor)
  hv.fx, hv.fy = v.X, v.Y

  hv.target_on = false
}

func (hv *HouseViewer) Focus(bx, by float64) {
  hv.targetx = float32(bx)
  hv.targety = float32(by)
  hv.target_on = true
}

func (hv *HouseViewer) String() string {
  return "house viewer"
}

func roomOverlapOnce(a,b *Room) bool {
  x1in := a.X + a.Size.Dx > b.X && a.X + a.Size.Dx <= b.X + b.Size.Dx
  x2in := b.X + b.Size.Dx > a.X && b.X + b.Size.Dx <= a.X + a.Size.Dx
  y1in := a.Y + a.Size.Dy > b.Y && a.Y + a.Size.Dy <= b.Y + b.Size.Dy
  y2in := b.Y + b.Size.Dy > a.Y && b.Y + b.Size.Dy <= a.Y + a.Size.Dy
  return (x1in || x2in) && (y1in || y2in)
}

func roomOverlap(a,b *Room) bool {
  return roomOverlapOnce(a, b) || roomOverlapOnce(b, a)
}

func (hv *HouseViewer) FindClosestDoorPos(ddef *doorDef, bx,by float32) (*Room, Door) {
  current_floor := 0
  best := 1.0e9  // If this is unsafe then the house is larger than earth
  var best_room *Room
  var best_door Door
  best_door.Defname = ddef.Name
  best_door.doorDef = ddef

  clamp_int := func(n, min, max int) int {
    if n < min { return min }
    if n > max { return max }
    return n
  }
  for _,room := range hv.house.Floors[current_floor].Rooms {
    fl := math.Abs(float64(by) - float64(room.Y + room.Size.Dy))
    fr := math.Abs(float64(bx) - float64(room.X + room.Size.Dx))
    if bx < float32(room.X) {
      fl += float64(room.X) - float64(bx)
    }
    if bx > float32(room.X + room.Size.Dx) {
      fl += float64(bx) - float64(room.X + room.Size.Dx)
    }
    if by < float32(room.Y) {
      fr += float64(room.Y) - float64(by)
    }
    if by > float32(room.Y + room.Size.Dy) {
      fr += float64(by) - float64(room.Y + room.Size.Dy)
    }
    if best <= fl && best <= fr { continue }
    best_room = room
    switch {
      case fl < fr:
        best = fl
        best_door.Facing = FarLeft
        best_door.Pos = clamp_int(int(bx - float32(room.X) - float32(ddef.Width) / 2), 0, room.Size.Dx - ddef.Width)

//      case fr < fl:  this case must be true, so we just call it default here
      default:
        best = fr
        best_door.Facing = FarRight
        best_door.Pos = clamp_int(int(by - float32(room.Y) - float32(ddef.Width) / 2), 0, room.Size.Dy - ddef.Width)
    }
  }
  return best_room, best_door
}

func (hv *HouseViewer) FindClosestExistingDoor(bx,by float32) (*Room, *Door) {
  current_floor := 0
  for _,room := range hv.house.Floors[current_floor].Rooms {
    for _,door := range room.Doors {
      if door.Facing != FarLeft && door.Facing != FarRight { continue }
      var vx,vy float32
      if door.Facing == FarLeft {
        vx = float32(room.X + door.Pos) + float32(door.Width) / 2
        vy = float32(room.Y + room.Size.Dy)
      } else {
        // door.Facing == FarRight
        vx = float32(room.X + room.Size.Dx)
        vy = float32(room.Y + door.Pos) + float32(door.Width) / 2
      }
      dsq := (vx - bx) * (vx - bx) + (vy - by) * (vy - by)
      if dsq <= float32(door.Width * door.Width) {
        return room, door
      }
    }
  }
  return nil, nil
}

type offsetDrawable struct {
  Drawable
  dx,dy int
}
func (o offsetDrawable) FPos() (float64, float64) {
  x,y := o.Drawable.FPos()
  return x + float64(o.dx), y + float64(o.dy)
}
func (o offsetDrawable) Pos() (int, int) {
  x,y := o.Drawable.Pos()
  return x + o.dx, y + o.dy
}

func (hv *HouseViewer) Draw(region gui.Region) {
  region.PushClipPlanes()
  defer region.PopClipPlanes()
  hv.Render_region = region

  current_floor := 0

  hv.rooms = hv.rooms[0:0]
  for _,room := range hv.house.Floors[current_floor].Rooms {
    hv.rooms = append(hv.rooms, room)
  }
  if hv.Temp.Room != nil {
    hv.rooms = append(hv.rooms, hv.Temp.Room)
  }
  hv.rooms = OrderRectObjects(hv.rooms)

  drawPrep()
  for i := len(hv.rooms) - 1; i >= 0; i-- {
    room := hv.rooms[i].(*Room)
    // TODO: Would be better to be able to just get the floor mats alone
    m_floor,_,m_left,_,m_right,_ := makeRoomMats(room.roomDef, region, hv.fx - float32(room.X), hv.fy - float32(room.Y), hv.angle, hv.zoom)

    var cstack base.ColorStack
    if room == hv.Temp.Room {
      valid := true
      for _,room := range hv.house.Floors[current_floor].Rooms {
        if roomOverlap(room, hv.Temp.Room) {
          valid = false
          break
        }
      }
      if valid {
        cstack.Push(0.5, 0.5, 1, 0.75)
      } else {
        cstack.Push(1.0, 0.2, 0.2, 0.9)
      }
    } else {
      cstack.Push(1, 1, 1, 1)
    }
    los_alpha := 1.0
    if hv.Los_tex != nil {
      max := hv.Los_tex.Pix()[room.X][room.Y]
      for x := room.X; x < room.X + room.Size.Dx; x++ {
        for y := room.Y; y < room.Y + room.Size.Dy; y++ {
          v := hv.Los_tex.Pix()[x][y]
          if v > max {
            max = v
          }
        }
      }
      if max < LosVisibilityThreshold {
        los_alpha = float64(max - LosMinVisibility) / float64(LosVisibilityThreshold - LosMinVisibility)
      }
    }
    if los_alpha == 0 { continue }
    if room == hv.Temp.Door_room && hv.Temp.Door_info.Door != nil {
      hv.Temp.Door_info.Valid = hv.house.Floors[current_floor].canAddDoor(room, hv.Temp.Door_info.Door)
      drawWall(room, m_floor, m_left, m_right, nil, hv.Temp.Door_info, cstack, hv.Los_tex, los_alpha)
    } else {
      drawWall(room, m_floor, m_left, m_right, nil, doorInfo{}, cstack, hv.Los_tex, los_alpha)
    }
    hv.temp_drawables = hv.temp_drawables[0:0]
    rx,ry := room.Pos()
    rdx,rdy := room.Dims()
    for _,d := range hv.drawables {
      x,y := d.Pos()
      if x >= rx && y >= ry && x < rx + rdx && y < ry + rdy {
        hv.temp_drawables = append(hv.temp_drawables, offsetDrawable{ Drawable:d, dx: -rx, dy: -ry})
      }
    }
    hv.all_furn = hv.all_furn[0:0]
    hv.spawns = hv.spawns[0:0]
    for _, furn := range room.Furniture {
      hv.all_furn = append(hv.all_furn, furn)
    }
    spawn_points := hv.house.Floors[current_floor].Spawns
    for _, spawn := range spawn_points {
      x, y := spawn.Pos()
      x -= rx
      y -= ry
      if x < 0 || y < 0 || x >= rdx || y >= rdy {
        continue
      }
      hv.temp_drawables = append(hv.temp_drawables, spawn)
      hv.spawns = append(hv.spawns, spawn)
    }
    if hv.Temp.Spawn != nil {
      hv.temp_drawables = append(hv.temp_drawables, hv.Temp.Spawn)
      hv.spawns = append(hv.spawns, hv.Temp.Spawn)
    }
    hv.floor_drawers = hv.floor_drawers[0:0]
    if hv.Floor_drawer != nil {
      hv.floor_drawers = append(hv.floor_drawers, hv.Floor_drawer)
    }
    for _, sp := range hv.spawns {
      hv.floor_drawers = append(hv.floor_drawers, sp)
    }
    drawFloor(room, m_floor, nil, cstack, hv.Los_tex, los_alpha, hv.floor_drawers)
    for i := range hv.spawns {
      hv.spawns[i].X -= rx
      hv.spawns[i].Y -= ry
    }
    drawFurniture(rx, ry, m_floor, hv.zoom, hv.all_furn, nil, hv.temp_drawables, cstack, hv.Los_tex, los_alpha)
    for i := range hv.spawns {
      hv.spawns[i].X += rx
      hv.spawns[i].Y += ry
    }
    // drawWall(room *roomDef, wall *texture.Data, left, right mathgl.Mat4, temp *WallTexture)
  }
}














