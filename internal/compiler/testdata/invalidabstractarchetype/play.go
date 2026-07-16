//go:build play

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type InvalidAbstract struct {
	play.Archetype `archetype:"abstract,name=Invalid,hasInput=true,key=1"`
}

type InvalidKey struct {
	play.Archetype `archetype:"name=InvalidKey,key=NaN"`
}
