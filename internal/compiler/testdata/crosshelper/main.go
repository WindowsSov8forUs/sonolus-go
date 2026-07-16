package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/testdata/crosshelper/helper"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func (*Note) UpdateParallel() { helper.Configure() }

func main() {}
