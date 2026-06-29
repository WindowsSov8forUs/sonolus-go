package preview

import (
	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/modecompile"
)

// Assemble folds compiled callbacks into the preview-data skeleton.
func Assemble(data *resource.EnginePreviewData, results []*modecompile.Result) error {
	return modecompile.Assemble(&data.Nodes, data.Archetypes, results, setPreviewCallback)
}
