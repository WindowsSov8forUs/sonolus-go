package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type Note struct {
	play.Archetype `archetype:"name=Note,hasInput=true"`
}

// Entity Life and Entity Score are writable only in preprocess.
func (*Note) UpdateSequential() {
	play.Entity.SetLife(play.LifeValues{Miss: -1})
	play.Entity.SetScoreMultiplierAt(0, 2)
}

func main() {}
