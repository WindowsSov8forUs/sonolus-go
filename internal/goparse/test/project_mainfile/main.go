package main

import (
	"project_mainfile/subpkg1"
	"project_mainfile/subpkg2"
)

func mainFn(args ...any) any { return args }

func main() {
	a := subpkg1.TypeA{}
	mainFn(a)
	_ = subpkg2.TypeB{}
}
