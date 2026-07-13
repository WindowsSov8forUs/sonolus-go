//go:build play

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

type ConfigurationReader struct {
	play.Archetype `archetype:"name=ConfigurationReader"`
}

func (*ConfigurationReader) SpawnOrder() float64 {
	if Config.Enabled {
		return Config.Speed + float64(Config.Lane)
	}
	return 0
}
