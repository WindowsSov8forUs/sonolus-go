package simexec

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/catalog"
)

type ExecutionErrorKind string

const (
	ExecutionErrorInvalidRequest  ExecutionErrorKind = "invalid-request"
	ExecutionErrorInvalidNode     ExecutionErrorKind = "invalid-node"
	ExecutionErrorInvalidArity    ExecutionErrorKind = "invalid-arity"
	ExecutionErrorInvalidArgument ExecutionErrorKind = "invalid-argument"
	ExecutionErrorInvalidState    ExecutionErrorKind = "invalid-state"
	ExecutionErrorMissingHandler  ExecutionErrorKind = "missing-handler"
	ExecutionErrorStepLimit       ExecutionErrorKind = "step-limit"
)

type ExecutionError struct {
	Kind          ExecutionErrorKind
	NodeIndex     int
	Function      resource.RuntimeFunction
	ArgumentIndex int
	Step          int
	Message       string
}

func (err *ExecutionError) Error() string {
	if err == nil {
		return "sim: execution error"
	}
	context := ""
	if err.NodeIndex >= 0 {
		context += fmt.Sprintf(" node %d", err.NodeIndex)
	}
	if err.Function != "" {
		context += " " + string(err.Function)
	}
	if err.ArgumentIndex >= 0 {
		context += fmt.Sprintf(" argument %d", err.ArgumentIndex)
	}
	if err.Step > 0 {
		context += fmt.Sprintf(" at step %d", err.Step)
	}
	if err.Message == "" {
		return fmt.Sprintf("sim: %s%s", err.Kind, context)
	}
	return fmt.Sprintf("sim: %s%s: %s", err.Kind, context, err.Message)
}

func executionError(kind ExecutionErrorKind, node int, function resource.RuntimeFunction, argument, step int, format string, args ...any) error {
	return &ExecutionError{Kind: kind, NodeIndex: node, Function: function, ArgumentIndex: argument, Step: step, Message: fmt.Sprintf(format, args...)}
}

func validateRuntimeArity(function resource.RuntimeFunction, count int) error {
	return validateRuntimeArityAt(-1, function, count, 0)
}

func validateRuntimeArityAt(node int, function resource.RuntimeFunction, count, step int) error {
	invalid := func(expectation string) error {
		return executionError(ExecutionErrorInvalidArity, node, function, -1, step, "got %d arguments, expected %s", count, expectation)
	}
	metadata, known := catalog.LookupRuntimeSimulation(function)
	if !known {
		return executionError(ExecutionErrorInvalidNode, node, function, -1, step, "RuntimeFunction has no generated simulation policy")
	}
	switch metadata.Shape {
	case "variadic":
		return nil
	case "if":
		if count != 3 {
			return invalid("exactly 3")
		}
		return nil
	case "switch":
		if count < 1 || count%2 == 0 {
			return invalid("an odd count of at least 1")
		}
		return nil
	case "switch-default":
		if count < 2 || count%2 != 0 {
			return invalid("an even count of at least 2")
		}
		return nil
	case "switch-integer":
		if count < 1 {
			return invalid("at least 1")
		}
		return nil
	case "switch-integer-default":
		if count < 2 {
			return invalid("at least 2")
		}
		return nil
	case "jump-loop":
		if count < 1 {
			return invalid("at least 1")
		}
		return nil
	case "block":
		if count != 1 {
			return invalid("exactly 1")
		}
		return nil
	case "binary-control":
		if count != 2 {
			return invalid("exactly 2")
		}
		return nil
	case "":
	default:
		return executionError(ExecutionErrorInvalidNode, node, function, -1, step, "RuntimeFunction has unknown generated shape %q", metadata.Shape)
	}
	signature := metadata.Signature
	if count < signature.MinArgs {
		return invalid(fmt.Sprintf("at least %d", signature.MinArgs))
	}
	if signature.MaxArgs >= 0 && count > signature.MaxArgs {
		if signature.MinArgs == signature.MaxArgs {
			return invalid(fmt.Sprintf("exactly %d", signature.MinArgs))
		}
		return invalid(fmt.Sprintf("between %d and %d", signature.MinArgs, signature.MaxArgs))
	}
	return nil
}

type Handler func(resource.RuntimeFunction, []float64) (float64, error)

type StreamEntry struct {
	Key   float64
	Value float64
}

type Request struct {
	Memory        map[int][]float64
	Streams       map[int][]StreamEntry
	ROM           []byte
	RandomSeed    int64
	StepLimit     int
	Handler       Handler
	DefaultMemory *float64
}

type SideEffect struct {
	Function  resource.RuntimeFunction
	Arguments []float64
	Result    float64
}

type Result struct {
	Value   float64
	Memory  map[int][]float64
	Streams map[int][]StreamEntry
	Effects []SideEffect
	Steps   int
}

func Classify(function resource.RuntimeFunction) string { return string(classifyRuntime(function)) }

func ValidateStreams(streams map[int][]StreamEntry) error {
	_, err := normalizeStreams(streams)
	return err
}

type Machine struct{ executor *executor }

func NewMachine() *Machine {
	return &Machine{executor: &executor{
		memory:  map[int][]float64{},
		streams: map[int]*streamState{},
		random:  rand.New(rand.NewSource(0)),
	}}
}

