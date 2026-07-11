package source

import (
	"golang.org/x/tools/go/packages"

	sourcetracer "github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/source/tracer"
)

type ASTTracer = sourcetracer.ASTTracer
type TypeSpecNode = sourcetracer.TypeSpecNode
type TypeSpecTree = sourcetracer.TypeSpecTree

type StaticKind = sourcetracer.StaticKind
type StaticPathKind = sourcetracer.StaticPathKind
type StaticPathStep = sourcetracer.StaticPathStep
type StaticAddress = sourcetracer.StaticAddress
type StaticSlice = sourcetracer.StaticSlice
type StaticMapEntry = sourcetracer.StaticMapEntry
type StaticMap = sourcetracer.StaticMap
type StaticField = sourcetracer.StaticField
type StaticValue = sourcetracer.StaticValue
type StaticObject = sourcetracer.StaticObject
type StaticBinding = sourcetracer.StaticBinding

const (
	StaticInvalid    = sourcetracer.StaticInvalid
	StaticConstant   = sourcetracer.StaticConstant
	StaticNil        = sourcetracer.StaticNil
	StaticArray      = sourcetracer.StaticArray
	StaticStruct     = sourcetracer.StaticStruct
	StaticSliceValue = sourcetracer.StaticSliceValue
	StaticMapValue   = sourcetracer.StaticMapValue
	StaticPointer    = sourcetracer.StaticPointer
	StaticInterface  = sourcetracer.StaticInterface

	StaticPathField   = sourcetracer.StaticPathField
	StaticPathElement = sourcetracer.StaticPathElement
)

func NewASTTracer(pkg *packages.Package) *ASTTracer {
	return sourcetracer.NewASTTracer(pkg)
}
