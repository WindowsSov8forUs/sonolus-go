package frontend

import (
	"fmt"
	"sort"

	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/ir"
	"github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/mode"
)

type diagnosticSite struct {
	modeOrder, ownerOrder int
	owner, callback       string
	block, instruction    int
	message               string
}

func ResolveDiagnostics(project *Project) error {
	if project == nil {
		return fmt.Errorf("diagnostics: project is nil")
	}
	sites := collectDiagnosticSites(project)
	sort.SliceStable(sites, func(left, right int) bool {
		a, b := sites[left], sites[right]
		if a.modeOrder != b.modeOrder {
			return a.modeOrder < b.modeOrder
		}
		if a.ownerOrder != b.ownerOrder {
			return a.ownerOrder < b.ownerOrder
		}
		if a.owner != b.owner {
			return a.owner < b.owner
		}
		if a.callback != b.callback {
			return a.callback < b.callback
		}
		if a.block != b.block {
			return a.block < b.block
		}
		if a.instruction != b.instruction {
			return a.instruction < b.instruction
		}
		return a.message < b.message
	})
	codes := make(map[string]int, len(sites))
	for _, site := range sites {
		if _, exists := codes[site.message]; !exists {
			codes[site.message] = len(codes) + 1
		}
	}
	for _, currentMode := range []mode.Mode{mode.ModePlay, mode.ModeWatch, mode.ModePreview, mode.ModeTutorial} {
		declarations := project.Modes[currentMode]
		if declarations == nil {
			continue
		}
		for _, callback := range modeCallbacks(declarations) {
			if callback == nil || callback.IR == nil {
				continue
			}
			callback.IR.Diagnostics = map[int]string{}
			resolveFunctionDiagnostics(callback.IR, codes)
		}
	}
	return nil
}

func collectDiagnosticSites(project *Project) []diagnosticSite {
	var sites []diagnosticSite
	for modeOrder, currentMode := range []mode.Mode{mode.ModePlay, mode.ModeWatch, mode.ModePreview, mode.ModeTutorial} {
		declarations := project.Modes[currentMode]
		if declarations == nil {
			continue
		}
		collect := func(ownerOrder int, owner string, callback *CallbackDeclaration) {
			if callback == nil || callback.IR == nil {
				return
			}
			for blockIndex, block := range diagnosticBlockOrder(callback.IR) {
				for instructionIndex, instruction := range block.Instructions {
					collectInstructionDiagnostics(instruction, func(message string) {
						sites = append(sites, diagnosticSite{modeOrder, ownerOrder, owner, callback.Name, blockIndex, instructionIndex, message})
					})
				}
				collectTerminatorDiagnostics(block.Terminator, func(message string) {
					sites = append(sites, diagnosticSite{modeOrder, ownerOrder, owner, callback.Name, blockIndex, len(block.Instructions), message})
				})
			}
		}
		for _, callback := range declarations.Globals {
			collect(0, "", callback)
		}
		for _, archetype := range declarations.Archetypes {
			for _, callback := range archetype.Callbacks {
				collect(1, archetype.Name, callback)
			}
		}
	}
	return sites
}

func modeCallbacks(declarations *ModeDeclarations) []*CallbackDeclaration {
	callbacks := append([]*CallbackDeclaration(nil), declarations.Globals...)
	for _, archetype := range declarations.Archetypes {
		callbacks = append(callbacks, archetype.Callbacks...)
	}
	return callbacks
}

func collectInstructionDiagnostics(instruction ir.Instruction, add func(string)) {
	switch value := instruction.(type) {
	case ir.Store:
		collectPlaceDiagnostics(value.Place, add)
		collectExprDiagnostics(value.Value, add)
	case ir.Eval:
		collectExprDiagnostics(value.Value, add)
	}
}

func collectTerminatorDiagnostics(terminator ir.Terminator, add func(string)) {
	switch value := terminator.(type) {
	case ir.Branch:
		collectExprDiagnostics(value.Condition, add)
	case ir.Switch:
		collectExprDiagnostics(value.Value, add)
	case ir.Return:
		for _, slot := range value.Value.Slots {
			collectExprDiagnostics(slot, add)
		}
	}
}