func (machine *Machine) Builtin(function resource.RuntimeFunction, arguments ...float64) (float64, bool, error) {
	if machine == nil || machine.executor == nil {
		return 0, false, fmt.Errorf("sim: machine is nil")
	}
	if err := validateRuntimeArity(function, len(arguments)); err != nil {
		return 0, false, err
	}
	if err := validateRuntimeArgumentsAt(-1, function, arguments, machine.executor.steps); err != nil {
		return 0, false, err
	}
	value, handled, err := machine.executor.builtin(function, arguments)
	if err != nil {
		return 0, handled, executionError(ExecutionErrorInvalidArgument, -1, function, -1, machine.executor.steps, "%v", err)
	}
	return value, handled, nil
}

func Execute(nodes []resource.EngineDataNode, root int, request Request) (Result, error) {
	memory := cloneMemory(request.Memory)
	streams, err := normalizeStreams(request.Streams)
	if err != nil {
		return Result{}, executionError(ExecutionErrorInvalidRequest, -1, "", -1, 0, "%v", err)
	}
	rom := make([]float64, len(request.ROM)/4)
	for index := range rom {
		rom[index] = float64(math.Float32frombits(binary.LittleEndian.Uint32(request.ROM[index*4:])))
	}
	memory[3000] = rom
	limit := request.StepLimit
	if limit <= 0 {
		limit = 1_000_000
	}
	executor := &executor{nodes: nodes, memory: memory, random: rand.New(rand.NewSource(request.RandomSeed)), limit: limit, handler: request.Handler, streams: streams, defaultMemory: request.DefaultMemory}
	value, signal, err := executor.eval(root)
	if err != nil {
		return Result{}, err
	}
	if signal != nil {
		value = signal.value
	}
	return Result{Value: value, Memory: cloneMemory(memory), Streams: cloneStreams(streams), Effects: append([]SideEffect(nil), executor.effects...), Steps: executor.steps}, nil
}

type breakSignal struct {
	count int
	value float64
}
type executor struct {
	nodes         []resource.EngineDataNode
	memory        map[int][]float64
	random        *rand.Rand
	limit, steps  int
	handler       Handler
	effects       []SideEffect
	streams       map[int]*streamState
	stack         []float64
	stackPointer  int
	framePointer  int
	defaultMemory *float64
}

func (e *executor) eval(index int) (float64, *breakSignal, error) {
	if index < 0 || index >= len(e.nodes) {
		return 0, nil, executionError(ExecutionErrorInvalidNode, index, "", -1, e.steps, "node index is outside [0,%d)", len(e.nodes))
	}
	e.steps++
	if e.steps > e.limit {
		return 0, nil, executionError(ExecutionErrorStepLimit, index, "", -1, e.steps, "step limit %d exceeded", e.limit)
	}
	switch node := e.nodes[index].(type) {
	case resource.EngineDataValueNode:
		return node.Value, nil, nil
	case *resource.EngineDataValueNode:
		if node == nil {
			return 0, nil, executionError(ExecutionErrorInvalidNode, index, "", -1, e.steps, "value node is nil")
		}
		return node.Value, nil, nil
	case resource.EngineDataFunctionNode:
		return e.call(index, node.Func, node.Args)
	case *resource.EngineDataFunctionNode:
		if node == nil {
			return 0, nil, executionError(ExecutionErrorInvalidNode, index, "", -1, e.steps, "function node is nil")
		}
		return e.call(index, node.Func, node.Args)
	default:
		return 0, nil, executionError(ExecutionErrorInvalidNode, index, "", -1, e.steps, "unsupported node %T", node)
	}
}

