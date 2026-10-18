//go:build play

package main

import "github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"

func ifFirst(t float64, k [][2]float64) float64 {
	if t <= k[0][0] {
		return k[0][1]
	}
	return 0
}

func forFirst(t float64, k [][2]float64) float64 {
	i := 0
	for k[i][0] < t {
		i++
	}
	return float64(i)
}

func switchFirst(t float64, k [][2]float64) float64 {
	switch {
	case k[0][0] > t:
		return 1
	}
	return 0
}

type Note struct {
	play.Archetype `archetype:"name=Note"`
	Value          float64 `archetype:"memory"`
}

func (n *Note) UpdateParallel() {
	t := play.Time.Now()
	n.Value = ifFirst(t, nil) + forFirst(t, nil) + switchFirst(t, nil)
}
