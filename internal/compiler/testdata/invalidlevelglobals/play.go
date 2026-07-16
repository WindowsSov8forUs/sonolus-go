//go:build play

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type MissingContainerInitializer struct {
	sonolus.LevelMemoryResource
	Values sonolus.VarArray[float64]
}

var MissingContainer = MissingContainerInitializer{}

type NonZeroInitializer struct {
	sonolus.LevelDataResource
	Value float64
}

var NonZero = NonZeroInitializer{Value: 1}

type MultipleSingletons struct {
	sonolus.LevelMemoryResource
	Value float64
}

var MultipleA = MultipleSingletons{}
var MultipleB = MultipleSingletons{}

type PromotedBase struct{ sonolus.LevelMemoryResource }
type PromotedMarker struct {
	PromotedBase
	Value float64
}

var Promoted = PromotedMarker{}

type MismatchedNestedArray struct {
	sonolus.LevelMemoryResource
	Values [2]sonolus.VarArray[int]
}

var MismatchedNested = MismatchedNestedArray{Values: [2]sonolus.VarArray[int]{
	sonolus.NewVarArray[int](2),
	sonolus.NewVarArray[int](3),
}}
