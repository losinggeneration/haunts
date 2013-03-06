package game

import (
	"github.com/MobRulesGames/haunts/base"
	"github.com/MobRulesGames/haunts/texture"
)

type gearDef struct {
	Name      string
	Condition string
	Action    string

	// 200x200 - displayed when choosing gear
	Large_icon texture.Object

	// 35x35 - displayed when playing the game
	Small_icon texture.Object
}

type Gear struct {
	Defname string
	*gearDef
}

func LoadAllGearInDir(dir string) {
	base.RemoveRegistry("gear")
	base.RegisterRegistry("gear", make(map[string]*gearDef))
	base.RegisterAllObjectsInDir("gear", dir, ".json", "json")
}

func MakeGear(name string) *Gear {
	g := Gear{Defname: name}
	base.GetObject("gear", &g)
	return &g
}

func GetAllGearNames() []string {
	return base.GetAllNamesInRegistry("gear")
}
