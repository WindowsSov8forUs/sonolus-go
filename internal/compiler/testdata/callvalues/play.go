//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/native"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Skipped struct {
	play.Archetype
	Value float64 `archetype:"imported,name=Value"`
}

func (p *Skipped) Preprocess() {
	native.DebugLog(branchParameter(1, p.Value < 0))
}

type Scalars struct {
	play.Archetype
	Value float64 `archetype:"imported,name=Value"`
}

func (p *Scalars) Preprocess() {
	native.DebugLog(float64(multiplyParameter(p.Value)))
	native.DebugLog(float64(countParameter(p.Value)))
	native.DebugLog(float64(conditionalParameter(p.Value)))
	native.DebugLog(float64(conditionalParameter(1)))
	native.DebugLog(float64(countParameter(1)))
	native.DebugLog(float64(conditionalParameter(8)))
	native.DebugLog(float64(conditionalParameter(2 - 1)))
	native.DebugLog(float64(conditionalParameter(One)))
	native.DebugLog(float64(plainAssignment(1)))
	native.DebugLog(float64(incrementParameter(1)))
	native.DebugLog(float64(incrementParameter(int(p.Value))))
	native.DebugLog(float64(branchParameter(1, p.Value > 0)))
	native.DebugLog(float64(branchParameter(1, p.Value < 0)))
	native.DebugLog(float64(localControl(p.Value)))
	native.DebugLog(float64(genericParameter(1)))
	native.DebugLog(float64(genericParameter(1.0)))
	native.DebugLog(float64(closureParameter()))
}
