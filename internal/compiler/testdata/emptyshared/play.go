//go:build play

package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus"

type EmptyConfiguration struct{ sonolus.Configuration }

var Config EmptyConfiguration
var ROM = sonolus.ROMValues{}
