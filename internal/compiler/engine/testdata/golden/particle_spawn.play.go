package golden

type Skin struct{ Note float64 }

type Effect struct {
	Hit float64
}

type Particle struct {
	Explosion float64
}

type Note struct {
	Beat float64 `sonolus:"imported"`
	X    float64 `sonolus:"memory"`
	Q    Quad    `sonolus:"memory"`
}

func (n Note) Initialize() {
	// Quad composite as single arg to particle spawn
	sonolus.EffectClip("Hit").Schedule(n.Beat, 0.1)
	sonolus.ParticleClip("Explosion").Spawn(n.Q, 1, 0)
}
