package status_test

import (
	"github.com/MobRulesGames/gospec/src/gospec"
	"github.com/MobRulesGames/haunts/game/status"
	"testing"
)

func TestAllSpecs(t *testing.T) {
	status.RegisterAllConditions()
	r := gospec.NewRunner()
	r.AddSpec(ConditionsSpec)
	gospec.MainGoTest(r, t)
}
