package helper

import (
	"github.com/WindowsSov8forUs/sonolus-go/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

func Configure() { play.UI.SetMenu(sonolus.RuntimeUILayout{}) }
