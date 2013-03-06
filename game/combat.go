package game

import (
	"github.com/MobRulesGames/haunts/game/status"
)

func (g *Game) DoAttack(attacker, defender *Entity, strength int, kind status.Kind) bool {
	// get attacker's bonus for using the specified kind of attack
	// get defender's bonus for defending against the specified kind of attack
	// get the defender's current ego/corpus
	// successful attack = strength + attack bonus + 1d10 >= defense bonus + ego/corpus
	attack := attacker.Stats.AttackBonusWith(kind)
	defense := defender.Stats.DefenseVs(kind)
	roll := int(g.Rand.Int63()%10) + 1
	return strength+attack+roll >= defense
}
