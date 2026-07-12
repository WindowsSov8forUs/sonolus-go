package backend

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/internal/newcompiler/mode"
)

func TestCompileRejectsMisalignedROM(t *testing.T) {
	_, err := Compile(&frontend.Project{ROM: []byte{1}, Modes: map[mode.Mode]*frontend.ModeDeclarations{}})
	if err == nil || !strings.Contains(err.Error(), "ROM length 1 is not divisible by 4") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFinalizeIndexedLocalUsesShiftedRuntimeFunctions(t *testing.T) {
	builder := ir.NewBuilder("indexed", ir.Type{})
	entry := builder.NewBlock()
	_ = builder.SetEntry(entry)
	_ = builder.SetCurrent(entry)
	local := builder.NewLocal("values", ir.Type{Name: "array", Slots: 4})
	base := ir.Places(local)[0].(ir.LocalPlace)
	place, err := builder.IndexedLocal(base, ir.Const{Value: 2}, 2, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.Store([]ir.Place{place}, ir.Value{Type: ir.Type{Name: "float", Slots: 1}, Slots: []ir.Expr{ir.Const{Value: 7}}}, ir.SourcePos{}); err != nil {
		t.Fatal(err)
	}
	_ = builder.Return(ir.Value{Type: ir.Type{}})
	function, err := builder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	node, err := finalizeFunction(mode.ModePlay, function)
	if err != nil {
		t.Fatal(err)
	}
	if !containsFunction(node, resource.RuntimeFunctionSetShifted) {
		t.Fatalf("final node does not contain SetShifted: %#v", node)
	}
}

func TestFinalizeExportedStoreUsesExportValue(t *testing.T) {
	builder := ir.NewBuilder("export", ir.Type{})
	entry := builder.NewBlock()
	_ = builder.SetEntry(entry)
	_ = builder.SetCurrent(entry)
	place, _ := builder.Memory("exported", ir.Const{}, 0, 3, false, true)
	_ = builder.Store([]ir.Place{place}, ir.Value{Type: ir.Type{Name: "float", Slots: 1}, Slots: []ir.Expr{ir.Const{Value: 9}}}, ir.SourcePos{})
	_ = builder.Return(ir.Value{Type: ir.Type{}})
	function, _ := builder.Finish()
	node, err := finalizeFunction(mode.ModePlay, function)
	if err != nil {
		t.Fatal(err)
	}
	if !containsFunction(node, resource.RuntimeFunctionExportValue) {
		t.Fatalf("final node does not contain ExportValue: %#v", node)
	}
}

func TestBuildROMPrefixesNonFiniteValues(t *testing.T) {
	user := make([]byte, 4)
	binary.LittleEndian.PutUint32(user, math.Float32bits(4.5))
	result := buildROM(user)
	if len(result) != 16 {
		t.Fatalf("ROM length = %d, want 16", len(result))
	}
	values := make([]float32, 4)
	for i := range values {
		values[i] = math.Float32frombits(binary.LittleEndian.Uint32(result[i*4:]))
	}
	if !math.IsNaN(float64(values[0])) || !math.IsInf(float64(values[1]), 1) || !math.IsInf(float64(values[2]), -1) || values[3] != 4.5 {
		t.Fatalf("unexpected ROM values: %v", values)
	}
}

func TestFinalizeRejectsUnknownSemanticStorage(t *testing.T) {
	builder := ir.NewBuilder("unknown", ir.Type{Name: "float", Slots: 1})
	entry := builder.NewBlock()
	_ = builder.SetEntry(entry)
	_ = builder.SetCurrent(entry)
	place, _ := builder.Memory("UnknownStorage", ir.Const{}, 0, 0, true, false)
	_ = builder.Return(ir.Value{Type: ir.Type{Name: "float", Slots: 1}, Slots: []ir.Expr{ir.Load{Place: place}}})
	function, _ := builder.Finish()
	if _, err := finalizeFunction(mode.ModePlay, function); err == nil {
		t.Fatal("unknown storage was accepted")
	}
}

func TestNodeAppenderDeduplicatesDeterministically(t *testing.T) {
	appender := newNodeAppender()
	node := call(resource.RuntimeFunctionAdd, valueNode(1), valueNode(2))
	first, err := appender.append(node)
	if err != nil {
		t.Fatal(err)
	}
	second, err := appender.append(node)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first != 2 || len(appender.nodes) != 3 {
		t.Fatalf("indexes=(%d,%d) nodes=%d", first, second, len(appender.nodes))
	}
	function, ok := appender.nodes[first].(resource.EngineDataFunctionNode)
	if !ok || len(function.Args) != 2 || function.Args[0] != 0 || function.Args[1] != 1 {
		t.Fatalf("unexpected function node: %#v", appender.nodes[first])
	}
}

func containsFunction(node snode, function resource.RuntimeFunction) bool {
	value, ok := node.(functionNode)
	if !ok {
		return false
	}
	if value.function == function {
		return true
	}
	for _, argument := range value.args {
		if containsFunction(argument, function) {
			return true
		}
	}
	return false
}
