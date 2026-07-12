// Package ir defines the source-independent, typed control-flow IR emitted by
// the compiler frontend.
package ir

import "github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

type SourcePos struct {
	File         string
	Line, Column int
}

type Type struct {
	Name   string
	Slots  int
	Fields []Field
}

type Field struct {
	Name   string
	Offset int
	Type   Type
}

type Value struct {
	Type  Type
	Slots []Expr
}

type Expr interface{ expr() }

type Const struct{ Value float64 }

func (Const) expr() {}

type Load struct{ Place Place }

func (Load) expr() {}

type RuntimeCall struct {
	Function resource.RuntimeFunction
	Args     []Expr
	Result   Type
	Pure     bool
	Pos      SourcePos
}

func (RuntimeCall) expr() {}

type Place interface{ place() }

type LocalPlace struct {
	ID     int
	Name   string
	Offset int
}

func (LocalPlace) place() {}

// SSAPlace is a transient scalar place used only inside optimizer SSA regions.
// It must be eliminated before IR reaches the backend.
type SSAPlace struct {
	ID   int
	Name string
}

func (SSAPlace) place() {}

// IndexedLocalPlace addresses a slot in a local aggregate. Index is evaluated
// at runtime and multiplied by Stride before Offset is applied.
type IndexedLocalPlace struct {
	ID     int
	Name   string
	Index  Expr
	Base   int
	Length int
	Stride int
	Offset int
}

func (IndexedLocalPlace) place() {}

// MemoryPlace retains the semantic storage name until the finalize stage maps
// it to a mode-specific runtime block.
type MemoryPlace struct {
	Storage string
	Index   Expr
	Stride  int
	Offset  int
	Read    bool
	Write   bool
}

func (MemoryPlace) place() {}

type Instruction interface{ instruction() }

type Store struct {
	Place Place
	Value Expr
	Pos   SourcePos
}

func (Store) instruction() {}

type Eval struct{ Value Expr }

func (Eval) instruction() {}

type Terminator interface{ terminator() }

type Jump struct{ Target int }

func (Jump) terminator() {}

type Branch struct {
	Condition   Expr
	True, False int
}

func (Branch) terminator() {}

type SwitchCase struct {
	Value  float64
	Target int
}
type Switch struct {
	Value   Expr
	Cases   []SwitchCase
	Default int
}

func (Switch) terminator() {}

type Return struct{ Value Value }

func (Return) terminator() {}

type Unreachable struct{}

func (Unreachable) terminator() {}

type Block struct {
	ID           int
	Phis         []Phi
	Instructions []Instruction
	Terminator   Terminator
}

type PhiArg struct {
	Predecessor int
	Value       SSAPlace
}

type Phi struct {
	Target SSAPlace
	Local  LocalPlace
	Args   []PhiArg
}

type Function struct {
	Name   string
	Result Type
	Entry  int
	Blocks []*Block
	Locals []Type
	// Allocated is set after locals have been rewritten to one physical
	// Temporary Memory layout. Frontend IR intentionally leaves it false.
	Allocated bool
}
