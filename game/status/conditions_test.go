package status_test

import (
	"bytes"
	"encoding/gob"
	"github.com/MobRulesGames/gospec/src/gospec"
	. "github.com/MobRulesGames/gospec/src/gospec"
	"github.com/MobRulesGames/haunts/base"
	"github.com/MobRulesGames/haunts/game/status"
	"path/filepath"
)

var datadir string

func init() {
	datadir, _ = filepath.Abs("../../data_test")
	base.SetDatadir(datadir)
}

func ConditionsSpec(c gospec.Context) {
	c.Specify("Conditions are loaded properly.", func() {
		basic := status.MakeCondition("Basic Test")
		_, ok := basic.(*status.BasicCondition)
		c.Expect(ok, Equals, true)
		c.Expect(basic.Strength(), Equals, 5)
		c.Expect(basic.Kind(), Equals, status.Fire)
		var b status.Base
		b = basic.ModifyBase(b, status.Unspecified)
		c.Expect(b.Attack, Equals, 3)
	})

	c.Specify("Conditions can be gobbed without loss of type.", func() {
		buf := bytes.NewBuffer(nil)
		enc := gob.NewEncoder(buf)

		var cs []status.Condition
		cs = append(cs, status.MakeCondition("Basic Test"))

		err := enc.Encode(cs)
		c.Assume(err, Equals, nil)

		dec := gob.NewDecoder(buf)
		var cs2 []status.Condition
		err = dec.Decode(&cs2)
		c.Assume(err, Equals, nil)

		_, ok := cs2[0].(*status.BasicCondition)
		c.Expect(ok, Equals, true)
	})

	c.Specify("Conditions stack properly", func() {
		var s status.Inst
		fd := status.MakeCondition("Fire Debuff Attack")
		pd := status.MakeCondition("Poison Debuff Attack")
		pd2 := status.MakeCondition("Poison Debuff Attack 2")
		c.Expect(s.AttackBonusWith(status.Unspecified), Equals, 0)
		s.ApplyCondition(pd)
		c.Expect(s.AttackBonusWith(status.Unspecified), Equals, -1)
		s.ApplyCondition(fd)
		c.Expect(s.AttackBonusWith(status.Unspecified), Equals, -2)
		s.ApplyCondition(fd)
		c.Expect(s.AttackBonusWith(status.Unspecified), Equals, -2)
		s.ApplyCondition(pd)
		c.Expect(s.AttackBonusWith(status.Unspecified), Equals, -2)
		s.ApplyCondition(pd2)
		c.Expect(s.AttackBonusWith(status.Unspecified), Equals, -3)
	})

	c.Specify("Resistances work", func() {
		var s status.Inst
		fr1 := status.MakeCondition("Fire Resistance")
		fr2 := status.MakeCondition("Greater Fire Resistance")
		c.Expect(s.CorpusVs("Fire"), Equals, s.CorpusVs("Unspecified"))
		s.ApplyCondition(fr1)
		c.Expect(s.CorpusVs("Fire"), Equals, s.CorpusVs("Unspecified")+1)
		c.Expect(s.CorpusVs("Panic"), Equals, s.CorpusVs("Unspecified"))
		c.Expect(s.CorpusVs("Brutal"), Equals, s.CorpusVs("Unspecified"))
		s.ApplyCondition(fr2)
		c.Expect(s.CorpusVs("Fire"), Equals, s.CorpusVs("Unspecified")+3)
		c.Expect(s.CorpusVs("Panic"), Equals, s.CorpusVs("Unspecified"))
		c.Expect(s.CorpusVs("Brutal"), Equals, s.CorpusVs("Unspecified"))
	})

	c.Specify("Basic conditions last the appropriate amount of time", func() {
		var s status.Inst
		s.UnmarshalJSON([]byte(`
      {
        "Base": {
          "Hp_max": 100,
          "Ap_max": 10
        },
        "Dynamic": {
          "Hp": 100
        }
      }`))
		pd := status.MakeCondition("Poison Debuff Attack")
		pd2 := status.MakeCondition("Poison Debuff Attack 2")
		pd.Strength()
		pd2.Strength()
		c.Expect(s.HpCur(), Equals, 100)
		s.ApplyCondition(status.MakeCondition("Fire Debuff Attack"))
		c.Expect(s.HpCur(), Equals, 100)
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 99)
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 98)
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 97)
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 97)

		s.ApplyCondition(status.MakeCondition("Fire Debuff Attack"))
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 96)
		s.ApplyCondition(status.MakeCondition("Fire Debuff Attack"))
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 95)
		s.ApplyCondition(status.MakeCondition("Fire Debuff Attack"))
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 94)
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 93)
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 92)
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 92)

		s.ApplyCondition(status.MakeCondition("Fire Debuff Attack"))
		s.ApplyCondition(status.MakeCondition("Poison Debuff Attack"))
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 90)
		s.ApplyCondition(status.MakeCondition("Fire Debuff Attack"))
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 88)
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 86)
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 85)
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 85)

		s.ApplyCondition(status.MakeCondition("Fire Debuff Attack"))
		s.ApplyCondition(status.MakeCondition("Poison Debuff Attack"))
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 83)
		s.ApplyCondition(status.MakeCondition("Poison Debuff Attack 2"))
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 80)
		s.ApplyCondition(status.MakeCondition("Poison Debuff Attack"))
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 77)
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 75)
		s.OnRound()
		c.Expect(s.HpCur(), Equals, 75)
	})
}
