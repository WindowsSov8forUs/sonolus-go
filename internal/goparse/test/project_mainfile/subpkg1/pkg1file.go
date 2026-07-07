package subpkg1

import (
	"mainfile/subpkg2"
	"mainfile/subpkg3"
)

var varA = TypeA{}

func FnB() subpkg2.TypeB {
	varA.Field1 = subpkg3.C
	return subpkg2.TypeB{}
}
