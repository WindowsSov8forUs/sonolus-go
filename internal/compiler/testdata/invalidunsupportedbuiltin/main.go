package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

type pointerAggregate struct {
	Pointer *int
}

type nestedArrayAggregate struct {
	Values [1]pointerAggregate
}

var packagePointerAggregate = pointerAggregate{}

func (*Note) Preprocess() { _ = complex(1.0, 2.0) }
func (*Note) Initialize() { sonolus.Unreachable("reachable path") }
func (*Note) UpdateSequential() {
	_ = sonolus.Zero[sonolus.Stream[int]]()
}
func (*Note) UpdateParallel() {
	_ = new(sonolus.Stream[int])
	_ = new(pointerAggregate)
	array := [1]pointerAggregate{}
	index := int(play.Time.Now())
	_ = array[index]
	for _, item := range array {
		_ = item
	}
	_ = nestedArrayAggregate{}
	value := pointerAggregate{}
	other := pointerAggregate{}
	alias := &value
	alias = &other
	_ = alias
	_ = packagePointerAggregate
}

func main() {}
