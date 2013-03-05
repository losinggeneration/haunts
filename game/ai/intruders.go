package ai

import (
	"fmt"
	"../../game"
	lua "github.com/xenith-studios/golua"
)

func (a *Ai) addIntrudersContext() {
	a.L.Register("IsActive", isActiveIntruder(a))
	a.L.Register("ExecIntruder", execIntruder(a))
	a.L.Register("SetEntityMasterInfo", setIntruderMasterInfo(a))
	a.L.Register("AllIntruders", allIntruders(a))
}

func isActiveIntruder(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "IsActive", game.LuaEntity) {
			return 0
		}
		ent := game.LuaToEntity(L, a.game, -1)
		if ent == nil {
			game.LuaDoError(L, "Tried to IsActive on an invalid entity.")
			return 0
		}
		if ent.ExplorerEnt == nil {
			game.LuaDoError(L, "Tried to IsActive on a non-intruder.")
			return 0
		}
		L.PushBoolean(ent.Ai.Active())
		return 1
	}
}

func allIntruders(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "AllIntruders") {
			return 0
		}
		L.NewTable()
		count := 0
		for _, ent := range a.game.Ents {
			if ent.ExplorerEnt != nil {
				count++
				L.PushInteger(count)
				game.LuaPushEntity(L, ent)
				L.SetTable(-3)
			}
		}
		return 1
	}
}

func setIntruderMasterInfo(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "SetEntityMasterInfo", game.LuaEntity, game.LuaString, game.LuaAnything) {
			return 0
		}
		ent := game.LuaToEntity(L, a.game, -3)
		if ent == nil {
			game.LuaDoError(L, "Tried to ExecIntruder on an invalid entity.")
			return 0
		}
		if ent.ExplorerEnt == nil {
			game.LuaDoError(L, "Tried to ExecIntruder on a non-intruder.")
			return 0
		}
		if ent.Ai_data == nil {
			ent.Ai_data = make(map[string]string)
		}
		if L.IsNil(-1) {
			delete(ent.Ai_data, L.ToString(-2))
		} else {
			ent.Ai_data[L.ToString(-2)] = L.ToString(-1)
		}
		return 0
	}
}

func execIntruder(a *Ai) lua.GoFunction {
	return func(L *lua.State) int {
		if !game.LuaNumParamsOk(L, 1, "ExecIntruder") {
			return 0
		}
		ent := game.LuaToEntity(L, a.game, -1)
		if ent == nil {
			game.LuaDoError(L, "Tried to ExecIntruder on an invalid entity.")
			return 0
		}
		if ent.ExplorerEnt == nil {
			game.LuaDoError(L, "Tried to ExecIntruder on a non-intruder.")
			return 0
		}
		if !ent.Ai.Active() {
			game.LuaDoError(L, fmt.Sprintf("Tried to ExecIntruder '%s', who is not active.", ent.Name))
			return 0
		}
		exec := <-ent.Ai.ActionExecs()
		if exec != nil {
			a.execs <- exec
		}
		<-a.pause
		return 0
	}
}
