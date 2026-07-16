package ir

import (
	"errors"
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

type Builder struct {
	function  *Function
	current   *Block
	nextBlock int
	errs      []error
}

func NewBuilder(name string, result Type) *Builder {
	return &Builder{function: &Function{Name: name, Result: result}}
}

func (b *Builder) Function() *Function { return b.function }
func (b *Builder) Current() *Block     { return b.current }

func (b *Builder) NewBlock() *Block {
	block := &Block{ID: b.nextBlock}
	b.nextBlock++
	b.function.Blocks = append(b.function.Blocks, block)
	return block
}

func (b *Builder) SetCurrent(block *Block) error {
	if !b.ownsBlock(block) {
		return b.fail("cannot select a block outside this function")
	}
	b.current = block
	return nil
}

func (b *Builder) SetEntry(block *Block) error {
	if !b.ownsBlock(block) {
		return b.fail("entry block does not belong to this function")
	}
	b.function.Entry = block.ID
	return nil
}

func (b *Builder) NewLocal(name string, typ Type) Value {
	if err := validateType(typ); err != nil {
		b.fail("invalid local %q: %v", name, err)
		return Value{Type: typ}
	}
	id := len(b.function.Locals)
	b.function.Locals = append(b.function.Locals, typ)
	slots := make([]Expr, typ.Slots)
	for offset := range slots {
		slots[offset] = Load{Place: LocalPlace{ID: id, Name: name, Offset: offset}}
	}
	return Value{Type: typ, Slots: slots}
}

func (b *Builder) ReuseLocal(id int, name string, typ Type) Value {
	if err := validateType(typ); err != nil {
		b.fail("invalid reused local %q: %v", name, err)
		return Value{Type: typ}
	}
	if id < 0 || id >= len(b.function.Locals) {
		b.fail("reused local %q has invalid ID %d", name, id)
		return Value{Type: typ}
	}
	if !sameTypeLayout(b.function.Locals[id], typ) {
		b.fail("reused local %q has type %#v; expected %#v", name, b.function.Locals[id], typ)
		return Value{Type: typ}
	}
	slots := make([]Expr, typ.Slots)
	for offset := range slots {
		slots[offset] = Load{Place: LocalPlace{ID: id, Name: name, Offset: offset}}
	}
	return Value{Type: typ, Slots: slots}
}

func (b *Builder) IndexedLocal(base LocalPlace, index Expr, length, stride, offset int) (IndexedLocalPlace, error) {
	place := IndexedLocalPlace{
		ID: base.ID, Name: base.Name, Index: index, Base: base.Offset,
		Length: length, Stride: stride, Offset: offset,
	}
	if err := validatePlace(place, b.function); err != nil {
		return IndexedLocalPlace{}, b.fail("invalid indexed local place: %v", err)
	}
	return place, nil
}

func (b *Builder) Memory(storage string, index Expr, stride, offset int, read, write bool) (MemoryPlace, error) {
	place := MemoryPlace{Storage: storage, Index: index, Stride: stride, Offset: offset, Read: read, Write: write}
	if err := validatePlace(place, b.function); err != nil {
		return MemoryPlace{}, b.fail("invalid memory place: %v", err)
	}
	return place, nil
}

func ZeroValue(typ Type) Value {
	value := Value{Type: typ, Slots: make([]Expr, typ.Slots)}
	for i := range value.Slots {
		value.Slots[i] = Const{}
	}
	return value
}

func (b *Builder) ZeroValue(typ Type) Value { return ZeroValue(typ) }

func Places(value Value) []Place {
	places := make([]Place, len(value.Slots))
	for i, slot := range value.Slots {
		load, ok := slot.(Load)
		if !ok {
			return nil
		}
		places[i] = load.Place
	}
	return places
}

func SliceValue(value Value, typ Type, offset int) (Value, error) {
	if offset < 0 || offset+typ.Slots > len(value.Slots) {
		return Value{}, fmt.Errorf("value slice [%d:%d] exceeds %d slots", offset, offset+typ.Slots, len(value.Slots))
	}
	return Value{Type: typ, Slots: value.Slots[offset : offset+typ.Slots]}, nil
}

func Flatten(values ...Value) []Expr {
	var result []Expr
	for _, value := range values {
		result = append(result, value.Slots...)
	}
	return result
}

func (b *Builder) Store(places []Place, value Value, pos SourcePos) error {
	if len(places) != len(value.Slots) {
		return b.fail("store layout mismatch: %d places for %d slots", len(places), len(value.Slots))
	}
	if err := b.requireOpen(); err != nil {
		return err
	}
	for i, place := range places {
		if place == nil {
			return b.fail("store target is not addressable")
		}
		if err := validatePlace(place, b.function); err != nil {
			return b.fail("invalid store place %d: %v", i, err)
		}
		if memory, ok := place.(MemoryPlace); ok && !memory.Write {
			return b.fail("%s storage is read-only", memory.Storage)
		}
		if err := validateExpr(value.Slots[i], b.function); err != nil {
			return b.fail("invalid store value %d: %v", i, err)
		}
	}
	for i, place := range places {
		b.current.Instructions = append(b.current.Instructions, Store{Place: place, Value: value.Slots[i], Pos: pos})
	}
	return nil
}

func (b *Builder) Eval(value Expr) error {
	if err := b.requireOpen(); err != nil {
		return err
	}
	call, ok := value.(RuntimeCall)
	if !ok || call.Result.Slots != 0 {
		return b.fail("eval requires a void runtime call")
	}
	if err := validateRuntimeCall(call, b.function); err != nil {
		return b.fail("invalid eval: %v", err)
	}
	b.current.Instructions = append(b.current.Instructions, Eval{Value: call})
	return nil
}

func (b *Builder) RuntimeCall(fn resource.RuntimeFunction, args []Expr, result Type, pure bool, pos SourcePos) RuntimeCall {
	return RuntimeCall{Function: fn, Args: append([]Expr(nil), args...), Result: result, Pure: pure, Pos: pos}
}

func (b *Builder) Jump(target *Block) error { return b.terminate(Jump{Target: b.target(target)}) }
func (b *Builder) JumpIfOpen(target *Block) error {
	if err := b.requireCurrent(); err != nil {
		return err
	}
	if b.current.Terminator != nil {
		return nil
	}
	return b.Jump(target)
}
func (b *Builder) Branch(condition Expr, whenTrue, whenFalse *Block) error {
	if err := validateExpr(condition, b.function); err != nil {
		return b.fail("invalid branch condition: %v", err)
	}
	return b.terminate(Branch{Condition: condition, True: b.target(whenTrue), False: b.target(whenFalse)})
}
func (b *Builder) Switch(value Expr, cases []SwitchCase, defaultBlock *Block) error {
	if err := validateExpr(value, b.function); err != nil {
		return b.fail("invalid switch value: %v", err)
	}
	for _, item := range cases {
		if !b.hasBlock(item.Target) {
			return b.fail("switch target block %d does not exist", item.Target)
		}
	}
	return b.terminate(Switch{Value: value, Cases: append([]SwitchCase(nil), cases...), Default: b.target(defaultBlock)})
}
func (b *Builder) Return(value Value) error {
	if len(value.Slots) != b.function.Result.Slots || !sameTypeLayout(value.Type, b.function.Result) {
		return b.fail("return layout does not match function result")
	}
	for i, slot := range value.Slots {
		if err := validateExpr(slot, b.function); err != nil {
			return b.fail("invalid return value %d: %v", i, err)
		}
	}
	return b.terminate(Return{Value: value})
}
func (b *Builder) MarkUnreachable() error { return b.terminate(Unreachable{}) }

func (b *Builder) Finish() (*Function, error) {
	if err := Validate(b.function); err != nil {
		b.errs = append(b.errs, err)
	}
	if len(b.errs) != 0 {
		return nil, errors.Join(b.errs...)
	}
	return b.function, nil
}

func (b *Builder) terminate(term Terminator) error {
	if err := b.requireCurrent(); err != nil {
		return err
	}
	if b.current.Terminator != nil {
		return b.fail("block %d already has a terminator", b.current.ID)
	}
	b.current.Terminator = term
	return nil
}

func (b *Builder) target(block *Block) int {
	if !b.ownsBlock(block) {
		b.fail("branch target does not belong to this function")
		return -1
	}
	return block.ID
}
func (b *Builder) ownsBlock(block *Block) bool {
	return block != nil && b.hasBlock(block.ID) && b.function.Blocks[block.ID] == block
}
func (b *Builder) hasBlock(id int) bool {
	return id >= 0 && id < len(b.function.Blocks) && b.function.Blocks[id].ID == id
}
func (b *Builder) requireCurrent() error {
	if b.current == nil {
		return b.fail("no current block")
	}
	return nil
}
func (b *Builder) requireOpen() error {
	if err := b.requireCurrent(); err != nil {
		return err
	}
	if b.current.Terminator != nil {
		return b.fail("block %d already has a terminator", b.current.ID)
	}
	return nil
}
func (b *Builder) fail(format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	b.errs = append(b.errs, err)
	return err
}
