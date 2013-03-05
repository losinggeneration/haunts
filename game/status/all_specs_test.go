package status_test

import (
	"github.com/orfjackal/gospec/src/gospec"
	"../../game/status"
	"testing"
)

func TestAllSpecs(t *testing.T) {
	status.RegisterAllConditions()
	r := gospec.NewRunner()
	r.AddSpec(ConditionsSpec)
	gospec.MainGoTest(r, t)
}
