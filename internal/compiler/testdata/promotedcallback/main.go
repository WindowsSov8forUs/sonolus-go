package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/testdata/promotedcallback/helper"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Note struct {
	play.Archetype `archetype:"name=Note"`
	helper.Callbacks
}

func main() {}
