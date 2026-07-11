package golden

type Note struct {
	X float64 `sonolus:"memory"`
}

func (n Note) Render() {
	sonolus.Canvas().Print(PrintOptions{
		Value:   123,
		Format:  0,
		AnchorX: 0.5, AnchorY: 0.5,
		SizeX: 16, SizeY: 16,
		Color: -1, Alpha: 1,
	})
}
