package frontend

import (
	"fmt"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/catalog"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
)

func validateRuntimeCalls(function *ir.Function) []error {
	var errs []error
	var expression func(ir.Expr)
	expression = func(expr ir.Expr) {
		switch value := expr.(type) {
		case ir.Load:
			switch place := value.Place.(type) {
			case ir.IndexedLocalPlace:
				expression(place.Index)
			case ir.MemoryPlace:
				expression(place.Index)
			}
		case ir.RuntimeCall:
			signature, ok := catalog.LookupRuntimeSignature(value.Function)
			if !ok {
				errs = append(errs, runtimeCallError(value, "RuntimeFunction %s has no native signature", value.Function))
			} else {
				count := len(value.Args)
				if count < signature.MinArgs || (signature.MaxArgs >= 0 && count > signature.MaxArgs) {
					expected := fmt.Sprintf("%d..%d", signature.MinArgs, signature.MaxArgs)
					if signature.MaxArgs < 0 {
						expected = fmt.Sprintf("at least %d", signature.MinArgs)
					} else if signature.MinArgs == signature.MaxArgs {
						expected = fmt.Sprintf("%d", signature.MinArgs)
					}
					errs = append(errs, runtimeCallError(value, "RuntimeFunction %s received %d arguments; expected %s", value.Function, count, expected))
				}
				if value.Result.Slots != signature.ResultSlots {
					errs = append(errs, runtimeCallError(value, "RuntimeFunction %s returns %d slots; native signature returns %d", value.Function, value.Result.Slots, signature.ResultSlots))
				}
			}
			for _, arg := range value.Args {
				expression(arg)
			}
		}
	}
	for _, block := range function.Blocks {
		for _, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				expression(value.Value)
			case ir.Eval:
				expression(value.Value)
			}
		}
		switch term := block.Terminator.(type) {
		case ir.Branch:
			expression(term.Condition)
		case ir.Switch:
			expression(term.Value)
		case ir.Return:
			for _, slot := range term.Value.Slots {
				expression(slot)
			}
		}
	}
	return errs
}

func runtimeCallError(call ir.RuntimeCall, format string, args ...any) error {
	message := fmt.Sprintf(format, args...)
	if call.Pos.File == "" {
		return fmt.Errorf("%s", message)
	}
	return fmt.Errorf("%s:%d:%d: %s", call.Pos.File, call.Pos.Line, call.Pos.Column, message)
}