func (e *executor) call(nodeIndex int, function resource.RuntimeFunction, indexes []int) (float64, *breakSignal, error) {
	if err := validateRuntimeArityAt(nodeIndex, function, len(indexes), e.steps); err != nil {
		return 0, nil, err
	}
	name := string(function)
	eval := func(index int) (float64, *breakSignal, error) { return e.eval(indexes[index]) }
	switch name {
	case "Execute", "Execute0":
		var value float64
		for i := range indexes {
			var signal *breakSignal
			var err error
			value, signal, err = eval(i)
			if err != nil || signal != nil {
				return value, signal, err
			}
		}
		if name == "Execute0" {
			value = 0
		}
		return value, nil, nil
	case "If":
		condition, signal, err := eval(0)
		if err != nil || signal != nil {
			return 0, signal, err
		}
		if condition != 0 {
			return eval(1)
		}
		return eval(2)
	case "SwitchIntegerWithDefault":
		value, signal, err := eval(0)
		if err != nil || signal != nil {
			return 0, signal, err
		}
		selected := len(indexes) - 1
		integerValue, integerErr := integer("switch value", value)
		if integerErr != nil {
			return 0, nil, executionError(ExecutionErrorInvalidArgument, nodeIndex, function, 0, e.steps, "%v", integerErr)
		}
		candidate := integerValue + 1
		if candidate >= 1 && candidate < len(indexes)-1 {
			selected = candidate
		}
		return eval(selected)
	case "SwitchInteger":
		value, signal, err := eval(0)
		if err != nil || signal != nil {
			return 0, signal, err
		}
		integerValue, integerErr := integer("switch value", value)
		if integerErr != nil {
			return 0, nil, executionError(ExecutionErrorInvalidArgument, nodeIndex, function, 0, e.steps, "%v", integerErr)
		}
		candidate := integerValue + 1
		if candidate >= 1 && candidate < len(indexes) {
			return eval(candidate)
		}
		return 0, nil, nil
	case "Switch":
		value, signal, err := eval(0)
		if err != nil || signal != nil {
			return 0, signal, err
		}
		for i := 1; i+1 < len(indexes); i += 2 {
			key, nextSignal, nextErr := eval(i)
			if nextErr != nil || nextSignal != nil {
				return 0, nextSignal, nextErr
			}
			if key == value {
				return eval(i + 1)
			}
		}
		return 0, nil, nil
	case "SwitchWithDefault":
		value, signal, err := eval(0)
		if err != nil || signal != nil {
			return 0, signal, err
		}
		for i := 1; i+1 < len(indexes); i += 2 {
			key, s, e2 := eval(i)
			if e2 != nil || s != nil {
				return 0, s, e2
			}
			if key == value {
				return eval(i + 1)
			}
		}
		return eval(len(indexes) - 1)
	case "JumpLoop":
		block := 0
		for {
			if block < 0 || block >= len(indexes) {
				return 0, nil, executionError(ExecutionErrorInvalidState, nodeIndex, function, -1, e.steps, "jump target %d is outside [0,%d)", block, len(indexes))
			}
			if block == len(indexes)-1 {
				return eval(block)
			}
			value, signal, err := eval(block)
			if err != nil {
				return 0, nil, err
			}
			if signal != nil {
				return 0, signal, nil
			}
			target, targetErr := integer("jump target", value)
			if targetErr != nil {
				return 0, nil, executionError(ExecutionErrorInvalidArgument, nodeIndex, function, block, e.steps, "%v", targetErr)
			}
			block = target
		}
	case "Block":
		value, signal, err := eval(0)
		if signal != nil {
			if signal.count <= 1 {
				return signal.value, nil, nil
			}
			signal.count--
		}
		return value, signal, err
	case "Break":
		count, s, err := eval(0)
		if err != nil || s != nil {
			return 0, s, err
		}
		countValue, countErr := integer("break count", count)
		if countErr != nil || countValue < 1 {
			if countErr == nil {
				countErr = fmt.Errorf("break count must be positive")
			}
			return 0, nil, executionError(ExecutionErrorInvalidArgument, nodeIndex, function, 0, e.steps, "%v", countErr)
		}
		value, s, err := eval(1)
		if err != nil || s != nil {
			return 0, s, err
		}
		return value, &breakSignal{count: countValue, value: value}, nil
	case "And", "Or":
		var value float64
		for i := range indexes {
			var signal *breakSignal
			var err error
			value, signal, err = eval(i)
			if err != nil || signal != nil {
				return value, signal, err
			}
			if (name == "And" && value == 0) || (name == "Or" && value != 0) {
				return value, nil, nil
			}
		}
		return value, nil, nil
	case "While", "DoWhile":
		if name == "DoWhile" {
			for {
				if _, signal, err := eval(0); err != nil || signal != nil {
					return 0, signal, err
				}
				condition, signal, err := eval(1)
				if err != nil || signal != nil || condition == 0 {
					return 0, signal, err
				}
			}
		}
		for {
			condition, signal, err := eval(0)
			if err != nil || signal != nil || condition == 0 {
				return 0, signal, err
			}
			if _, signal, err := eval(1); err != nil || signal != nil {
				return 0, signal, err
			}
		}
	}
	args := make([]float64, len(indexes))
	for i := range indexes {
		value, signal, err := eval(i)
		if err != nil || signal != nil {
			return 0, signal, err
		}
		args[i] = value
	}
	if err := validateRuntimeArgumentsAt(nodeIndex, function, args, e.steps); err != nil {
		return 0, nil, err
	}
	class := classifyRuntime(function)
	if class == "" {
		return 0, nil, executionError(ExecutionErrorInvalidNode, nodeIndex, function, -1, e.steps, "RuntimeFunction is unclassified")
	}
	value, handled, err := e.builtin(function, args)
	if err != nil {
		return 0, nil, executionError(ExecutionErrorInvalidArgument, nodeIndex, function, -1, e.steps, "%v", err)
	}
	if !handled && e.handler != nil && (class == classHandler || class == classEffect) {
		value, err = e.handler(function, args)
		handled = true
	}
	if err != nil {
		return 0, nil, executionError(ExecutionErrorInvalidState, nodeIndex, function, -1, e.steps, "%v", err)
	}
	if !handled {
		signature, known := catalog.LookupRuntimeSignature(function)
		if !known || signature.ResultSlots != 0 || class == classHandler {
			return 0, nil, executionError(ExecutionErrorMissingHandler, nodeIndex, function, -1, e.steps, "RuntimeFunction requires a handler")
		}
		value = 0
	}
	if !handled || runtimeEffect(function) {
		e.effects = append(e.effects, SideEffect{Function: function, Arguments: append([]float64(nil), args...), Result: value})
	}
	return value, nil, nil
}

func validateRuntimeArgumentsAt(node int, function resource.RuntimeFunction, arguments []float64, step int) error {
	integerArgument := func(index int, label string, nonNegative bool) error {
		value, err := integer(label, arguments[index])
		if err == nil && nonNegative && value < 0 {
			err = fmt.Errorf("%s must be non-negative", label)
		}
		if err != nil {
			return executionError(ExecutionErrorInvalidArgument, node, function, index, step, "%v", err)
		}
		return nil
	}
	finiteArgument := func(index int, label string) error {
		if math.IsNaN(arguments[index]) || math.IsInf(arguments[index], 0) {
			return executionError(ExecutionErrorInvalidArgument, node, function, index, step, "%s must be finite", label)
		}
		return nil
	}
	metadata, ok := catalog.LookupRuntimeSimulation(function)
	if !ok {
		return executionError(ExecutionErrorInvalidNode, node, function, -1, step, "RuntimeFunction has no generated simulation policy")
	}
	switch metadata.Arguments {
	case "memory":
		if err := integerArgument(0, "memory block", false); err != nil {
			return err
		}
		return integerArgument(1, "memory index", true)
	case "pointed-memory":
		if err := integerArgument(0, "pointer block", false); err != nil {
			return err
		}
		if err := integerArgument(1, "pointer index", true); err != nil {
			return err
		}
		return integerArgument(2, "pointer offset", false)
	case "shifted-memory":
		for index, label := range []string{"memory block", "shift x", "shift y", "shift stride"} {
			if err := integerArgument(index, label, index == 1 || index == 2); err != nil {
				return err
			}
		}
		return nil
	case "copy":
		labels := []string{"source block", "source index", "target block", "target index", "copy count"}
		for index, label := range labels {
			if err := integerArgument(index, label, index == 1 || index == 3 || index == 4); err != nil {
				return err
			}
		}
		return nil
	case "stream":
		if err := integerArgument(0, "stream ID", true); err != nil {
			return err
		}
		return finiteArgument(1, "stream key")
	case "integer-range":
		if err := integerArgument(0, "random minimum", false); err != nil {
			return err
		}
		return integerArgument(1, "random maximum", false)
	}
	return nil
}

