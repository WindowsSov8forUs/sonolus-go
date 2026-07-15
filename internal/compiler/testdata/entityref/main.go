package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Target struct {
	play.Archetype `archetype:"name=Target"`
	Imported       float64                   `archetype:"imported"`
	Next           sonolus.EntityRef[Target] `archetype:"data"`
	Data           float64                   `archetype:"data"`
	Samples        [2]float64                `archetype:"data"`
	Shared         float64                   `archetype:"shared"`
	Values         sonolus.VarArray[float64] `archetype:"shared,cap=4"`
}

func (target *Target) AddShared(value float64) {
	target.Shared += value
}

func updateTarget(target *Target, replacement sonolus.EntityRef[Target], choose bool, value float64) float64 {
	if choose {
		target = replacement.Get()
	}
	target.AddShared(value)
	target.Values.Append(target.Shared)
	return target.Values.Get(0)
}

func tracedReference(reference sonolus.EntityRef[Target]) sonolus.EntityRef[Target] {
	play.Debug.Log(reference.Index)
	return reference
}

func returnedView(reference sonolus.EntityRef[Target]) *Target {
	return reference.Get()
}

type Reader struct {
	play.Archetype `archetype:"name=Reader,hasInput=true"`
	First          sonolus.EntityRef[Target] `archetype:"imported"`
	Second         sonolus.EntityRef[Target] `archetype:"imported"`
	Selector       float64                   `archetype:"imported"`
}

type TargetHolder struct {
	Target *Target
	Weight float64
}

func (reader *Reader) Preprocess() {
	_ = reader.First.GetUnchecked().Imported
	anyRef := sonolus.EntityRefAs[sonolus.AnyArchetype](reader.First)
	_ = sonolus.EntityRefMatches[Target](anyRef, true)
	_ = sonolus.EntityRefGetAs[Target](anyRef).Imported
	target := tracedReference(reader.First).Get()
	alias := target
	returned := returnedView(reader.First)
	if returned == target {
		returned.Shared += 1
	}
	views := [2]*Target{reader.First.Get(), reader.Second.Get()}
	holder := TargetHolder{Target: reader.First.Get(), Weight: 2}
	if reader.Selector > 0 {
		holder.Target = reader.Second.Get()
	}
	holder.Target.Shared += holder.Weight
	viewIndex := 0
	if reader.Selector > 0 {
		viewIndex = 1
	}
	views[viewIndex].Shared += 1
	viewList := sonolus.NewVarArray[*Target](2)
	viewList.Append(reader.First.Get())
	viewList.Append(reader.Second.Get())
	for view := range viewList.Values() {
		view.Shared += 1
	}
	if reader.Selector > 0 {
		target = reader.Second.Get()
	} else {
		target = alias
	}
	switch {
	case reader.Selector < -1:
		target = reader.First.Get()
	case reader.Selector > 1:
		target = reader.Second.Get()
	}
	for target.Next.Index > 0 {
		target = target.Next.Get()
	}
	target.Data = target.Imported
	for _, sample := range target.Samples {
		target.Shared += sample
	}
	target.Shared = updateTarget(target, reader.First, reader.Selector != 0, target.Data)
}

func (reader *Reader) UpdateSequential() {
	target := reader.First.Get()
	if reader.Selector != 0 {
		target = reader.Second.Get()
	}
	target.Shared = reader.Selector
}

func (reader *Reader) Touch() {
	target := reader.First.Get()
	updateTarget(target, reader.Second, reader.Selector > 0, reader.Selector)
}

func main() {}
