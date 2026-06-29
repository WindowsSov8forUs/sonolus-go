package ir

import "fmt"

// Place is an addressable memory location: a BlockPlace or an SSAPlace. Port of
// sonolus.py place.py.
type Place interface {
	Node
	placeNode()
}

// BlockPlace addresses block[index + offset]. Block and Index are themselves IR
// nodes (typically Const for concrete block ids / indices, but may be
// expressions). Mirrors sonolus.py BlockPlace.
type BlockPlace struct {
	Block  Node
	Index  Node // nil is treated as Const(0)
	Offset int
}

func (BlockPlace) irNode()    {}
func (BlockPlace) placeNode() {}

// TempBlock is a virtual scratch block backing a local variable (sonolus.py
// TempBlock). It is identified by pointer: each local gets one TempBlock that is
// shared by all its accesses. TempBlock-backed places must be resolved to a
// concrete memory block by allocateTempBlocks before finalization.
type TempBlock struct {
	Name string
	Size int
}

func (*TempBlock) irNode() {}

// NewTemp creates a size-1 scratch block named name.
func NewTemp(name string) *TempBlock { return &TempBlock{Name: name, Size: 1} }

// TempCell returns the place for a size-1 temp block.
func TempCell(t *TempBlock) BlockPlace {
	return BlockPlace{Block: t, Index: Const(0), Offset: 0}
}

// SSAPlace is an SSA value (sonolus.py SSAPlace). It must be removed by register
// allocation before finalization.
type SSAPlace struct {
	Name string
	Num  int
}

func (SSAPlace) irNode()    {}
func (SSAPlace) placeNode() {}

func (p SSAPlace) String() string { return fmt.Sprintf("%s.%d", p.Name, p.Num) }

// NewBlockPlace builds a BlockPlace addressing block[index + offset].
func NewBlockPlace(block, index Node, offset int) BlockPlace {
	return BlockPlace{Block: block, Index: index, Offset: offset}
}

// Cell is a convenience for a fixed block[index] location with constant ids.
func Cell(block, index int) BlockPlace {
	return BlockPlace{Block: Const(block), Index: Const(index)}
}