func (e *executor) builtin(function resource.RuntimeFunction, a []float64) (float64, bool, error) {
	name := string(function)
	switch name {
	case "Get":
		return e.read(a[0], a[1])
	case "GetPointed":
		block, index, offset, err := address3(a)
		if err != nil {
			return 0, true, err
		}
		return e.readPointed(block, index, offset)
	case "GetShifted":
		block, index, err := shiftedAddress(a)
		if err != nil {
			return 0, true, err
		}
		value, err := e.readIndex(block, index)
		return value, true, err
	case "Set":
		return e.write(a[0], a[1], a[2])
	case "SetPointed":
		block, index, offset, err := address3(a)
		if err != nil {
			return 0, true, err
		}
		return e.writePointed(block, index, offset, a[3])
	case "SetShifted":
		block, index, err := shiftedAddress(a)
		if err != nil {
			return 0, true, err
		}
		return e.writeIndex(block, index, a[4])
	case "Copy":
		return e.copy(a)
	case "IncrementPost", "IncrementPostPointed", "IncrementPostShifted", "IncrementPre", "IncrementPrePointed", "IncrementPreShifted",
		"DecrementPost", "DecrementPostPointed", "DecrementPostShifted", "DecrementPre", "DecrementPrePointed", "DecrementPreShifted":
		return e.increment(name, a)
	case "Add":
		return fold(a, 0, func(x, y float64) float64 { return x + y }), true, nil
	case "Subtract":
		if len(a) == 1 {
			return -a[0], true, nil
		}
		return a[0] - a[1], true, nil
	case "Multiply":
		return reduce(a, func(x, y float64) float64 { return x * y }), true, nil
	case "Divide":
		return reduce(a, func(left, right float64) float64 { return left / right }), true, nil
	case "Mod":
		return reduce(a, func(left, right float64) float64 { return left - math.Floor(left/right)*right }), true, nil
	case "Rem":
		return reduce(a, math.Mod), true, nil
	case "Power":
		return reduce(a, math.Pow), true, nil
	case "Negate":
		return -a[0], true, nil
	case "Equal":
		return boolNumber(a[0] == a[1]), true, nil
	case "NotEqual":
		return boolNumber(a[0] != a[1]), true, nil
	case "Less":
		return boolNumber(a[0] < a[1]), true, nil
	case "LessOr":
		return boolNumber(a[0] <= a[1]), true, nil
	case "Greater":
		return boolNumber(a[0] > a[1]), true, nil
	case "GreaterOr":
		return boolNumber(a[0] >= a[1]), true, nil
	case "Not":
		return boolNumber(a[0] == 0 || math.IsNaN(a[0])), true, nil
	case "And":
		for _, v := range a {
			if v == 0 {
				return 0, true, nil
			}
		}
		return 1, true, nil
	case "Or":
		for _, v := range a {
			if v != 0 {
				return 1, true, nil
			}
		}
		return 0, true, nil
	case "Trunc":
		return math.Trunc(a[0]), true, nil
	case "Floor":
		return math.Floor(a[0]), true, nil
	case "Ceil":
		return math.Ceil(a[0]), true, nil
	case "Round":
		return jsRound(a[0]), true, nil
	case "Frac":
		return a[0] - math.Floor(a[0]), true, nil
	case "Sign":
		if math.IsNaN(a[0]) {
			return math.NaN(), true, nil
		}
		if a[0] == 0 {
			return a[0], true, nil
		}
		return math.Copysign(1, a[0]), true, nil
	case "Abs":
		return math.Abs(a[0]), true, nil
	case "Min":
		return fold(a, math.Inf(1), math.Min), true, nil
	case "Max":
		return fold(a, math.Inf(-1), math.Max), true, nil
	case "Clamp":
		return math.Max(a[1], math.Min(a[0], a[2])), true, nil
	case "Sin":
		return math.Sin(a[0]), true, nil
	case "Cos":
		return math.Cos(a[0]), true, nil
	case "Tan":
		return math.Tan(a[0]), true, nil
	case "Sinh":
		return math.Sinh(a[0]), true, nil
	case "Cosh":
		return math.Cosh(a[0]), true, nil
	case "Tanh":
		return math.Tanh(a[0]), true, nil
	case "Arcsin":
		return math.Asin(a[0]), true, nil
	case "Arccos":
		return math.Acos(a[0]), true, nil
	case "Arctan":
		return math.Atan(a[0]), true, nil
	case "Arctan2":
		return math.Atan2(a[0], a[1]), true, nil
	case "Degree":
		return a[0] * 180 / math.Pi, true, nil
	case "Radian":
		return a[0] * math.Pi / 180, true, nil
	case "Sqrt":
		return math.Sqrt(a[0]), true, nil
	case "Log":
		return math.Log(a[0]), true, nil
	case "Exp":
		return math.Exp(a[0]), true, nil
	case "Lerp", "LerpClamped":
		ratio := a[2]
		if name == "LerpClamped" {
			ratio = clamp(ratio, 0, 1)
		}
		return a[0]*(1-ratio) + a[1]*ratio, true, nil
	case "Unlerp", "UnlerpClamped":
		value := (a[2] - a[0]) / (a[1] - a[0])
		if name == "UnlerpClamped" {
			value = clamp(value, 0, 1)
		}
		return value, true, nil
	case "Remap", "RemapClamped":
		ratio := (a[4] - a[0]) / (a[1] - a[0])
		if name == "RemapClamped" {
			ratio = clamp(ratio, 0, 1)
		}
		return a[2]*(1-ratio) + a[3]*ratio, true, nil
	case "StackInit", "StackPush", "StackPop", "StackGrow", "StackEnter", "StackLeave", "StackGet", "StackSet", "StackGetFrame", "StackSetFrame", "StackGetPointer", "StackSetPointer", "StackGetFramePointer", "StackSetFramePointer":
		return e.stackBuiltin(name, a)
	case "Random":
		return a[0] + e.random.Float64()*(a[1]-a[0]), true, nil
	case "RandomInteger":
		minValue, err := integer("random minimum", a[0])
		if err != nil {
			return 0, true, err
		}
		maxValue, err := integer("random maximum", a[1])
		if err != nil {
			return 0, true, err
		}
		if maxValue <= minValue {
			return 0, true, fmt.Errorf("random maximum must exceed minimum")
		}
		return float64(minValue + e.random.Intn(maxValue-minValue)), true, nil
	case "StreamSet":
		stream, err := e.runtimeStream(a[0], a[1])
		if err != nil {
			return 0, true, err
		}
		stream.set(a[1], a[2])
		return 0, true, nil
	case "StreamHas":
		stream, err := e.runtimeStream(a[0], a[1])
		if err != nil {
			return 0, true, err
		}
		return boolNumber(stream.has(a[1])), true, nil
	case "StreamGetValue":
		stream, err := e.runtimeStream(a[0], a[1])
		if err != nil {
			return 0, true, err
		}
		return stream.value(a[1]), true, nil
	case "StreamGetPreviousKey", "StreamGetNextKey":
		stream, err := e.runtimeStream(a[0], a[1])
		if err != nil {
			return 0, true, err
		}
		if name == "StreamGetPreviousKey" {
			return stream.previous(a[1]), true, nil
		}
		return stream.next(a[1]), true, nil
	}
	if value, ok := easing(name, a); ok {
		return value, true, nil
	}
	if len(a) >= 3 && len(name) > 3 && name[:3] == "Set" {
		return e.setCompound(name, a)
	}
	return 0, false, nil
}

