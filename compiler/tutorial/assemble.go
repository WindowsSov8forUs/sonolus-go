package tutorial

import "github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

// Assemble assigns compiled tutorial callback indices to the tutorial-data
// skeleton. Tutorial has no archetypes — its three global callbacks
// (Preprocess, Navigate, Update) are set directly on the data struct.
func Assemble(data *resource.EngineTutorialData, pp, nav, upd int) {
	data.Preprocess = pp
	data.Navigate = nav
	data.Update = upd
}
