package main

import dep "github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/testdata/source/staticdep"

type CrossType struct {
	dep.Pair
	Local int
}

var CrossConst = dep.ExternalConst + 1
var CrossVar = dep.ExternalStatic + 1
var CrossStruct = dep.ExternalPair
var CrossPointer = &dep.ExternalStatic
var CrossPointed = *CrossPointer
var CrossBad = dep.ExternalDynamic

func dynamic() int { return 1 }

var LocalBad = dynamic()
var LocalGood = 7
