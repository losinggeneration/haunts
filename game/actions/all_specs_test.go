package actions_test

import (
	"github.com/MobRulesGames/gospec/src/gospec"
	"testing"
)

func TestAllSpecs(t *testing.T) {
	r := gospec.NewRunner()
	r.AddSpec(ActionSpec)
	gospec.MainGoTest(r, t)
}
