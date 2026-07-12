package helper

import "github.com/WindowsSov8forUs/sonolus-go/sonolus/play"

type Callbacks struct{}

func (*Callbacks) Preprocess() {
	play.Debug.Log(7)
}
