package main

import "github.com/WindowsSov8forUs/sonolus-go/sonolus/play"

type Note struct {
	play.Archetype `sonolus:"name=Note"`
	A              float64 `sonolus:"memory"`
	B              float64 `sonolus:"memory"`
	Imported       float64 `sonolus:"imported"`
	Data           float64 `sonolus:"data"`
	Shared         float64 `sonolus:"shared"`
}

type counter struct{ Value float64 }

func (c *counter) Add() { c.Value++ }

func (c counter) MutateCopy() { c.Value = 99 }

func (n *Note) mutate(value float64) float64 {
	n.A = value + 1
	return value
}

func (n *Note) index() int {
	n.A = 10
	return 0
}

func (n *Note) rhs() float64 {
	n.B = 20
	return 30
}

func (n *Note) Preprocess() {
	n.Imported = 1
	n.Data = 2
	n.Shared = 3
	values := [1]float64{}
	values[n.index()] = n.rhs()
	c := counter{}
	c.Add()
	c.MutateCopy()
	n.A += c.Value
	n.A, n.B = n.B, n.mutate(n.A)
}

func main() {}
