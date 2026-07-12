package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type Note struct {
	play.Archetype      `sonolus:"name=Note"`
	play.CallbackOrders `sonolus:"preprocess=-10"`
	Value               float64 `sonolus:"memory"`
}

func (*Note) Preprocess() {}

func main() {}
