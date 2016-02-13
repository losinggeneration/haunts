package status_test

import (
	"testing"

	"github.com/MobRulesGames/gospec/src/gospec"
	"github.com/MobRulesGames/haunts/game/status"
)

func TestAllSpecs(t *testing.T) {
	status.RegisterAllConditions()
	r := gospec.NewRunner()
	r.AddSpec(ConditionsSpec)
	gospec.MainGoTest(r, t)
}
