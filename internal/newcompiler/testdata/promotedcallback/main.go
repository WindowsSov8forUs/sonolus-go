package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/testdata/promotedcallback/helper"
	"github.com/WindowsSov8forUs/sonolus-go/sonolus/play"
)

type Note struct {
	play.Archetype `sonolus:"name=Note"`
	helper.Callbacks
}

func main() {}
