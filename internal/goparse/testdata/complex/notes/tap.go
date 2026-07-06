package notegarupa

type TapNote struct {
	Beat float64 `sonolus:"imported"`
}

func (n *TapNote) Initialize() {
	debugPause()
}
