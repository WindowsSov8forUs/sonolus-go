//go:build play

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"

type EmptyConfiguration struct{ sonolus.Configuration }

var Config EmptyConfiguration
var ROM = sonolus.ROMValues{}
