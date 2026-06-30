package ir

import (
	"fmt"
	"math"

	"github.com/WindowsSov8forUs/sonolus-go/compiler/snode"
)

// BlockEngineRom (defined in ir.go) is the EngineRom memory block; non-finite
// constants are read from it.

// CFGToSNode lowers an (optimized) CFG into a single snode.SNode tree, encoding
// inter-block control flow as a Block(JumpLoop(...)) of per-block Execute nodes.
// Port of sonolus.py finalize.cfg_to_engine_node.
func CFGToSNode(gen *IDGen, entry *BasicBlock) (snode.SNode, error) {
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
			lowered, err := Lower(s)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, lowered)
		}
		cf, err := controlFlow(gen, b, index, exit)
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, cf)
		blockStatements = append(blockStatements, snode.Call(OpExecute, stmts...))
	}
	blockStatements = append(blockStatements, snode.Val(0))

	return snode.Call(OpBlock, snode.Call(OpJumpLoop, blockStatements...)), nil
}

// condEdge is one non-default outgoing edge.
type condEdge struct {
	cond float64
	dst  *BasicBlock
}

// controlFlow encodes a block's outgoing edges as the trailing node of its
// Execute body: a fallthrough index, an If, or a Switch.
func controlFlow(gen *IDGen, b *BasicBlock, index map[*BasicBlock]int, exit int) (snode.SNode, error) {
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
		return snode.Val(float64(exit)), nil

	case len(conds) == 0 && defaultDst != nil:
		return snode.Val(float64(index[defaultDst])), nil

	case defaultDst != nil && len(conds) == 1 && conds[0].cond == 0:
		test, err := Lower(b.Test)
		if err != nil {
			return nil, err
		}
		return snode.Call(OpIf,
			test,
			snode.Val(float64(index[defaultDst])),
			snode.Val(float64(index[conds[0].dst])),
		), nil

	case defaultDst != nil && len(conds) == 1:
		test, err := Lower(gen.PureInstr(OpEqual, b.Test, Const(conds[0].cond)))
		if err != nil {
			return nil, err
		}
		return snode.Call(OpIf,
			test,
			snode.Val(float64(index[conds[0].dst])),
			snode.Val(float64(index[defaultDst])),
		), nil

	default:
		return switchNode(b.Test, conds, defaultDst, index, exit)
	}
}

// switchNode emits SwitchIntegerWithDefault when the conditions are a dense
// 0..n-1 integer range, otherwise SwitchWithDefault. Conditions arrive sorted
// ascending (default handled separately).
func switchNode(test Node, conds []condEdge, defaultDst *BasicBlock, index map[*BasicBlock]int, exit int) (snode.SNode, error) {
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
		lowered, err := Lower(test)
		if err != nil {
			return nil, err
		}
		args := make([]snode.SNode, 0, len(conds)+2)
		args = append(args, lowered)
		for _, c := range conds {
			args = append(args, snode.Val(float64(index[c.dst])))
		}
		args = append(args, snode.Val(float64(def)))
		return snode.Call(OpSwitchIntegerWithDefault, args...), nil
	}

	lowered, err := Lower(test)
	if err != nil {
		return nil, err
	}
	args := make([]snode.SNode, 0, len(conds)*2+2)
	args = append(args, lowered)
	for _, c := range conds {
		args = append(args, snode.Val(c.cond), snode.Val(float64(index[c.dst])))
	}
	args = append(args, snode.Val(float64(def)))
	return snode.Call(OpSwitchWithDefault, args...), nil
}

// Lower converts a single IR node into an snode.SNode. Port of
// finalize.ir_to_engine_node.
func Lower(n Node) (snode.SNode, error) {
	switch t := n.(type) {
	case Const:
		return numeric(float64(t)), nil
	case Instr:
		args := make([]snode.SNode, len(t.Args))
		for i, a := range t.Args {
			var err error
			args[i], err = Lower(a)
			if err != nil {
				return nil, err
			}
		}
		return snode.Call(t.Op, args...), nil
	case Get:
		return Lower(t.Place)
	case Set:
		place, err := Lower(t.Place)
		if err != nil {
			return nil, err
		}
		placeFunc, ok := place.(snode.Func)
		if !ok {
			return nil, fmt.Errorf("ir: Set place did not lower to a function node")
		}
		val, err := Lower(t.Value)
		if err != nil {
			return nil, err
		}
		args := append(append([]snode.SNode{}, placeFunc.Args...), val)
		return snode.Call(OpSet, args...), nil
	case BlockPlace:
		return blockPlaceToSNode(t)
	case *TempBlock:
		return nil, fmt.Errorf("ir: TempBlock reached finalize (AllocateTempBlocks not run)")
	case SSAPlace:
		return nil, fmt.Errorf("ir: SSAPlace reached finalize (register allocation not run)")
	case nil:
		return nil, fmt.Errorf("ir: cannot lower nil node")
	default:
		return nil, fmt.Errorf("ir: cannot lower %T", n)
	}
}

// numeric lowers a numeric constant, reading non-finite values from ROM
// (mirrors finalize._numeric_to_engine_node).
func numeric(v float64) snode.SNode {
	switch {
	case math.IsInf(v, 1):
		return snode.Call(OpGet, snode.Val(BlockEngineRom), snode.Val(1))
	case math.IsInf(v, -1):
		return snode.Call(OpGet, snode.Val(BlockEngineRom), snode.Val(2))
	case math.IsNaN(v):
		return snode.Call(OpGet, snode.Val(BlockEngineRom), snode.Val(0))
	default:
		return snode.Val(v)
	}
}

func blockPlaceToSNode(p BlockPlace) (snode.SNode, error) {
	var index snode.SNode
	switch {
	case p.Offset == 0:
		var err error
		index, err = Lower(orZero(p.Index))
		if err != nil {
			return nil, err
		}
	case isZeroIndex(p.Index):
		index = snode.Val(float64(p.Offset))
	default:
		pi, err := Lower(p.Index)
		if err != nil {
			return nil, err
		}
		index = snode.Call(OpAdd, pi, snode.Val(float64(p.Offset)))
	}
	blk, err := Lower(orZero(p.Block))
	if err != nil {
		return nil, err
	}
	return snode.Call(OpGet, blk, index), nil
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
