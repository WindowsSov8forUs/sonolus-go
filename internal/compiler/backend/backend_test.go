package backend

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/frontend"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/optimize"
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
	function, err = optimize.NewOptimizer(optimize.LevelMinimal).Optimize(optimize.Context{Mode: mode.ModePlay, Callback: "indexed"}, function)
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
	function, _ = optimize.NewOptimizer(optimize.LevelMinimal).Optimize(optimize.Context{Mode: mode.ModePlay, Callback: "export"}, function)
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
func TestOmitConstantCallback(t *testing.T) {
	tests := []struct {
		name     string
		mode     mode.Mode
		callback string
		value    valueNode
		want     bool
	}{
		{name: "play spawn order default", mode: mode.ModePlay, callback: "spawnOrder", value: 0, want: true},
		{name: "play spawn order nondefault", mode: mode.ModePlay, callback: "spawnOrder", value: 1, want: false},
		{name: "play should spawn default", mode: mode.ModePlay, callback: "shouldSpawn", value: 1, want: true},
		{name: "play should spawn false", mode: mode.ModePlay, callback: "shouldSpawn", value: 0, want: false},
		{name: "watch spawn time default", mode: mode.ModeWatch, callback: "spawnTime", value: 0, want: true},
		{name: "watch spawn time nondefault", mode: mode.ModeWatch, callback: "spawnTime", value: 1, want: false},
		{name: "watch despawn time default", mode: mode.ModeWatch, callback: "despawnTime", value: 0, want: true},
		{name: "watch despawn time nondefault", mode: mode.ModeWatch, callback: "despawnTime", value: -1, want: false},
		{name: "watch update spawn default", mode: mode.ModeWatch, callback: "updateSpawn", value: 0, want: true},
		{name: "watch update spawn nondefault", mode: mode.ModeWatch, callback: "updateSpawn", value: 3, want: false},
		{name: "ordinary callback", mode: mode.ModeWatch, callback: "initialize", value: 1, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := omitConstantCallback(test.mode, test.callback, test.value); got != test.want {
				t.Fatalf("omitConstantCallback(%s, %q, %v) = %v, want %v", test.mode, test.callback, test.value, got, test.want)
			}
		})
	}
}
func TestSNodeMultiplyZeroPreservesDynamicEvaluation(t *testing.T) {
	dynamic := call(resource.RuntimeFunctionDebugLog, valueNode(1))
	result := simplify(call(resource.RuntimeFunctionMultiply, valueNode(0), dynamic))
	execute, ok := result.(functionNode)
	if !ok || execute.function != resource.RuntimeFunctionExecute || len(execute.args) != 2 || !isValue(execute.args[1], 0) {
		t.Fatalf("result = %#v", result)
	}
}

func TestSNodeFusesSetAdd(t *testing.T) {
	get := call(resource.RuntimeFunctionGet, valueNode(10000), valueNode(3))
	result := simplify(call(resource.RuntimeFunctionSet, valueNode(10000), valueNode(3), call(resource.RuntimeFunctionAdd, get, valueNode(2))))
	set, ok := result.(functionNode)
	if !ok || set.function != resource.RuntimeFunctionSetAdd || len(set.args) != 3 {
		t.Fatalf("result = %#v", result)
	}
}

func TestSNodeNormalizesArithmeticSwitch(t *testing.T) {
	result := simplify(call(resource.RuntimeFunctionSwitchWithDefault,
		call(resource.RuntimeFunctionGet, valueNode(1), valueNode(2)),
		valueNode(2), valueNode(10), valueNode(4), valueNode(20), valueNode(6), valueNode(30), valueNode(0),
	))
	switchNode, ok := result.(functionNode)
	if !ok || switchNode.function != resource.RuntimeFunctionSwitchInteger || len(switchNode.args) != 4 {
		t.Fatalf("result = %#v", result)
	}
}
