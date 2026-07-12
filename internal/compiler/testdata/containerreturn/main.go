package main

import (
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus"
	"github.com/WindowsSov8forUs/sonolus-go/v2/sonolus/play"
)

type Note struct {
	play.Archetype `archetype:"name=Note"`
}

func makeValues(value float64) sonolus.VarArray[float64] {
	values := sonolus.NewVarArray[float64](4)
	values.Append(value)
	return values
}

func makeNamedValues(value float64) (values sonolus.VarArray[float64]) {
	values = sonolus.NewVarArray[float64](4)
	values.Append(value)
	return
}

func (*Note) Preprocess() {
	values := makeValues(1)
	values = makeNamedValues(2)
	values = func() sonolus.VarArray[float64] { return values }()
	values.Append(2)
	for _, value := range values {
		play.Debug.Log(value)
	}
}

func main() {}
