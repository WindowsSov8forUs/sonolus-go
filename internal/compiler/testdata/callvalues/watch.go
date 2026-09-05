//go:build watch

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/native"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/watch"
)

type Parameter struct {
	watch.Archetype
	Value float64 `archetype:"imported,name=Value"`
}

func (p *Parameter) Preprocess() {
	native.DebugLog(positionParameter(p.Value))
}

type Return struct {
	watch.Archetype
	Value float64 `archetype:"imported,name=Value"`
}

func (p *Return) Preprocess() {
	native.DebugLog(positionReturn(p.Value))
}

type Direct struct {
	watch.Archetype
	Value float64 `archetype:"imported,name=Value"`
}

func (p *Direct) Preprocess() {
	native.DebugLog(positionDirect(p.Value))
}

type Skipped struct {
	watch.Archetype
	Value float64 `archetype:"imported,name=Value"`
}

func (p *Skipped) Preprocess() {
	native.DebugLog(branchParameter(1, p.Value < 0))
}

type Scalars struct {
	watch.Archetype
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

type Combined20 struct {
	watch.Archetype
	Value float64 `archetype:"imported,name=Value"`
}

func (p *Combined20) Preprocess() {
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
	native.DebugLog(float64(arrayParameter(p.Value)))
	native.DebugLog(float64(arrayRangeParameter(p.Value)))
	native.DebugLog(float64(arrayReturn(p.Value)))
	native.DebugLog(float64(structArrayParameter(p.Value)))
	native.DebugLog(float64(structArrayReturn(p.Value)))
	native.DebugLog(float64(positionReturn(p.Value)))
}

type Combined23 struct {
	watch.Archetype
	Value float64 `archetype:"imported,name=Value"`
}

func (p *Combined23) Preprocess() {
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
	native.DebugLog(float64(arrayParameter(p.Value)))
	native.DebugLog(float64(arrayRangeParameter(p.Value)))
	native.DebugLog(float64(arrayReturn(p.Value)))
	native.DebugLog(float64(structArrayParameter(p.Value)))
	native.DebugLog(float64(structArrayReturn(p.Value)))
	native.DebugLog(float64(positionReturn(p.Value)))
	native.DebugLog(float64(positionDirect(p.Value)))
	native.DebugLog(float64(positionParameter(p.Value)))
	native.DebugLog(float64(arrayDynamicReturn(p.Value)))
}

type Elements struct {
	watch.Archetype
	Value float64 `archetype:"imported,name=Value"`
}

func (p *Elements) Preprocess() {
	a := makePositions(p.Value)
	for i := 0; i < 24; i++ {
		native.DebugLog(a[i].X)
		native.DebugLog(a[i].Y)
	}
	logPositions(a)
}

func logPositions(a [24]Vec2) {
	for i := 0; i < 24; i++ {
		native.DebugLog(a[i].X)
		native.DebugLog(a[i].Y)
	}
}
