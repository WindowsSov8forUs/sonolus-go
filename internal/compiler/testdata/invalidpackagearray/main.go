package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

var values = [2]float64{1, 2}

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func (*Note) Preprocess() {
	values[int(play.Time.Now())%len(values)] = 3
}

func main() {}
