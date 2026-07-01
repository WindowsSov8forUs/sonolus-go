// Golden test exercising Skin, Effect, and Particle resource declarations.
package golden

type Skin struct {
	Note   float64
	Hold   float64
	Flick  float64
	Sprite float64
}

type Effect struct {
	Hit    float64
	Miss   float64
	Perfect float64
}

type Particle struct {
	Explosion float64
	Trail     float64
}

type Buckets struct {
	NoteBucket float64 `sonolus:"bucket"`
}

type Note struct {
	Beat float64 `sonolus:"imported"`
}
