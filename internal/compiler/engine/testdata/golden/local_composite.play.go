package golden

type Skin struct{ Note float64 }
type Effect struct{ Hit float64 }
type Particle struct{ Explosion float64 }

type Note struct {
	Beat float64 `sonolus:"imported"`
	X    float64 `sonolus:"memory"`
}

func (n Note) Initialize() {
	// EntityInfo: State field read
	info := sonolus.EntityInfoAt(0)
	if info.IsActive() {
		n.X = info.State
	}

	// Effect: Play method
	clip := sonolus.EffectClip("Hit")
	clip.Play(0.1)

	// Life: nested field chain
	life := sonolus.Life()
	n.X = life.Initial
}
