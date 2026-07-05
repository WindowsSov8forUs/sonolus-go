package golden

type Skin struct {
	Note float64
	Hold float64
}

type Note struct {
	Beat float64 `sonolus:"imported"`
	X    float64 `sonolus:"memory"`
}

func (n Note) Initialize() {
	skin := sonolus.Skin()
	n.X = skin.Sprites.Note.Exists() +
		skin.Sprites.Exists(1)
}
