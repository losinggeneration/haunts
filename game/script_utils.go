package game

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/MobRulesGames/haunts/base"
	"github.com/MobRulesGames/haunts/house"
	lua "github.com/MobRulesGames/golua/lua"
	"io"
	"sort"
)

type luaEncodable int32

const (
	luaEncBool luaEncodable = iota
	luaEncNumber
	luaEncNil
	luaEncTable
	luaEncString
	luaEncEntity
)

// Encodes a lua value, only bool, number, nil, table, and string can be
// encoded.  If an unencodable value is encountered an error will be
// returned.
func LuaEncodeValue(w io.Writer, L *lua.State, index int) error {
	var err1, err2, err3 error
	switch {
	case L.IsBoolean(index):
		err1 = binary.Write(w, binary.LittleEndian, luaEncBool)
		var v byte = 0
		if L.ToBoolean(index) {
			v = 1
		}
		err2 = binary.Write(w, binary.LittleEndian, v)
	case L.IsNumber(index):
		err1 = binary.Write(w, binary.LittleEndian, luaEncNumber)
		err2 = binary.Write(w, binary.LittleEndian, L.ToNumber(index))
	case L.IsNil(index):
		err1 = binary.Write(w, binary.LittleEndian, luaEncNil)
	case LuaIsEntity(L, index):
		err1 = binary.Write(w, binary.LittleEndian, luaEncEntity)
		L.PushString("id")
		L.GetTable(index - 1)
		err2 = binary.Write(w, binary.LittleEndian, uint64(L.ToInteger(-1)))
		L.Pop(1)
	case L.IsTable(index):
		err1 = binary.Write(w, binary.LittleEndian, luaEncTable)
		err2 = LuaEncodeTable(w, L, index)
	case L.IsString(index):
		err1 = binary.Write(w, binary.LittleEndian, luaEncString)
		str := L.ToString(index)
		err2 = binary.Write(w, binary.LittleEndian, uint32(len(str)))
		err3 = binary.Write(w, binary.LittleEndian, []byte(str))
	default:
		return errors.New(fmt.Sprintf("Cannot encode lua type id == %d.", L.Type(index)))
	}
	switch {
	case err1 != nil:
		return err1
	case err2 != nil:
		return err2
	case err3 != nil:
		return err3
	}
	return nil
}

// Decodes a value from the reader and pushes it onto the stack
func LuaDecodeValue(r io.Reader, L *lua.State, g *Game) error {
	var le luaEncodable
	err := binary.Read(r, binary.LittleEndian, &le)
	if err != nil {
		return err
	}
	switch le {
	case luaEncBool:
		var v byte
		err = binary.Read(r, binary.LittleEndian, &v)
		L.PushBoolean(v == 1)
	case luaEncNumber:
		var f float64
		err = binary.Read(r, binary.LittleEndian, &f)
		L.PushNumber(f)
	case luaEncNil:
		L.PushNil()
	case luaEncEntity:
		var id uint64
		err = binary.Read(r, binary.LittleEndian, &id)
		ent := g.EntityById(EntityId(id))
		LuaPushEntity(L, ent)
		if ent != nil {
			base.Log().Printf("LUA: Push Ent %s", ent.Name)
		} else {
			base.Log().Printf("LUA: Push Ent NIL")
		}
	case luaEncTable:
		err = LuaDecodeTable(r, L, g)
	case luaEncString:
		var length uint32
		err = binary.Read(r, binary.LittleEndian, &length)
		if err != nil {
			return err
		}
		sb := make([]byte, length)
		err = binary.Read(r, binary.LittleEndian, &sb)
		L.PushString(string(sb))
	default:
		return errors.New(fmt.Sprintf("Unknown lua value id == %d.", le))
	}
	if err != nil {
		return err
	}
	return nil
}

func LuaEncodeTable(w io.Writer, L *lua.State, index int) error {
	L.PushNil()
	for L.Next(index-1) != 0 {
		binary.Write(w, binary.LittleEndian, byte(1))
		err := LuaEncodeValue(w, L, -2)
		if err != nil {
			return err
		}
		err = LuaEncodeValue(w, L, -1)
		if err != nil {
			return err
		}
		L.Pop(1)
	}
	return binary.Write(w, binary.LittleEndian, byte(0))
}

