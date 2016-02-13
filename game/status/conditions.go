package status

import (
	"encoding/gob"
	"path/filepath"

	"github.com/MobRulesGames/haunts/base"
)

// Conditions represent instantaneous or ongoing Conditions on an entity.
// Every round the Condition can
type Condition interface {
	// Returns the name of this condition as it should be displayed to the user.
	Name() string

	// Returns this condition's Kind
	Kind() Kind

	// Returns the strength of this condition relative to other conditions of
	// the same Kind.  This is used to determine which conditions will displace
	// others.
	Strength() int

	ModifyDamage(Damage) Damage

	// Called any time a Base stat is queried
	ModifyBase(Base, Kind) Base

	// Called at the beginning of each round.  May return a damage object to
	// deal damage, and must return a bool indicating whether this effect has
	// completed or not.
	OnRound() (dmg *Damage, complete bool)
}

var condition_registerers []func()
var condition_makers map[string]func() Condition

func RegisterAllConditions() {
	condition_makers = make(map[string]func() Condition)
	for _, f := range condition_registerers {
		f()
	}
}

func MakeCondition(name string) Condition {
	maker, ok := condition_makers[name]
	if !ok {
		base.Error().Printf("Unable to find a Condition named '%s'", name)
		return condition_makers["Error"]()
	}
	return maker()
}

func registerBasicConditions() {
	registry_name := "conditions-basic_conditions"
	base.RemoveRegistry(registry_name)
	base.RegisterRegistry(registry_name, make(map[string]*BasicConditionDef))
	base.RegisterAllObjectsInDir(registry_name, filepath.Join(base.GetDataDir(), "conditions", "basic_conditions"), ".json", "json")
	names := base.GetAllNamesInRegistry(registry_name)
	for _, name := range names {
		cname := name
		f := func() Condition {
			c := BasicCondition{Defname: cname}
			base.GetObject(registry_name, &c)
			return &c
		}
		condition_makers[name] = f
	}
}

func init() {
	condition_registerers = append(condition_registerers, registerBasicConditions)
	gob.Register(&BasicCondition{})
}

type BasicCondition struct {
	Defname string
	*BasicConditionDef
	Time int
}

type BasicConditionDef struct {
	Name string

	// On OnRound() this Condition will create a Damage object with this Dynamic
	// object.
	Dynamic Dynamic

	// This Condition will modify its target unit by adding every value in Base
	// to the unit's Base stats
	Base Base

	// Use Type here instead of Kind so it doesn't overlap with the required
	// method name Kind.  Also Type will be used in the json files so it should
	// be no less obvious what it is.
	Kind Kind

	// The strength of this condition
	Strength int

	// This Condition will OnRound() exactly Duration + 1 times.
	// If Duration < 0 then it will OnRound() forever.
	Duration int

	// Resistances["Foo"] = -4 would mean that when attacked with an attack of
	// kind "Foo" that -4 is added to the this entity's defensive value, either
	// corpus or ego, as appropriate.  In this case it would be a penalty, but
	// if it were a positive value it would be a resistance.
	Resistances map[string]int
}

func (bc *BasicCondition) Name() string {
	return bc.BasicConditionDef.Name
}

func (bc *BasicCondition) Strength() int {
	return bc.BasicConditionDef.Strength
}

func (bc *BasicCondition) Kind() Kind {
	return bc.BasicConditionDef.Kind
}

func (bc *BasicConditionDef) ModifyDamage(dmg Damage) Damage {
	return dmg
}

func (bc *BasicConditionDef) ModifyBase(base Base, kind Kind) Base {
	base.Ap_max += bc.Base.Ap_max
	base.Hp_max += bc.Base.Hp_max
	base.Sight += bc.Base.Sight
	base.Attack += bc.Base.Attack
	base.Corpus += bc.Base.Corpus
	base.Ego += bc.Base.Ego
	if val, ok := bc.Resistances[string(kind)]; ok {
		base.Corpus += val
		base.Ego += val
	}
	return base
}

func (bc *BasicCondition) OnRound() (dmg *Damage, complete bool) {
	var d Dynamic
	if bc.Dynamic != d {
		dmg = &Damage{Dynamic: bc.Dynamic, Kind: bc.Kind()}
	}
	bc.Time++
	complete = (bc.Time == bc.Duration)
	return
}
