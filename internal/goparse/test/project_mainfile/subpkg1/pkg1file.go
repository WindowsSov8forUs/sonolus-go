package subpkg1

import (
	"project_mainfile/subpkg2"
	"project_mainfile/subpkg3"
)

var varA = TypeA{}

func FnB() subpkg2.TypeB {
	varA.Field1 = subpkg3.C
	return subpkg2.TypeB{}
}
