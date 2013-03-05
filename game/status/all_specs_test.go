package status_test

import (
  "testing"
  "github.com/orfjackal/gospec/src/gospec"
  "../../game/status"
)

func TestAllSpecs(t *testing.T) {
  status.RegisterAllConditions()
  r := gospec.NewRunner()
  r.AddSpec(ConditionsSpec)
  gospec.MainGoTest(r, t)
}