func collectExprDiagnostics(expression ir.Expr, add func(string)) {
	switch value := expression.(type) {
	case ir.Load:
		collectPlaceDiagnostics(value.Place, add)
	case ir.RuntimeCall:
		for _, argument := range value.Args {
			collectExprDiagnostics(argument, add)
		}
		if value.Diagnostic != "" {
			add(value.Diagnostic)
		}
	}
}

func collectPlaceDiagnostics(place ir.Place, add func(string)) {
	switch value := place.(type) {
	case ir.IndexedLocalPlace:
		collectExprDiagnostics(value.Index, add)
	case ir.MemoryPlace:
		collectExprDiagnostics(value.Index, add)
	}
}

func resolveFunctionDiagnostics(function *ir.Function, codes map[string]int) {
	for _, block := range diagnosticBlockOrder(function) {
		for index, instruction := range block.Instructions {
			switch value := instruction.(type) {
			case ir.Store:
				value.Place = resolveDiagnosticPlace(value.Place, function, codes)
				value.Value = resolveDiagnosticExpr(value.Value, function, codes)
				block.Instructions[index] = value
			case ir.Eval:
				value.Value = resolveDiagnosticExpr(value.Value, function, codes)
				block.Instructions[index] = value
			}
		}
		switch value := block.Terminator.(type) {
		case ir.Branch:
			value.Condition = resolveDiagnosticExpr(value.Condition, function, codes)
			block.Terminator = value
		case ir.Switch:
			value.Value = resolveDiagnosticExpr(value.Value, function, codes)
			block.Terminator = value
		case ir.Return:
			for index, slot := range value.Value.Slots {
				value.Value.Slots[index] = resolveDiagnosticExpr(slot, function, codes)
			}
			block.Terminator = value
		}
	}
}

func resolveDiagnosticExpr(expression ir.Expr, function *ir.Function, codes map[string]int) ir.Expr {
	switch value := expression.(type) {
	case ir.Load:
		value.Place = resolveDiagnosticPlace(value.Place, function, codes)
		return value
	case ir.RuntimeCall:
		for index, argument := range value.Args {
			value.Args[index] = resolveDiagnosticExpr(argument, function, codes)
		}
		if value.Diagnostic != "" {
			code, exists := codes[value.Diagnostic]
			if exists && len(value.Args) == 1 {
				value.Args[0] = ir.Const{Value: float64(code)}
				function.Diagnostics[code] = value.Diagnostic
				value.Diagnostic = ""
			}
		}
		return value
	default:
		return expression
	}
}

func resolveDiagnosticPlace(place ir.Place, function *ir.Function, codes map[string]int) ir.Place {
	switch value := place.(type) {
	case ir.IndexedLocalPlace:
		value.Index = resolveDiagnosticExpr(value.Index, function, codes)
		return value
	case ir.MemoryPlace:
		value.Index = resolveDiagnosticExpr(value.Index, function, codes)
		return value
	default:
		return place
	}
}

func diagnosticBlockOrder(function *ir.Function) []*ir.Block {
	seen := map[int]bool{}
	postorder := make([]*ir.Block, 0, len(function.Blocks))
	var visit func(int)
	visit = func(id int) {
		if id < 0 || id >= len(function.Blocks) || seen[id] || function.Blocks[id] == nil {
			return
		}
		seen[id] = true
		block := function.Blocks[id]
		for _, target := range diagnosticTargets(block.Terminator) {
			visit(target)
		}
		postorder = append(postorder, block)
	}
	visit(function.Entry)
	for id := range function.Blocks {
		visit(id)
	}
	for left, right := 0, len(postorder)-1; left < right; left, right = left+1, right-1 {
		postorder[left], postorder[right] = postorder[right], postorder[left]
	}
	return postorder
}

func diagnosticTargets(terminator ir.Terminator) []int {
	switch value := terminator.(type) {
	case ir.Jump:
		return []int{value.Target}
	case ir.Branch:
		return []int{value.True, value.False}
	case ir.Switch:
		result := make([]int, 0, len(value.Cases)+1)
		for _, item := range value.Cases {
			result = append(result, item.Target)
		}
		return append(result, value.Default)
	default:
		return nil
	}
}