func (e *executor) setCompound(name string, a []float64) (float64, bool, error) {
	shifted := len(name) >= len("Shifted") && name[len(name)-len("Shifted"):] == "Shifted"
	pointed := len(name) >= len("Pointed") && name[len(name)-len("Pointed"):] == "Pointed"
	valueIndex := 2
	block, index, err := address2(a)
	if err != nil {
		return 0, true, err
	}
	op := name[3:]
	if shifted {
		valueIndex = 4
		block, index, err = shiftedAddress(a)
		if err != nil {
			return 0, true, err
		}
		op = op[:len(op)-7]
	} else if pointed {
		valueIndex = 3
		_, _, offset, addressErr := address3(a)
		if addressErr != nil {
			return 0, true, addressErr
		}
		blockValue, readErr := e.readIndex(block, index)
		if readErr != nil {
			return 0, true, readErr
		}
		indexValue, readErr := e.readIndex(block, index+1)
		if readErr != nil {
			return 0, true, readErr
		}
		block, err = integer("pointed block", blockValue)
		if err != nil {
			return 0, true, err
		}
		base, intErr := integer("pointed index", indexValue)
		if intErr != nil {
			return 0, true, intErr
		}
		index = base + offset
		op = op[:len(op)-7]
	}
	old, err := e.readIndex(block, index)
	if err != nil {
		return 0, true, err
	}
	value := a[valueIndex]
	switch op {
	case "Add":
		value = old + value
	case "Subtract":
		value = old - value
	case "Multiply":
		value = old * value
	case "Divide":
		value = old / value
	case "Mod":
		value = old - math.Floor(old/value)*value
	case "Rem":
		value = math.Mod(old, value)
	case "Power":
		value = math.Pow(old, value)
	default:
		return 0, false, nil
	}
	return e.writeIndex(block, index, value)
}

func (e *executor) read(blockValue, indexValue float64) (float64, bool, error) {
	block, err := integer("memory block", blockValue)
	if err != nil {
		return 0, true, err
	}
	index, err := memoryIndex(indexValue)
	if err != nil {
		return 0, true, err
	}
	value, err := e.readIndex(block, index)
	return value, true, err
}

func (e *executor) write(blockValue, indexValue, value float64) (float64, bool, error) {
	block, err := integer("memory block", blockValue)
	if err != nil {
		return 0, true, err
	}
	index, err := memoryIndex(indexValue)
	if err != nil {
		return 0, true, err
	}
	return e.writeIndex(block, index, value)
}

