package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Target struct {
	play.Archetype `archetype:"name=Target"`
	Imported       float64 `archetype:"imported"`
	Shared         float64 `archetype:"shared"`
	Memory         float64 `archetype:"memory"`
	Exported       float64 `archetype:"exported"`
}

type NotArchetype struct {
	Value float64
}

type Holder struct {
	Target *Target
}

type Reader struct {
	play.Archetype `archetype:"name=Reader"`
	Target         sonolus.EntityRef[Target]       `archetype:"imported"`
	Other          sonolus.EntityRef[NotArchetype] `archetype:"imported"`
}

func (r *Reader) Preprocess() {
	_ = r.Target.Get().Memory
	_ = r.Target.Get().Exported
	_ = r.Other.Get().Value
	_ = [1]*Target{r.Target.Get()}
	_ = any(Holder{Target: r.Target.Get()})
	views := sonolus.NewVarArray[*Target](2)
	views.Append(r.Target.Get())
	_ = any(r.Target.Get())
	_ = &r.Target.Get().Imported
	_ = *r.Target.Get()
}

func (r *Reader) UpdateParallel() {
	target := r.Target.Get()
	target.Imported = 1
	target.Shared = 1
}
