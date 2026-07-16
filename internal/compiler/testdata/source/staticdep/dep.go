package staticdep

const ExternalConst = 11

type Pair struct {
	Number int
	Text   string
	hidden int
}

var hiddenStatic = ExternalConst + 1
var ExternalStatic = hiddenStatic
var ExternalPair = Pair{Number: ExternalStatic, Text: "dependency"}

func dynamic() int { return 99 }

var ExternalDynamic = dynamic()
