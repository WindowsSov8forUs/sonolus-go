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
func (base *Base) Add(value float64) { base.Value += value }

type Derived struct {
	Base           `archetype:"base"`
	play.Archetype `archetype:"name=Derived"`
	Extra          float64 `archetype:"shared"`
}

func (derived *Derived) UpdateSequential() { derived.Base.Add(2) }

type GrandDerived struct {
	Derived        `archetype:"base"`
	play.Archetype `archetype:"name=GrandDerived"`
	GrandExtra     float64 `archetype:"shared"`
}

type AbstractNote struct {
	play.Archetype      `archetype:"abstract"`
	play.CallbackOrders `archetype:"preprocess=-7"`
	Beat                float64 `archetype:"imported,name=#BEAT"`
	Time                float64 `archetype:"data"`
}

func (n *AbstractNote) Preprocess() {
	n.Time = play.CurrentEntityRef[AbstractNote]().Index + play.CurrentEntityRef[sonolus.AnyArchetype]().Index + play.Entity.Key()
}

type ConcreteNote struct {
	AbstractNote   `archetype:"base"`
	play.Archetype `archetype:"name=ConcreteNote,key=7"`
}

type Holder struct {
	play.Archetype `archetype:"name=Holder"`
	Target         sonolus.EntityRef[sonolus.AnyArchetype] `archetype:"imported"`
	RuntimeKey     float64                                 `archetype:"shared"`
}

func (holder *Holder) UpdateSequential() {
	holder.RuntimeKey = holder.Target.Key()
	if sonolus.EntityRefMatches[Base](holder.Target, false) {
		sonolus.EntityRefGetAs[Base](holder.Target).Value = 2
	}
	if sonolus.EntityRefMatches[AbstractNote](holder.Target, false) {
		_ = sonolus.EntityRefGetAs[AbstractNote](holder.Target).Beat
	}
}

func main() {}
