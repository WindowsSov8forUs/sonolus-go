package backend

import (
	"testing"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
)

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