func (e *executor) pointedAddress(block, index, offset int) (int, int, error) {
	blockValue, err := e.readIndex(block, index)
	if err != nil {
		return 0, 0, err
	}
	indexValue, err := e.readIndex(block, index+1)
	if err != nil {
		return 0, 0, err
	}
	targetBlock, err := integer("pointed block", blockValue)
	if err != nil {
		return 0, 0, err
	}
	targetIndex, err := integer("pointed index", indexValue)
	if err != nil {
		return 0, 0, err
	}
	targetIndex += offset
	if err := validateMemoryIndex(targetIndex); err != nil {
		return 0, 0, err
	}
	return targetBlock, targetIndex, nil
}

func (e *executor) readPointed(block, index, offset int) (float64, bool, error) {
	targetBlock, targetIndex, err := e.pointedAddress(block, index, offset)
	if err != nil {
		return 0, true, err
	}
	value, err := e.readIndex(targetBlock, targetIndex)
	return value, true, err
}

func (e *executor) writePointed(block, index, offset int, value float64) (float64, bool, error) {
	targetBlock, targetIndex, err := e.pointedAddress(block, index, offset)
	if err != nil {
		return 0, true, err
	}
	return e.writeIndex(targetBlock, targetIndex, value)
}

func (e *executor) readIndex(block, index int) (float64, error) {
	if err := validateMemoryIndex(index); err != nil {
		return 0, err
	}
	return e.get(block, index), nil
}

func (e *executor) writeIndex(block, index int, value float64) (float64, bool, error) {
	if err := validateMemoryIndex(index); err != nil {
		return 0, true, err
	}
	e.set(block, index, value)
	return value, true, nil
}

func (e *executor) copy(a []float64) (float64, bool, error) {
	sourceBlock, err := integer("source block", a[0])
	if err != nil {
		return 0, true, err
	}
	sourceIndex, err := memoryIndex(a[1])
	if err != nil {
		return 0, true, err
	}
	targetBlock, err := integer("target block", a[2])
	if err != nil {
		return 0, true, err
	}
	targetIndex, err := memoryIndex(a[3])
	if err != nil {
		return 0, true, err
	}
	count, err := integer("copy count", a[4])
	if err != nil {
		return 0, true, err
	}
	if count < 0 {
		return 0, true, fmt.Errorf("copy count must be non-negative")
	}
	if count != 0 {
		if err := validateMemoryIndex(sourceIndex + count - 1); err != nil {
			return 0, true, err
		}
		if err := validateMemoryIndex(targetIndex + count - 1); err != nil {
			return 0, true, err
		}
	}
	values := make([]float64, count)
	for index := range count {
		values[index] = e.get(sourceBlock, sourceIndex+index)
	}
	for index, value := range values {
		e.set(targetBlock, targetIndex+index, value)
	}
	return 0, true, nil
}

func (e *executor) increment(name string, a []float64) (float64, bool, error) {
	delta := 1.0
	if len(name) >= len("Decrement") && name[:len("Decrement")] == "Decrement" {
		delta = -1
	}
	pre := name == "IncrementPre" || name == "IncrementPrePointed" || name == "IncrementPreShifted" ||
		name == "DecrementPre" || name == "DecrementPrePointed" || name == "DecrementPreShifted"
	block, index, err := address2(a)
	if err != nil {
		return 0, true, err
	}
	if len(name) >= len("Pointed") && name[len(name)-len("Pointed"):] == "Pointed" {
		offset, offsetErr := integer("pointed offset", a[2])
		if offsetErr != nil {
			return 0, true, offsetErr
		}
		block, index, err = e.pointedAddress(block, index, offset)
	} else if len(name) >= len("Shifted") && name[len(name)-len("Shifted"):] == "Shifted" {
		block, index, err = shiftedAddress(a)
	}
	if err != nil {
		return 0, true, err
	}
	old, err := e.readIndex(block, index)
	if err != nil {
		return 0, true, err
	}
	value := old + delta
	if _, _, err := e.writeIndex(block, index, value); err != nil {
		return 0, true, err
	}
	if pre {
		return value, true, nil
	}
	return old, true, nil
}

func address2(a []float64) (int, int, error) {
	block, err := integer("memory block", a[0])
	if err != nil {
		return 0, 0, err
	}
	index, err := memoryIndex(a[1])
	return block, index, err
}

func address3(a []float64) (int, int, int, error) {
	block, index, err := address2(a)
	if err != nil {
		return 0, 0, 0, err
	}
	offset, err := integer("pointed offset", a[2])
	return block, index, offset, err
}

func shiftedAddress(a []float64) (int, int, error) {
	block, err := integer("memory block", a[0])
	if err != nil {
		return 0, 0, err
	}
	offset, err := integer("memory offset", a[1])
	if err != nil {
		return 0, 0, err
	}
	index, err := integer("memory index", a[2])
	if err != nil {
		return 0, 0, err
	}
	stride, err := integer("memory stride", a[3])
	if err != nil {
		return 0, 0, err
	}
	result := offset + index*stride
	if err := validateMemoryIndex(result); err != nil {
		return 0, 0, err
	}
	return block, result, nil
}

func integer(name string, value float64) (int, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value != math.Trunc(value) {
		return 0, fmt.Errorf("%s must be a finite integer, got %v", name, value)
	}
	return int(value), nil
}

func memoryIndex(value float64) (int, error) {
	index, err := integer("memory index", value)
	if err != nil {
		return 0, err
	}
	return index, validateMemoryIndex(index)
}

func validateMemoryIndex(index int) error {
	if index < 0 {
		return fmt.Errorf("memory index %d must be non-negative", index)
	}
	if index > 65535 {
		return fmt.Errorf("memory index %d exceeds 65535", index)
	}
	return nil
}