// decodes a lua table and pushes it onto the stack
func LuaDecodeTable(r io.Reader, L *lua.State, g *Game) error {
	L.NewTable()
	var cont byte
	err := binary.Read(r, binary.LittleEndian, &cont)
	for cont != 0 && err == nil {
		for i := 0; i < 2 && err == nil; i++ {
			err = LuaDecodeValue(r, L, g)
		}
		if err == nil {
			err = binary.Read(r, binary.LittleEndian, &cont)
		}

		L.SetTable(-3)
	}
	if err != nil {
		return err
	}

	return nil
}

func LuaIsEntity(L *lua.State, index int) bool {
	L.PushString("type")
	L.GetTable(index - 1)
	if L.IsNil(-1) {
		L.Pop(1)
		return false
	}
	if L.ToString(-1) == "Entity" {
		L.Pop(1)
		return true
	}
	L.Pop(1)
	return false
}

// Gets the id out of the table at the specified index and returns the
// associated Entity, or nil if there is none.
func LuaToEntity(L *lua.State, game *Game, index int) *Entity {
	L.PushString("id")
	L.GetTable(index - 1)
	id := EntityId(L.ToInteger(-1))
	L.Pop(1)
	return game.EntityById(id)
}

type FunctionTable map[string]func()

// Create a table in Lua with Go functions 
func LuaPushSmartFunctionTable(L *lua.State, tblname string, ft FunctionTable) {
	// Copy it just in case - I can't imagine someone changing it after passing
	// it to this function, but I don't want to take any chances.
	myft := make(FunctionTable)
	for n, f := range ft {
		myft[n] = f
	}

	names := make([]string, len(myft))[0:0]
	for name := range myft {
		names = append(names, name)
	}
	sort.Strings(names)
	valid_selectors := "["
	for i, name := range names {
		if i > 0 {
			valid_selectors += ", "
		}
		valid_selectors += fmt.Sprintf("'%s'", name)
	}
	valid_selectors += "]."
	base.Log().Printf("Functions available: %s\n", valid_selectors)

	L.NewTable()

	// create the metatable
	L.NewTable()
	L.PushString("__index")
	L.PushGoClosure(func(L *lua.State) int {
		name := L.ToString(-1)
		if f, ok := myft[name]; ok {
			f()
		} else {
			base.Error().Printf("'%s' is not a valid selector, valid seletors are %s", name, valid_selectors)
			L.PushNil()
		}
		return 1
	}, 1)
	L.SetTable(-3)

	L.SetMetaTable(-2)
	L.SetGlobal(tblname)
}

