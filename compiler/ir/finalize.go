package ir

import (
	"fmt"
	"math"

	"github.com/WindowsSov8forUs/sonolus-core-go/core/resource"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

// Ops used during finalization.
const (
	opAdd                      = resource.RuntimeFunctionAdd
	opGet                      = resource.RuntimeFunctionGet
	opSet                      = resource.RuntimeFunctionSet
	opIf                       = resource.RuntimeFunctionIf
	opEqual                    = resource.RuntimeFunctionEqual
	opExecute                  = resource.RuntimeFunctionExecute
	opBlock                    = resource.RuntimeFunctionBlock
	opJumpLoop                 = resource.RuntimeFunctionJumpLoop
	opSwitchWithDefault        = resource.RuntimeFunctionSwitchWithDefault
	opSwitchIntegerWithDefault = resource.RuntimeFunctionSwitchIntegerWithDefault
)

// romBlock is the EngineRom memory block; non-finite constants are read from it.
const romBlock = 3000

// CFGToSNode lowers an (optimized) CFG into a single snode.SNode tree, encoding
// inter-block control flow as a Block(JumpLoop(...)) of per-block Execute nodes.
// Port of sonolus.py finalize.cfg_to_engine_node.
func CFGToSNode(entry *BasicBlock) snode.SNode {
	order := traverseReversePostorder(entry)
	index := make(map[*BasicBlock]int, len(order))
	for i, b := range order {
		index[b] = i
	}
	exit := len(order)

	blockStatements := make([]snode.SNode, 0, len(order)+1)
	for _, b := range order {
		stmts := make([]snode.SNode, 0, len(b.Statements)+1)
		for _, s := range b.Statements {
			stmts = append(stmts, Lower(s))
		}
		stmts = append(stmts, controlFlow(b, index, exit))
		blockStatements = append(blockStatements, snode.Call(opExecute, stmts...))
	}
	blockStatements = append(blockStatements, snode.Val(0))

	return snode.Call(opBlock, snode.Call(opJumpLoop, blockStatements...))
}

// condEdge is one non-default outgoing edge.
type condEdge struct {
	cond float64
	dst  *BasicBlock
}

// controlFlow encodes a block's outgoing edges as the trailing node of its
// Execute body: a fallthrough index, an If, or a Switch.
func controlFlow(b *BasicBlock, index map[*BasicBlock]int, exit int) snode.SNode {
	edges := sortedOutgoing(b)

	var defaultDst *BasicBlock
	var conds []condEdge
	for _, e := range edges {
		if e.Cond == nil {
			defaultDst = e.Dst
		} else {
			conds = append(conds, condEdge{*e.Cond, e.Dst})
		}
	}

	switch {
	case len(edges) == 0:
		// No successors: jump past the last block to terminate the loop.
		return snode.Val(float64(exit))

	case len(conds) == 0 && defaultDst != nil:
		// Unconditional edge.
		return snode.Val(float64(index[defaultDst]))

	case defaultDst != nil && len(conds) == 1 && conds[0].cond == 0:
		// {0: false, default: true} -> If(test, true, false).
		return snode.Call(opIf,
			Lower(b.Test),
			snode.Val(float64(index[defaultDst])),
			snode.Val(float64(index[conds[0].dst])),
		)

	case defaultDst != nil && len(conds) == 1:
		// {cond: branch, default} -> If(test == cond, branch, default).
		return snode.Call(opIf,
			Lower(PureInstr(opEqual, b.Test, Const(conds[0].cond))),
			snode.Val(float64(index[conds[0].dst])),
			snode.Val(float64(index[defaultDst])),
		)

	default:
		return switchNode(b.Test, conds, defaultDst, index, exit)
	}
}

// switchNode emits SwitchIntegerWithDefault when the conditions are a dense
// 0..n-1 integer range, otherwise SwitchWithDefault. Conditions arrive sorted
// ascending (default handled separately).
func switchNode(test Node, conds []condEdge, defaultDst *BasicBlock, index map[*BasicBlock]int, exit int) snode.SNode {
	dense := len(conds) > 0
	for i, c := range conds {
		if c.cond != float64(i) {
			dense = false
			break
		}
	}

	def := exit
	if defaultDst != nil {
		def = index[defaultDst]
	}

	if dense {
		args := make([]snode.SNode, 0, len(conds)+2)
		args = append(args, Lower(test))
		for _, c := range conds {
			args = append(args, snode.Val(float64(index[c.dst])))
		}
		args = append(args, snode.Val(float64(def)))
		return snode.Call(opSwitchIntegerWithDefault, args...)
	}

	args := make([]snode.SNode, 0, len(conds)*2+2)
	args = append(args, Lower(test))
	for _, c := range conds {
		args = append(args, snode.Val(c.cond), snode.Val(float64(index[c.dst])))
	}
	args = append(args, snode.Val(float64(def)))
	return snode.Call(opSwitchWithDefault, args...)
}

// Lower converts a single IR node into an snode.SNode. Port of
// finalize.ir_to_engine_node.
func Lower(n Node) snode.SNode {
	switch t := n.(type) {
	case Const:
		return numeric(float64(t))
	case Instr:
		args := make([]snode.SNode, len(t.Args))
		for i, a := range t.Args {
			args[i] = Lower(a)
		}
		return snode.Call(t.Op, args...)
	case Get:
		return Lower(t.Place)
	case Set:
		place, ok := Lower(t.Place).(snode.Func)
		if !ok {
			panic("ir: Set place did not lower to a function node")
		}
		args := append(append([]snode.SNode{}, place.Args...), Lower(t.Value))
		return snode.Call(opSet, args...)
	case BlockPlace:
		return blockPlaceToSNode(t)
	case *TempBlock:
		panic("ir: TempBlock reached finalize (AllocateTempBlocks not run)")
	case SSAPlace:
		panic("ir: SSAPlace reached finalize (register allocation not run)")
	case nil:
		panic("ir: cannot lower nil node")
	default:
		panic(fmt.Sprintf("ir: cannot lower %T", n))
	}
}

// numeric lowers a numeric constant, reading non-finite values from ROM
// (mirrors finalize._numeric_to_engine_node).
func numeric(v float64) snode.SNode {
	switch {
	case math.IsInf(v, 1):
		return snode.Call(opGet, snode.Val(romBlock), snode.Val(1))
	case math.IsInf(v, -1):
		return snode.Call(opGet, snode.Val(romBlock), snode.Val(2))
	case math.IsNaN(v):
		return snode.Call(opGet, snode.Val(romBlock), snode.Val(0))
	default:
		return snode.Val(v)
	}
}

func blockPlaceToSNode(p BlockPlace) snode.SNode {
	var index snode.SNode
	switch {
	case p.Offset == 0:
		index = Lower(orZero(p.Index))
	case isZeroIndex(p.Index):
		index = snode.Val(float64(p.Offset))
	default:
		index = snode.Call(opAdd, Lower(p.Index), snode.Val(float64(p.Offset)))
	}
	return snode.Call(opGet, Lower(orZero(p.Block)), index)
}

func orZero(n Node) Node {
	if n == nil {
		return Const(0)
	}
	return n
}

func isZeroIndex(n Node) bool {
	if n == nil {
		return true
	}
	c, ok := n.(Const)
	return ok && float64(c) == 0
}
