package optimize

// Level selects an optimisation preset.
type Level int

const (
	LevelMinimal  Level = iota + 1 // only essential cleanup, no SSA
	LevelFast                      // single SSA round, no LICM/CSE
	LevelStandard                  // full pipeline (~40 passes)
)
