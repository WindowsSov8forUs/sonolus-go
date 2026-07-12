package ir

import (
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

func TestTypedCFG(t *testing.T) {
	local := LocalPlace{ID: 0, Name: "x"}
	entry := &Block{ID: 0}
	entry.Instructions = append(entry.Instructions, Store{Place: local, Value: Const{Value: 1}})
	entry.Terminator = Branch{Condition: RuntimeCall{Function: resource.RuntimeFunctionGreater, Args: []Expr{Load{Place: local}, Const{}}, Result: Type{Name: "bool", Slots: 1}, Pure: true}, True: 1, False: 2}
	fn := Function{Name: "test", Entry: 0, Blocks: []*Block{entry, {ID: 1, Terminator: Return{}}, {ID: 2, Terminator: Unreachable{}}}}
	if fn.Entry != 0 || len(fn.Blocks) != 3 {
		t.Fatalf("unexpected CFG: %#v", fn)
	}
	branch, ok := fn.Blocks[0].Terminator.(Branch)
	if !ok || branch.True != 1 || branch.False != 2 {
		t.Fatalf("unexpected branch: %#v", fn.Blocks[0].Terminator)
	}
}

func TestBuilderFinish(t *testing.T) {
	b := NewBuilder("callback", Type{Name: "float64", Slots: 1})
	entry, exit := b.NewBlock(), b.NewBlock()
	if err := b.SetEntry(entry); err != nil {
		t.Fatal(err)
	}
	if err := b.SetCurrent(entry); err != nil {
		t.Fatal(err)
	}
	local := b.NewLocal("x", Type{Name: "float64", Slots: 1})
	if err := b.Store(Places(local), Value{Type: local.Type, Slots: []Expr{Const{Value: 2}}}, SourcePos{}); err != nil {
		t.Fatal(err)
	}
	if err := b.Jump(exit); err != nil {
		t.Fatal(err)
	}
	if err := b.SetCurrent(exit); err != nil {
		t.Fatal(err)
	}
	if err := b.Return(local); err != nil {
		t.Fatal(err)
	}
	fn, err := b.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if fn.Entry != 0 || len(fn.Locals) != 1 || len(fn.Blocks) != 2 {
		t.Fatalf("unexpected function: %#v", fn)
	}
}

func TestBuilderRejectsInvalidIR(t *testing.T) {
	b := NewBuilder("invalid", Type{})
	entry := b.NewBlock()
	_ = b.SetEntry(entry)
	_ = b.SetCurrent(entry)
	readonly := MemoryPlace{Storage: "imported", Index: Const{}, Read: true}
	if err := b.Store([]Place{readonly}, Value{Type: Type{Slots: 1}, Slots: []Expr{Const{}}}, SourcePos{}); err == nil {
		t.Fatal("expected read-only store error")
	}
	_ = b.Jump(entry)
	if err := b.Jump(entry); err == nil {
		t.Fatal("expected duplicate terminator error")
	}
	if _, err := b.Finish(); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuilderCreatesValidatedPlaces(t *testing.T) {
	b := NewBuilder("places", Type{})
	local := b.NewLocal("array", Type{Name: "array", Slots: 4})
	base := Places(local)[0].(LocalPlace)
	indexed, err := b.IndexedLocal(base, Const{Value: 1}, 2, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if indexed.Base != 0 || indexed.Length != 2 || indexed.Stride != 2 || indexed.Offset != 1 {
		t.Fatalf("unexpected indexed place: %#v", indexed)
	}
	memory, err := b.Memory("shared", Const{}, 4, 2, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if memory.Storage != "shared" || memory.Stride != 4 || memory.Offset != 2 {
		t.Fatalf("unexpected memory place: %#v", memory)
	}
}

func TestBuilderRejectsInvalidPlacesImmediately(t *testing.T) {
	b := NewBuilder("places", Type{})
	local := b.NewLocal("array", Type{Name: "array", Slots: 4})
	base := Places(local)[0].(LocalPlace)
	if _, err := b.IndexedLocal(base, Const{}, 2, 3, 0); err == nil {
		t.Fatal("expected indexed local layout error")
	}
	if _, err := b.Memory("", Const{}, 0, 0, true, false); err == nil {
		t.Fatal("expected memory layout error")
	}
}

func TestBuilderJumpIfOpenPreservesTerminator(t *testing.T) {
	b := NewBuilder("terminated", Type{})
	entry, first, second := b.NewBlock(), b.NewBlock(), b.NewBlock()
	_ = b.SetEntry(entry)
	_ = b.SetCurrent(entry)
	if err := b.Jump(first); err != nil {
		t.Fatal(err)
	}
	if err := b.JumpIfOpen(second); err != nil {
		t.Fatal(err)
	}
	if got := entry.Terminator.(Jump).Target; got != first.ID {
		t.Fatalf("jump target = %d, want %d", got, first.ID)
	}
}

func TestBuilderRejectsForgedBlockAndInstructionAfterTerminator(t *testing.T) {
	b := NewBuilder("ownership", Type{})
	entry := b.NewBlock()
	if err := b.SetEntry(entry); err != nil {
		t.Fatal(err)
	}
	if err := b.SetCurrent(&Block{ID: entry.ID}); err == nil {
		t.Fatal("expected forged block to be rejected")
	}
	if err := b.SetCurrent(entry); err != nil {
		t.Fatal(err)
	}
	if err := b.Return(Value{Type: Type{}}); err != nil {
		t.Fatal(err)
	}
	if err := b.Eval(RuntimeCall{Function: resource.RuntimeFunctionDebugPause, Result: Type{}}); err == nil {
		t.Fatal("expected instruction after terminator to be rejected")
	}
}

func TestBuilderRejectsInvalidLocalTypeWithoutPanicking(t *testing.T) {
	b := NewBuilder("invalid-local", Type{})
	value := b.NewLocal("bad", Type{Name: "bad", Slots: -1})
	if len(value.Slots) != 0 {
		t.Fatalf("invalid local returned slots: %#v", value)
	}
	if _, err := b.Finish(); err == nil || !strings.Contains(err.Error(), "invalid local") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRejectsReachableUnterminatedBlock(t *testing.T) {
	fn := &Function{Name: "invalid", Entry: 0, Blocks: []*Block{{ID: 0}}}
	if err := Validate(fn); err == nil || !strings.Contains(err.Error(), "no terminator") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryPlaceRetainsSemanticStorage(t *testing.T) {
	p := MemoryPlace{Storage: "shared", Index: Const{Value: 3}, Offset: 2, Read: true, Write: true}
	if p.Storage != "shared" || p.Offset != 2 || !p.Read || !p.Write {
		t.Fatalf("unexpected memory place: %#v", p)
	}
}

func TestValidateRejectsInvalidLoadedPlace(t *testing.T) {
	fn := &Function{
		Name:   "invalid-load",
		Entry:  0,
		Locals: []Type{{Name: "float64", Slots: 1}},
		Blocks: []*Block{{ID: 0, Terminator: Return{Value: Value{Type: Type{Slots: 1}, Slots: []Expr{Load{Place: LocalPlace{ID: 0, Offset: 1}}}}}}},
		Result: Type{Slots: 1},
	}
	if err := Validate(fn); err == nil || !strings.Contains(err.Error(), "outside its layout") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRejectsWriteOnlyLoad(t *testing.T) {
	fn := &Function{
		Name:   "write-only-load",
		Entry:  0,
		Blocks: []*Block{{ID: 0, Terminator: Return{Value: Value{Type: Type{Slots: 1}, Slots: []Expr{Load{Place: MemoryPlace{Storage: "exported", Index: Const{}, Write: true}}}}}}},
		Result: Type{Slots: 1},
	}
	if err := Validate(fn); err == nil || !strings.Contains(err.Error(), "write-only") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateChecksUnreachableBlockStructure(t *testing.T) {
	fn := &Function{
		Name:  "invalid-unreachable",
		Entry: 0,
		Blocks: []*Block{
			{ID: 0, Terminator: Return{}},
			{ID: 1, Terminator: Jump{Target: 3}},
		},
	}
	if err := Validate(fn); err == nil || !strings.Contains(err.Error(), "target 3") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRejectsInvalidDynamicPlace(t *testing.T) {
	fn := &Function{
		Name:   "invalid-indexed-local",
		Entry:  0,
		Locals: []Type{{Name: "array", Slots: 4}},
		Blocks: []*Block{{ID: 0, Terminator: Return{Value: Value{Type: Type{Name: "float64", Slots: 1}, Slots: []Expr{
			Load{Place: IndexedLocalPlace{ID: 0, Index: Load{Place: LocalPlace{ID: 9}}, Length: 2, Stride: 3, Offset: 2}},
		}}}}},
		Result: Type{Name: "float64", Slots: 1},
	}
	if err := Validate(fn); err == nil || !strings.Contains(err.Error(), "invalid layout") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRejectsReturnTypeMismatch(t *testing.T) {
	fn := &Function{
		Name:   "wrong-return-type",
		Entry:  0,
		Blocks: []*Block{{ID: 0, Terminator: Return{Value: Value{Type: Type{Name: "int", Slots: 1}, Slots: []Expr{Const{}}}}}},
		Result: Type{Name: "float64", Slots: 1},
	}
	if err := Validate(fn); err == nil || !strings.Contains(err.Error(), "returns type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateChecksRuntimeCallResultShape(t *testing.T) {
	fn := &Function{
		Name:  "scalar-eval",
		Entry: 0,
		Blocks: []*Block{{
			ID: 0,
			Instructions: []Instruction{Eval{Value: RuntimeCall{
				Function: resource.RuntimeFunctionAdd,
				Result:   Type{Name: "float64", Slots: 1},
			}}},
			Terminator: Return{Value: Value{Type: Type{Name: "void"}}},
		}},
		Result: Type{Name: "void"},
	}
	if err := Validate(fn); err == nil || !strings.Contains(err.Error(), "eval requires a void runtime call") {
		t.Fatalf("unexpected error: %v", err)
	}
}