func (e *executor) stackBuiltin(name string, a []float64) (float64, bool, error) {
	stackIndex := func(name string, value float64) (int, error) {
		index, err := integer(name, value)
		if err != nil {
			return 0, err
		}
		if err := validateMemoryIndex(index); err != nil {
			return 0, err
		}
		return index, nil
	}
	get := func(index int) float64 {
		if index >= len(e.stack) {
			return 0
		}
		return e.stack[index]
	}
	set := func(index int, value float64) {
		if index >= len(e.stack) {
			e.stack = append(e.stack, make([]float64, index-len(e.stack)+1)...)
		}
		e.stack[index] = value
	}
	switch name {
	case "StackInit":
		e.stack = nil
		e.stackPointer = 0
		e.framePointer = 0
		return 0, true, nil
	case "StackPush":
		if err := validateMemoryIndex(e.stackPointer); err != nil {
			return 0, true, err
		}
		set(e.stackPointer, a[0])
		e.stackPointer++
		return a[0], true, nil
	case "StackPop":
		if e.stackPointer <= 0 {
			return 0, true, fmt.Errorf("stack underflow")
		}
		e.stackPointer--
		return get(e.stackPointer), true, nil
	case "StackGrow":
		size, err := integer("stack growth", a[0])
		if err != nil {
			return 0, true, err
		}
		if size < 0 {
			return 0, true, fmt.Errorf("stack growth must be non-negative")
		}
		old := e.stackPointer
		if size != 0 {
			if err := validateMemoryIndex(e.stackPointer + size - 1); err != nil {
				return 0, true, err
			}
		}
		e.stackPointer += size
		return float64(old), true, nil
	case "StackEnter":
		if _, _, err := e.stackBuiltin("StackPush", []float64{float64(e.framePointer)}); err != nil {
			return 0, true, err
		}
		e.framePointer = e.stackPointer
		_, _, err := e.stackBuiltin("StackGrow", a)
		return 0, true, err
	case "StackLeave":
		if e.framePointer <= 0 || e.framePointer > e.stackPointer {
			return 0, true, fmt.Errorf("invalid stack frame pointer %d", e.framePointer)
		}
		e.stackPointer = e.framePointer
		value, _, err := e.stackBuiltin("StackPop", nil)
		if err != nil {
			return 0, true, err
		}
		pointer, err := stackIndex("stack frame pointer", value)
		if err != nil {
			return 0, true, err
		}
		e.framePointer = pointer
		return 0, true, nil
	case "StackGet", "StackSet", "StackGetFrame", "StackSetFrame":
		offset, err := integer("stack offset", a[0])
		if err != nil {
			return 0, true, err
		}
		base := e.stackPointer
		if name == "StackGetFrame" || name == "StackSetFrame" {
			base = e.framePointer
		}
		index, err := stackIndex("stack index", float64(base+offset))
		if err != nil {
			return 0, true, err
		}
		if name == "StackGet" || name == "StackGetFrame" {
			return get(index), true, nil
		}
		set(index, a[1])
		return a[1], true, nil
	case "StackGetPointer":
		return float64(e.stackPointer), true, nil
	case "StackGetFramePointer":
		return float64(e.framePointer), true, nil
	case "StackSetPointer", "StackSetFramePointer":
		pointer, err := stackIndex("stack pointer", a[0])
		if err != nil {
			return 0, true, err
		}
		if name == "StackSetPointer" {
			e.stackPointer = pointer
		} else {
			e.framePointer = pointer
		}
		return a[0], true, nil
	default:
		return 0, false, nil
	}
}

func (e *executor) get(block, index int) float64 {
	values := e.memory[block]
	if index < 0 || index >= len(values) {
		if e.defaultMemory != nil && block != 10000 {
			return *e.defaultMemory
		}
		return 0
	}
	return values[index]
}
func (e *executor) set(block, index int, value float64) {
	if index < 0 {
		return
	}
	values := e.memory[block]
	if index >= len(values) {
		values = append(values, make([]float64, index-len(values)+1)...)
	}
	values[index] = value
	e.memory[block] = values
}
func cloneMemory(value map[int][]float64) map[int][]float64 {
	result := map[int][]float64{}
	for block, values := range value {
		result[block] = append([]float64(nil), values...)
	}
	return result
}
func fold(values []float64, initial float64, fn func(float64, float64) float64) float64 {
	result := initial
	for _, value := range values {
		result = fn(result, value)
	}
	return result
}