// Pushes an entity onto the stack, it is a table containing the following:
// e.id -> EntityId of this entity
// e.name -> Name as displayed to the user
// e.gear_options -> Table mapping gear to icon for all available gear
// e.gear -> Name of the selected gear, nil if none is selected
// e.actions -> Array of actions this entity has available
func LuaPushEntity(L *lua.State, _ent *Entity) {
	if _ent == nil {
		L.PushNil()
		return
	}
	// id and Name can be added to the ent table as static data since they 
	// never change.
	L.NewTable()
	L.PushString("Name")
	L.PushString(_ent.Name)
	L.SetTable(-3)
	L.PushString("id")
	L.PushInteger(int64(_ent.Id))
	L.SetTable(-3)
	L.PushString("type")
	L.PushString("Entity")
	L.SetTable(-3)

/*
	id := _ent.Id

	// Meta table for the Entity so that any dynamic data is generated
	// on-the-fly
	LuaPushSmartFunctionTable(L, FunctionTable{
		"Conditions": func() {
			ent := _ent.Game().EntityById(id)
			L.NewTable()
			for _, condition := range ent.Stats.ConditionNames() {
				L.PushString(condition)
				L.PushBoolean(true)
				L.SetTable(-3)
			}
		},
		"Side": func() {
			ent := _ent.Game().EntityById(id)
			L.NewTable()
			sides := map[string]Side{
				"Denizen":  SideHaunt,
				"Intruder": SideExplorers,
				"Npc":      SideNpc,
				"Object":   SideObject,
			}
			for str, side := range sides {
				L.PushString(str)
				L.PushBoolean(ent.Side() == side)
				L.SetTable(-3)
			}
		},
		"State": func() {
			ent := _ent.Game().EntityById(id)
			L.PushString(ent.Sprite().State())
		},
		"Master": func() {
			ent := _ent.Game().EntityById(id)
			L.NewTable()
			for key, val := range ent.Ai_data {
				L.PushString(key)
				L.PushString(val)
				L.SetTable(-3)
			}
		},
		"GearOptions": func() {
			ent := _ent.Game().EntityById(id)
			L.NewTable()
			if ent.ExplorerEnt != nil {
				for _, gear_name := range ent.ExplorerEnt.Gear_names {
					var g Gear
					g.Defname = gear_name
					base.GetObject("gear", &g)
					L.PushString(gear_name)
					L.PushString(g.Large_icon.Path.String())
					L.SetTable(-3)
				}
			}
		},
		"Gear": func() {
			ent := _ent.Game().EntityById(id)
			if ent.ExplorerEnt != nil && ent.ExplorerEnt.Gear != nil {
				L.PushString(ent.ExplorerEnt.Gear.Name)
			} else {
				L.PushNil()
			}
		},
		"Actions": func() {
			ent := _ent.Game().EntityById(id)
			L.NewTable()
			for _, action := range ent.Actions {
				L.PushString(action.String())
				action.Push(L)
				L.SetTable(-3)
			}
		},
		"Pos": func() {
			ent := _ent.Game().EntityById(id)
			x, y := ent.Pos()
			LuaPushPoint(L, x, y)
		},
		"Corpus": func() {
			ent := _ent.Game().EntityById(id)
			L.PushInteger(ent.Stats.Corpus())
		},
		"Ego": func() {
			ent := _ent.Game().EntityById(id)
			L.PushInteger(ent.Stats.Ego())
		},
		"HpCur": func() {
			ent := _ent.Game().EntityById(id)
			L.PushInteger(ent.Stats.HpCur())
		},
		"HpMax": func() {
			ent := _ent.Game().EntityById(id)
			L.PushInteger(ent.Stats.HpMax())
		},
		"ApCur": func() {
			ent := _ent.Game().EntityById(id)
			L.PushInteger(ent.Stats.ApCur())
		},
		"ApMax": func() {
			ent := _ent.Game().EntityById(id)
			L.PushInteger(ent.Stats.ApMax())
		},
		"Info": func() {
			ent := _ent.Game().EntityById(id)
			L.NewTable()
			L.PushString("LastEntityThatIAttacked")
			LuaPushEntity(L, ent.Game().EntityById(ent.Info.LastEntThatIAttacked))
			L.SetTable(-3)
			L.PushString("LastEntThatAttackedMe")
			LuaPushEntity(L, ent.Game().EntityById(ent.Info.LastEntThatAttackedMe))
			L.SetTable(-3)
		},
	})
	L.SetMetaTable(-2)
*/
}

func LuaPushPoint(L *lua.State, x, y int) {
	L.NewTable()
	L.PushString("X")
	L.PushInteger(int64(x))
	L.SetTable(-3)
	L.PushString("Y")
	L.PushInteger(int64(y))
	L.SetTable(-3)
}

func LuaPushDims(L *lua.State, dx, dy int) {
	L.NewTable()
	L.PushString("Dx")
	L.PushInteger(int64(dx))
	L.SetTable(-3)
	L.PushString("Dy")
	L.PushInteger(int64(dy))
	L.SetTable(-3)
}

func LuaToPoint(L *lua.State, pos int) (x, y int) {
	L.PushString("X")
	L.GetTable(pos - 1)
	x = L.ToInteger(-1)
	L.Pop(1)
	L.PushString("Y")
	L.GetTable(pos - 1)
	y = L.ToInteger(-1)
	L.Pop(1)
	return
}

func LuaPushRoom(L *lua.State, game *Game, room *house.Room) {
	for fi, f := range game.House.Floors {
		for ri, r := range f.Rooms {
			if r == room {
				L.NewTable()
				L.PushString("type")
				L.PushString("room")
				L.SetTable(-3)
				L.PushString("floor")
				L.PushInteger(int64(fi))
				L.SetTable(-3)
				L.PushString("room")
				L.PushInteger(int64(ri))
				L.SetTable(-3)
				L.PushString("Pos")
				LuaPushPoint(L, room.X, room.Y)
				L.SetTable(-3)
				L.PushString("Dims")
				LuaPushDims(L, room.Size.Dx, room.Size.Dy)
				L.SetTable(-3)
				return
			}
		}
	}
	L.PushNil()
}

