package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus/play"

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func helper() {}

func (*Note) Preprocess() {
	if play.Time.Now() > 0 {
		fn := helper
		fn()
	}
}

func main() {}
