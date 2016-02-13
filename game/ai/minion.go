package ai

import (
	"fmt"

	lua "github.com/MobRulesGames/golua/lua"
	"github.com/MobRulesGames/haunts/game"
)

func (a *Ai) addMinionsContext() {
	a.L.Register("IsActive", isActiveMinion(a))
	a.L.Register("ExecMinion", execMinion(a))
	a.L.Register("SetEntityMasterInfo", setMinionMasterInfo(a))
	a.L.Register("AllMinions", allMinions(a))
}

func isActiveMinion(a *Ai) lua.LuaGoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "IsActive", game.LuaEntity) {
			return 0
		}
		ent := game.LuaToEntity(L, a.game, -1)
		if ent == nil {
			game.LuaDoError(L, "Tried to IsActive on an invalid entity.")
			return 0
		}
		if ent.HauntEnt == nil {
			game.LuaDoError(L, "Tried to IsActive on a non-minion.")
			return 0
		}
		L.PushBoolean(ent.Ai.Active())
		return 1
	}
}

func allMinions(a *Ai) lua.LuaGoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "AllMinions") {
			return 0
		}
		L.NewTable()
		count := 0
		for _, ent := range a.game.Ents {
			if ent.HauntEnt != nil {
				count++
				L.PushInteger(int64(count))
				game.LuaPushEntity(L, ent)
				L.SetTable(-3)
			}
		}
		return 1
	}
}

func setMinionMasterInfo(a *Ai) lua.LuaGoFunction {
	return func(L *lua.State) int {
		if !game.LuaCheckParamsOk(L, "SetEntityMasterInfo", game.LuaEntity, game.LuaString, game.LuaAnything) {
			return 0
		}
		ent := game.LuaToEntity(L, a.game, -3)
		if ent == nil {
			game.LuaDoError(L, "Tried to ExecMinion on an invalid entity.")
			return 0
		}
		if ent.HauntEnt == nil {
			game.LuaDoError(L, "Tried to ExecMinion on a non-minion.")
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

func execMinion(a *Ai) lua.LuaGoFunction {
	return func(L *lua.State) int {
		if !game.LuaNumParamsOk(L, 1, "ExecMinion") {
			return 0
		}
		ent := game.LuaToEntity(L, a.game, -1)
		if ent == nil {
			game.LuaDoError(L, "Tried to ExecMinion on an invalid entity.")
			return 0
		}
		if ent.HauntEnt == nil || ent.HauntEnt.Level != game.LevelMinion {
			game.LuaDoError(L, "Tried to ExecMinion on a non-minion.")
			return 0
		}
		if !ent.Ai.Active() {
			game.LuaDoError(L, fmt.Sprintf("Tried to ExecMinion '%s', who is not active.", ent.Name))
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
