package play

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
)

// Assemble folds compiled callbacks into the play-data skeleton using the shared
// modecompile loop.
func Assemble(data *resource.EnginePlayData, results []*modecompile.Result) error {
	return modecompile.Assemble(&data.Nodes, data.Archetypes, results, SetPlayCallback)
}
