package optimize

import (
	"sort"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/ir"
)

// NormalizeSwitch normalizes dense sequential cases: {100,101,102,103} becomes
// {(cond-100)}→{0,1,2,3} by transforming the test expression.
type NormalizeSwitch struct{}

func (NormalizeSwitch) Name() string { return "NormalizeSwitch" }

func (NormalizeSwitch) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	for _, b := range ir.Preorder(entry) {
		cases := map[float64]bool{}
		var hasNil bool
		for _, e := range b.Outgoing {
			if e.Cond == nil {
				hasNil = true
			} else {
				cases[*e.Cond] = true
			}
		}
		if len(cases) <= 2 || !hasNil {
			continue
		}
		offset, stride := normSwitchParams(cases)
		if offset == 0 && stride == 1 {
			continue
		}
		for _, e := range b.Outgoing {
			if e.Cond == nil {
				continue
			}
			v := (*e.Cond - offset) / stride
			e.Cond = &v
		}
		if offset != 0 {
			b.Test = gen.PureInstr(opSubtract, b.Test, ir.Const(offset))
		}
		if stride != 1 {
			b.Test = gen.PureInstr(opDivide, b.Test, ir.Const(stride))
		}
	}
	return entry
}

func normSwitchParams(cases map[float64]bool) (offset, stride float64) {
	// Collect and sort case values.
	vals := make([]float64, 0, len(cases))
	for v := range cases {
		vals = append(vals, v)
	}
	sort.Float64s(vals)
	offset = vals[0]
	stride = vals[1] - offset
	if float64(int(offset)) != offset || float64(int(stride)) != stride {
		return 0, 1
	}
	for i, v := range vals[2:] {
		if v != offset+float64(i+2)*stride {
			return 0, 1
		}
	}
	return offset, stride
}
