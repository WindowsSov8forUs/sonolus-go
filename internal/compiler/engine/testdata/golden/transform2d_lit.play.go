package golden

type Skin struct{ Note float64 }

type Note struct {
	Beat float64 `sonolus:"imported"`
	X    float64 `sonolus:"memory"`
}

func (n Note) Preprocess() {
	t := Transform2d{
		A00: 1, A11: 1,
		A22: 1, A33: 1,
		A13: 5,
	}
	sonolus.SetSkinTransform(t)
}
