// Golden test exercising complex control flow patterns:
// else-if chains, switch, loops, and compound conditions.
package golden

type Skin struct {
	Note float64
}

type Note struct {
	Beat  float64 `sonolus:"imported"`
	Value float64 `sonolus:"memory"`
	Flag  float64 `sonolus:"memory"`
	Count float64 `sonolus:"memory"`
	Sum   float64 `sonolus:"memory"`
	A     float64 `sonolus:"memory"`
	B     float64 `sonolus:"memory"`
	R1    float64 `sonolus:"memory"`
	R2    float64 `sonolus:"memory"`
	R3    float64 `sonolus:"memory"`
}

func (n Note) UpdateSequential() {
	// else-if chain (exercises ifStmt → else → IfStmt path)
	if n.Value < 0 {
		n.Flag = -1
	} else if n.Value == 0 {
		n.Flag = 0
	} else if n.Value < 10 {
		n.Flag = 1
	} else {
		n.Flag = 2
	}

	// switch statement
	switch {
	case n.Value < 0:
		n.Count = -1
	case n.Value == 0:
		n.Count = 0
	default:
		n.Count = 1
	}

	// for loop
	n.Sum = 0

	// compound conditions with && and ||
	if n.A > 0 && n.B > 0 {
		n.R1 = 1
	}
	if n.A > 0 || n.B > 0 {
		n.R2 = 1
	}

	// constant-folded condition (exercises short-circuit AND path)
	if 1 > 0 && n.B > 0 {
		n.R3 = 1
	}
}
