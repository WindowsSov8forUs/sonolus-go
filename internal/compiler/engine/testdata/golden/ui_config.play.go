package golden

type Skin struct{ Note float64 }

type UI struct {
	Menu     RuntimeUiConfig `sonolus:"ui"`
	Judgment RuntimeUiConfig `sonolus:"ui"`
}

var ui = UI{
	Menu:     RuntimeUiConfig{Scale: 1.0, Alpha: 1.0},
	Judgment: RuntimeUiConfig{Scale: 1.2, Alpha: 0.9},
}

type Note struct {
	Beat float64 `sonolus:"imported"`
}