func LuaToRoom(L *lua.State, game *Game, index int) *house.Room {
	L.PushString("floor")
	L.GetTable(index - 1)
	floor := L.ToInteger(-1)
	L.Pop(1)
	L.PushString("room")
	L.GetTable(index - 1)
	room := L.ToInteger(-1)
	L.Pop(1)

	if floor < 0 || floor >= len(game.House.Floors) {
		return nil
	}
	if room < 0 || room >= len(game.House.Floors[floor].Rooms) {
		return nil
	}

	return game.House.Floors[floor].Rooms[room]
}

func LuaPushDoor(L *lua.State, game *Game, door *house.Door) {
	for fi, f := range game.House.Floors {
		for ri, r := range f.Rooms {
			for di, d := range r.Doors {
				if d == door {
					L.NewTable()
					L.PushString("type")
					L.PushString("door")
					L.SetTable(-3)
					L.PushString("floor")
					L.PushInteger(int64(fi))
					L.SetTable(-3)
					L.PushString("room")
					L.PushInteger(int64(ri))
					L.SetTable(-3)
					L.PushString("door")
					L.PushInteger(int64(di))
					L.SetTable(-3)
					return
				}
			}
		}
	}
	L.PushNil()
}

func LuaToDoor(L *lua.State, game *Game, index int) *house.Door {
	L.PushString("floor")
	L.GetTable(index - 1)
	floor := L.ToInteger(-1)
	L.Pop(1)
	L.PushString("room")
	L.GetTable(index - 1)
	room := L.ToInteger(-1)
	L.Pop(1)
	L.PushString("door")
	L.GetTable(index - 1)
	door := L.ToInteger(-1)
	L.Pop(1)

	if floor < 0 || floor >= len(game.House.Floors) {
		return nil
	}
	if room < 0 || room >= len(game.House.Floors[floor].Rooms) {
		return nil
	}
	if door < 0 || door >= len(game.House.Floors[floor].Rooms[room].Doors) {
		return nil
	}

	return game.House.Floors[floor].Rooms[room].Doors[door]
}

func LuaPushSpawnPoint(L *lua.State, game *Game, sp *house.SpawnPoint) {
	index := -1
	for i, spawn := range game.House.Floors[0].Spawns {
		if spawn == sp {
			index = i
		}
	}
	if index == -1 {
		LuaDoError(L, "Unable to push SpawnPoint, not found in the house.")
		L.NewTable()
		L.PushString("id")
		L.PushInteger(-1)
		L.SetTable(-3)
		L.PushString("type")
		L.PushString("SpawnPoint")
		L.SetTable(-3)
		return
	}
	L.NewTable()
	x, y := sp.Pos()
	dx, dy := sp.Dims()
	L.PushString("id")
	L.PushInteger(int64(index))
	L.SetTable(-3)
	L.PushString("type")
	L.PushString("SpawnPoint")
	L.SetTable(-3)
	L.PushString("Name")
	L.PushString(sp.Name)
	L.SetTable(-3)
	L.PushString("Pos")
	LuaPushPoint(L, x, y)
	L.SetTable(-3)
	L.PushString("Dims")
	LuaPushDims(L, dx, dy)
	L.SetTable(-3)
}

func LuaToSpawnPoint(L *lua.State, game *Game, pos int) *house.SpawnPoint {
	L.PushString("id")
	L.GetTable(pos - 1)
	index := L.ToInteger(-1)
	L.Pop(1)
	if index < 0 || index >= len(game.House.Floors[0].Spawns) {
		return nil
	}
	return game.House.Floors[0].Spawns[index]
}

type LuaType int

const (
	LuaInteger LuaType = iota
	LuaFloat
	LuaBoolean
	LuaString
	LuaEntity
	LuaPoint
	LuaRoom
	LuaDoor
	LuaSpawnPoint
	LuaArray
	LuaTable
	LuaAnything
)

func luaMakeSigniature(name string, params []LuaType) string {
	sig := name + "("
	for i := range params {
		switch params[i] {
		case LuaInteger:
			sig += "integer"
		case LuaFloat:
			sig += "float"
		case LuaBoolean:
			sig += "boolean"
		case LuaString:
			sig += "string"
		case LuaEntity:
			sig += "Entity"
		case LuaPoint:
			sig += "Point"
		case LuaRoom:
			sig += "Room"
		case LuaDoor:
			sig += "Door"
		case LuaSpawnPoint:
			sig += "SpawnPoint"
		case LuaArray:
			sig += "Array"
		case LuaTable:
			sig += "table"
		case LuaAnything:
			sig += "anything"
		default:
			sig += "<unknown type>"
		}
		if i != len(params)-1 {
			sig += ", "
		}
	}
	sig += ")"
	return sig
}

