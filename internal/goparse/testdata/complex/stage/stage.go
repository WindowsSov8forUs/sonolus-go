package stage

type Stage struct {
	Beat float64 `sonolus:"imported"`
}

func (s *Stage) UpdateParallel(dt float64) {
	debugPause()
}
