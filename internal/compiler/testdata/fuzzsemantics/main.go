package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

func sum(values ...float64) float64 {
	result := 0.0
	for _, value := range values {
		result += value
	}
	return result
}

type Note struct {
	play.Archetype `archetype:"name=Note"`
	Value          float64 `archetype:"memory"`
}

func (n *Note) Preprocess() {
	result := sum(1, 2, 3, 4)
	for i := range int(n.Value) {
		result += float64(i)
	}
	if n.Value > 0 {
		result += 1
	} else {
		result -= 1
	}
	n.Value = result
	play.Debug.Log(result)
}

func main() {}
