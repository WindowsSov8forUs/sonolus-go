package mode

type Mode string

const (
	ModePlay     Mode = "play"
	ModeWatch    Mode = "watch"
	ModePreview  Mode = "preview"
	ModeTutorial Mode = "tutorial"
)

func (m Mode) Valid() bool {
	switch m {
	case ModePlay, ModeWatch, ModePreview, ModeTutorial:
		return true
	default:
		return false
	}
}
