package actions_test

import (
	"testing"

	"github.com/MobRulesGames/gospec/src/gospec"
)

func TestAllSpecs(t *testing.T) {
	r := gospec.NewRunner()
	r.AddSpec(ActionSpec)
	gospec.MainGoTest(r, t)
}
