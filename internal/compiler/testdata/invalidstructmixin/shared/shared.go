package shared

type Leaf struct {
	Value float64 `archetype:"data"`
}

type Left struct{ Leaf }
type Right struct{ Leaf }

type Imported struct {
	Value float64 `archetype:"imported,name=duplicate"`
}

type Oversized struct {
	Values [33]float64 `archetype:"data"`
}

type BadCallback struct{}

func (*BadCallback) Preprocess(float64) {}