func LuaCheckParamsOk(L *lua.State, name string, params ...LuaType) bool {
	fmt.Sprintf("%s(")
	n := L.GetTop()
	if n != len(params) {
		LuaDoError(L, fmt.Sprintf("Got %d parameters to %s.", n, luaMakeSigniature(name, params)))
		return false
	}
	for i := -n; i < 0; i++ {
		ok := false
		switch params[i+n] {
		case LuaInteger:
			ok = L.IsNumber(i)
		case LuaFloat:
			ok = L.IsNumber(i)
		case LuaBoolean:
			ok = L.IsBoolean(i)
		case LuaString:
			ok = L.IsString(i)
		case LuaEntity:
			if L.IsTable(i) {
				L.PushNil()
				for L.Next(i-1) != 0 {
					if L.ToString(-2) == "type" && L.ToString(-1) == "Entity" {
						ok = true
					}
					L.Pop(1)
				}
			}
		case LuaPoint:
			if L.IsTable(i) {
				var x, y bool
				L.PushNil()
				for L.Next(i-1) != 0 {
					if L.ToString(-2) == "X" {
						x = true
					}
					if L.ToString(-2) == "Y" {
						y = true
					}
					L.Pop(1)
				}
				ok = x && y
			}
		case LuaRoom:
			if L.IsTable(i) {
				var floor, room, door bool
				L.PushNil()
				for L.Next(i-1) != 0 {
					switch L.ToString(-2) {
					case "floor":
						floor = true
					case "room":
						room = true
					case "door":
						door = true
					}
					L.Pop(1)
				}
				ok = floor && room && !door
			}
		case LuaDoor:
			if L.IsTable(i) {
				var floor, room, door bool
				L.PushNil()
				for L.Next(i-1) != 0 {
					switch L.ToString(-2) {
					case "floor":
						floor = true
					case "room":
						room = true
					case "door":
						door = true
					}
					L.Pop(1)
				}
				ok = floor && room && door
			}
		case LuaSpawnPoint:
			if L.IsTable(i) {
				L.PushNil()
				for L.Next(i-1) != 0 {
					if L.ToString(-2) == "type" && L.ToString(-1) == "SpawnPoint" {
						ok = true
					}
					L.Pop(1)
				}
			}
		case LuaArray:
			// Make sure that all of the indices 1..length are there, and no others.
			check := make(map[int]int)
			if L.IsTable(i) {
				L.PushNil()
				for L.Next(i-1) != 0 {
					if L.IsNumber(-2) {
						check[L.ToInteger(-2)]++
					} else {
						break
					}
					L.Pop(1)
				}
			}
			count := 0
			for i := 1; i <= len(check); i++ {
				if _, ok := check[i]; ok {
					count++
				}
			}
			ok = (count == len(check))
		case LuaTable:
			ok = L.IsTable(i)
		case LuaAnything:
			ok = true
		}
		if !ok {
			LuaDoError(L, fmt.Sprintf("Unexpected parameters to %s.", luaMakeSigniature(name, params)))
			return false
		}
	}
	return true
}

func LuaDoError(L *lua.State, err_str string) {
	base.Error().Printf(err_str)
	L.PushString(err_str)
	L.SetExecutionLimit(1)
}

func LuaNumParamsOk(L *lua.State, num_params int, name string) bool {
	n := L.GetTop()
	if n != num_params {
		err_str := fmt.Sprintf("%s expects exactly %d parameters, got %d.", name, num_params, n)
		LuaDoError(L, err_str)
		return false
	}
	return true
}

func LuaStringifyParam(L *lua.State, index int) string {
	if L.IsTable(index) {
		str := "table <not implemented> {"
		return str
		first := true
		L.PushNil()
		for L.Next(index-1) != 0 {
			if !first {
				str += ", "
			}
			first = false
			str += fmt.Sprintf("(%s) -> (%s)", LuaStringifyParam(L, -2), LuaStringifyParam(L, -1))
			L.Pop(1)
		}
		return str + "}"
	}
	if L.IsBoolean(index) {
		if L.ToBoolean(index) {
			return "true"
		}
		return "false"
	}
	return L.ToString(index)
}
