package golden

type Skin struct{ Note float64 }

type Note struct {
	Beat   float64                 `sonolus:"imported"`
	Result sonolus.PlayEntityInput `sonolus:"input"`
}

func (n Note) Touch() {
	n.Result.Judgment = 1
	n.Result.BucketIndex = 0
}