func reduce(values []float64, fn func(float64, float64) float64) float64 {
	if len(values) == 0 {
		return 0
	}
	result := values[0]
	for _, value := range values[1:] {
		result = fn(result, value)
	}
	return result
}
func boolNumber(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func clamp(value, minValue, maxValue float64) float64 {
	return math.Max(minValue, math.Min(value, maxValue))
}

func jsRound(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value == 0 {
		return value
	}
	result := math.Floor(value + 0.5)
	if result == 0 && value < 0 {
		return math.Copysign(0, -1)
	}
	return result
}

func easing(name string, args []float64) (float64, bool) {
	if len(args) != 1 || len(name) < len("EaseInSine") || name[:len("Ease")] != "Ease" {
		return 0, false
	}
	rest := name[len("Ease"):]
	direction := ""
	for _, candidate := range []string{"InOut", "OutIn", "In", "Out"} {
		if len(rest) >= len(candidate) && rest[:len(candidate)] == candidate {
			direction = candidate
			rest = rest[len(candidate):]
			break
		}
	}
	if direction == "" {
		return 0, false
	}
	in := func(value float64) (float64, bool) {
		switch rest {
		case "Sine":
			return 1 - math.Cos(value*math.Pi/2), true
		case "Quad":
			return value * value, true
		case "Cubic":
			return value * value * value, true
		case "Quart":
			return value * value * value * value, true
		case "Quint":
			return value * value * value * value * value, true
		case "Expo":
			if value == 0 {
				return 0, true
			}
			return math.Pow(2, 10*value-10), true
		case "Circ":
			return 1 - math.Sqrt(1-value*value), true
		case "Back":
			const overshoot = 1.70158
			return (overshoot+1)*value*value*value - overshoot*value*value, true
		case "Elastic":
			if value == 0 || value == 1 {
				return value, true
			}
			return -math.Pow(2, 10*value-10) * math.Sin((10*value-10.75)*2*math.Pi/3), true
		default:
			return 0, false
		}
	}
	out := func(value float64) (float64, bool) {
		result, ok := in(1 - value)
		return 1 - result, ok
	}
	value := args[0]
	switch direction {
	case "In":
		return in(value)
	case "Out":
		return out(value)
	case "InOut":
		if value < 0.5 {
			result, ok := in(value * 2)
			return result / 2, ok
		}
		result, ok := in(2 - value*2)
		return 1 - result/2, ok
	case "OutIn":
		if value < 0.5 {
			result, ok := out(value * 2)
			return result / 2, ok
		}
		result, ok := in(value*2 - 1)
		return (result + 1) / 2, ok
	default:
		return 0, false
	}
}

type streamState struct {
	entries []StreamEntry
}

func normalizeStreams(input map[int][]StreamEntry) (map[int]*streamState, error) {
	result := make(map[int]*streamState, len(input))
	for id, entries := range input {
		if id < 0 || id > 65535 {
			return nil, fmt.Errorf("sim: invalid stream ID %d", id)
		}
		cloned := append([]StreamEntry(nil), entries...)
		sort.Slice(cloned, func(i, j int) bool { return cloned[i].Key < cloned[j].Key })
		for index, entry := range cloned {
			if math.IsNaN(entry.Key) || math.IsInf(entry.Key, 0) {
				return nil, fmt.Errorf("sim: stream %d has non-finite key", id)
			}
			if index != 0 && cloned[index-1].Key == entry.Key {
				return nil, fmt.Errorf("sim: stream %d has duplicate key %v", id, entry.Key)
			}
		}
		result[id] = &streamState{entries: cloned}
	}
	return result, nil
}

func cloneStreams(input map[int]*streamState) map[int][]StreamEntry {
	result := make(map[int][]StreamEntry, len(input))
	for id, stream := range input {
		result[id] = append([]StreamEntry(nil), stream.entries...)
	}
	return result
}

func (e *executor) stream(id int) *streamState {
	stream := e.streams[id]
	if stream == nil {
		stream = &streamState{}
		e.streams[id] = stream
	}
	return stream
}

func (e *executor) runtimeStream(idValue, key float64) (*streamState, error) {
	id, err := integer("stream ID", idValue)
	if err != nil {
		return nil, err
	}
	if id < 0 || id > 65535 {
		return nil, fmt.Errorf("stream ID %d is outside 0..65535", id)
	}
	if math.IsNaN(key) || math.IsInf(key, 0) {
		return nil, fmt.Errorf("stream key must be finite, got %v", key)
	}
	return e.stream(id), nil
}

func (s *streamState) search(key float64) (int, bool) {
	index := sort.Search(len(s.entries), func(index int) bool { return s.entries[index].Key >= key })
	return index, index < len(s.entries) && s.entries[index].Key == key
}

func (s *streamState) set(key, value float64) {
	index, found := s.search(key)
	if found {
		s.entries[index].Value = value
		return
	}
	s.entries = append(s.entries, StreamEntry{})
	copy(s.entries[index+1:], s.entries[index:])
	s.entries[index] = StreamEntry{Key: key, Value: value}
}

func (s *streamState) has(key float64) bool {
	_, found := s.search(key)
	return found
}

func (s *streamState) previous(key float64) float64 {
	index, _ := s.search(key)
	index--
	if index < 0 || index >= len(s.entries) {
		return key
	}
	return s.entries[index].Key
}

func (s *streamState) next(key float64) float64 {
	index, found := s.search(key)
	if found {
		index++
	}
	if index < 0 || index >= len(s.entries) {
		return key
	}
	return s.entries[index].Key
}

func (s *streamState) value(key float64) float64 {
	if len(s.entries) == 0 {
		return 0
	}
	index, found := s.search(key)
	if found {
		return s.entries[index].Value
	}
	if index == 0 {
		return s.entries[0].Value
	}
	if index == len(s.entries) {
		return s.entries[len(s.entries)-1].Value
	}
	left, right := s.entries[index-1], s.entries[index]
	ratio := (key - left.Key) / (right.Key - left.Key)
	return left.Value + (right.Value-left.Value)*ratio
}

type runtimeClass string

const (
	classPure    runtimeClass = "pure"
	classControl runtimeClass = "control"
	classMemory  runtimeClass = "memory"
	classRandom  runtimeClass = "random"
	classEffect  runtimeClass = "effect"
	classHandler runtimeClass = "handler"
)

func classifyRuntime(function resource.RuntimeFunction) runtimeClass {
	metadata, ok := catalog.LookupRuntimeSimulation(function)
	if !ok {
		return ""
	}
	return runtimeClass(metadata.Class)
}

func runtimeEffect(function resource.RuntimeFunction) bool {
	return classifyRuntime(function) == classEffect
}
