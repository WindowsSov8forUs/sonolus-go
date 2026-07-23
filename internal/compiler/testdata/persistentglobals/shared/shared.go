package shared

type Unit struct {
	Value float64
}

type Pair struct {
	Unit *Unit
}

var Root = &Pair{Unit: &Unit{Value: 9}}
