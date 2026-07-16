//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type PlayInputMemory struct {
	sonolus.LevelMemoryResource
	Claimed sonolus.ArraySet[int]
}

var InputMemory = PlayInputMemory{Claimed: sonolus.NewArraySet[int](4)}

type playNoteState struct {
	Index int
	Point sonolus.Vec2
}

type PlayNoteMemory struct {
	sonolus.LevelMemoryResource
	Notes sonolus.VarArray[playNoteState]
}

var NoteMemory = &PlayNoteMemory{Notes: sonolus.NewVarArray[playNoteState](3)}

type nestedCollectionBase struct {
	Values sonolus.VarArray[int]
}

type nestedCollections struct {
	nestedCollectionBase
	Sets   [2]sonolus.ArraySet[int]
	Groups [2]nestedCollectionGroup
}

type nestedCollectionGroup struct {
	Values sonolus.VarArray[int]
}

type PlayNestedMemory struct {
	sonolus.LevelMemoryResource
	Collections nestedCollections
}

var NestedMemory = PlayNestedMemory{Collections: nestedCollections{
	nestedCollectionBase: nestedCollectionBase{Values: sonolus.NewVarArray[int](3)},
	Sets: [2]sonolus.ArraySet[int]{
		sonolus.NewArraySet[int](2),
		sonolus.NewArraySet[int](2),
	},
	Groups: [2]nestedCollectionGroup{
		{Values: sonolus.NewVarArray[int](2)},
		{Values: sonolus.NewVarArray[int](2)},
	},
}}

type PlayComputedData struct {
	sonolus.LevelDataResource
	Scale      float64
	Pair       [2]sonolus.Vec2
	View       sonolus.Transform2D
	Invertible sonolus.InvertibleTransform2D
}

var ComputedData = PlayComputedData{}

type PlayGlobalNote struct {
	play.Archetype `archetype:"name=GlobalNote"`
}

func (*PlayGlobalNote) Preprocess() {
	ComputedData.Scale = 2
	ComputedData.Pair[1] = sonolus.NewVec2(3, 4)
	ComputedData.View = sonolus.IdentityTransform2D().PerspectiveY(-0.5, sonolus.NewVec2(0, 1.35))
	ComputedData.Invertible = sonolus.IdentityInvertibleTransform2D().PerspectiveY(-0.5, sonolus.NewVec2(0, 1.35))
	InputMemory.Claimed.Clear()
	NoteMemory.Notes.Clear()
	NestedMemory.Collections.Values.Clear()
	NestedMemory.Collections.Sets[0].Clear()
	NestedMemory.Collections.Sets[1].Clear()
	NestedMemory.Collections.Groups[0].Values.Clear()
	NestedMemory.Collections.Groups[1].Values.Clear()
}

func (*PlayGlobalNote) UpdateSequential() {
	InputMemory.Claimed.Add(1)
	NoteMemory.Notes.Append(playNoteState{Index: InputMemory.Claimed.Len(), Point: ComputedData.Pair[1]})
	NestedMemory.Collections.Values.Append(InputMemory.Claimed.Len())
	index := int(play.Time.Now()) % len(NestedMemory.Collections.Sets)
	NestedMemory.Collections.Sets[index].Add(InputMemory.Claimed.Len())
	NestedMemory.Collections.Groups[index].Values.Append(InputMemory.Claimed.Len())
}

func (*PlayGlobalNote) UpdateParallel() {
	_ = transformThroughPointer(&ComputedData.Invertible, ComputedData.Pair[1]).X + ComputedData.View.TransformVec(ComputedData.Pair[1]).X + ComputedData.Scale + float64(NoteMemory.Notes.Len())
}

func transformThroughPointer(transform *sonolus.InvertibleTransform2D, point sonolus.Vec2) sonolus.Vec2 {
	return transform.TransformVec(point)
}
