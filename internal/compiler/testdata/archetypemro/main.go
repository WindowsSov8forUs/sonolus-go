//go:build play

package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Base struct {
	play.Archetype      `archetype:"name=Base,hasInput=true"`
	play.CallbackOrders `archetype:"updateSequential=-4"`
	Value               float64 `archetype:"shared"`
}

func (base *Base) UpdateSequential() { base.Value++ }

type Derived struct {
	Base           `archetype:"base"`
	play.Archetype `archetype:"name=Derived"`
	Extra          float64 `archetype:"shared"`
}

type GrandDerived struct {
	Derived        `archetype:"base"`
	play.Archetype `archetype:"name=GrandDerived"`
	GrandExtra     float64 `archetype:"shared"`
}

type Holder struct {
	play.Archetype `archetype:"name=Holder"`
	Target         sonolus.EntityRef[sonolus.AnyArchetype] `archetype:"imported"`
}

func (holder *Holder) UpdateSequential() {
	if sonolus.EntityRefMatches[Base](holder.Target, false) {
		sonolus.EntityRefGetAs[Base](holder.Target).Value = 2
	}
}

func main() {}
