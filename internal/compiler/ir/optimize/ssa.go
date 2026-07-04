package optimize

import (
	"strconv"

	"github.com/WindowsSov8forUs/sonolus-go/internal/compiler/ir"
)

// undefVarName is used as a placeholder SSA name when a variable is accessed
// in unreachable code. The "#" prefix avoids collisions with user-defined names.
const undefVarName = "#undef"

// ToSSA converts size-1 temp-block variables into SSA form: it inserts phi nodes
// at the iterated dominance frontiers of each variable's definitions, then
// renames definitions and uses into versioned SSA places. Port of sonolus.py
// ssa.ToSSA.
type ToSSA struct{}

func (ToSSA) Name() string { return "ToSSA" }

func (ToSSA) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	return (ToSSA{}).RunWithDom(gen, entry, &DominanceCache{})
}

func (ToSSA) RunWithDom(gen *ir.IDGen, entry *ir.BasicBlock, dc *DominanceCache) *ir.BasicBlock {
	dom := dc.Get(entry)

	order, defBlocks := defsToBlocks(entry)
	insertPhis(order, defBlocks, dom)

	r := &renamer{
		dom:   dom,
		stack: map[*ir.TempBlock][]ir.SSAPlace{},
		used:  map[string]int{},
	}
	r.rename(entry)
	return entry
}

// defsToBlocks returns the temps defined in the CFG (in first-seen preorder) and
// the set of blocks defining each.
func defsToBlocks(entry *ir.BasicBlock) ([]*ir.TempBlock, map[*ir.TempBlock]map[*ir.BasicBlock]bool) {
	var order []*ir.TempBlock
	blocks := map[*ir.TempBlock]map[*ir.BasicBlock]bool{}
	for _, b := range ir.Preorder(entry) {
		for _, s := range b.Statements {
			tb := stmtDef(s)
			if tb == nil {
				continue
			}
			if _, ok := blocks[tb]; !ok {
				blocks[tb] = map[*ir.BasicBlock]bool{}
				order = append(order, tb)
			}
			blocks[tb][b] = true
		}
	}
	return order, blocks
}

// stmtDef returns the size-1 temp block a statement defines, if any.
func stmtDef(s ir.Node) *ir.TempBlock {
	set, ok := s.(ir.Set)
	if !ok {
		return nil
	}
	bp, ok := set.Place.(ir.BlockPlace)
	if !ok {
		return nil
	}
	tb, _ := blockTemp(bp)
	return tb
}

// insertPhis places empty phi nodes for each variable at its iterated dominance
// frontier. Phis are appended in variable first-seen order so per-block phi
// order is deterministic.
func insertPhis(order []*ir.TempBlock, defBlocks map[*ir.TempBlock]map[*ir.BasicBlock]bool, dom *Dominance) {
	for _, tb := range order {
		for b := range iteratedDF(defBlocks[tb], dom) {
			b.Phis = append(b.Phis, &ir.Phi{Var: tb, Args: map[*ir.BasicBlock]ir.Place{}})
		}
	}
}

func iteratedDF(blocks map[*ir.BasicBlock]bool, dom *Dominance) map[*ir.BasicBlock]bool {
	result := map[*ir.BasicBlock]bool{}
	worklist := make([]*ir.BasicBlock, 0, len(blocks))
	for b := range blocks {
		worklist = append(worklist, b)
	}
	for len(worklist) > 0 {
		b := worklist[len(worklist)-1]
		worklist = worklist[:len(worklist)-1]
		for f := range dom.DF[b] {
			if !result[f] {
				result[f] = true
				worklist = append(worklist, f)
			}
		}
	}
	return result
}

type renamer struct {
	dom   *Dominance
	stack map[*ir.TempBlock][]ir.SSAPlace
	used  map[string]int
}

func (r *renamer) newSSA(name string) ir.SSAPlace {
	r.used[name]++
	return ir.SSAPlace{Name: name, Num: r.used[name]}
}

func (r *renamer) top(tb *ir.TempBlock) (ir.SSAPlace, bool) {
	st := r.stack[tb]
	if len(st) == 0 {
		return ir.SSAPlace{}, false
	}
	return st[len(st)-1], true
}

func (r *renamer) rename(block *ir.BasicBlock) {
	var toPop []*ir.TempBlock

	// Phi targets define a fresh SSA value for their variable.
	for _, phi := range block.Phis {
		np := r.newSSA(phi.Var.Name)
		r.stack[phi.Var] = append(r.stack[phi.Var], np)
		toPop = append(toPop, phi.Var)
		phi.Target = np
	}

	for i, s := range block.Statements {
		block.Statements[i] = r.renameNode(s, &toPop)
	}

	// Supply this block's SSA values as phi arguments in successors.
	for _, e := range block.Outgoing {
		for _, phi := range e.Dst.Phis {
			if v, ok := r.top(phi.Var); ok {
				phi.Args[block] = v
			}
		}
	}

	block.Test = r.renameNode(block.Test, &toPop)

	for _, c := range r.dom.DomChildren[block] {
		r.rename(c)
	}

	for _, tb := range toPop {
		r.stack[tb] = r.stack[tb][:len(r.stack[tb])-1]
	}
}

// renameNode rewrites temp references into their current SSA values, and pushes
// a fresh SSA value for a temp definition (IRSet to a size-1 temp).
func (r *renamer) renameNode(n ir.Node, toPop *[]*ir.TempBlock) ir.Node {
	switch t := n.(type) {
	case nil, ir.Const, ir.SSAPlace:
		return n
	case ir.Instr:
		args := make([]ir.Node, len(t.Args))
		for i, a := range t.Args {
			args[i] = r.renameNode(a, toPop)
		}
		return ir.Instr{ID: t.ID, Op: t.Op, Args: args, Pure: t.Pure}
	case ir.Get:
		return ir.Get{Place: r.renamePlace(t.Place, toPop)}
	case ir.Set:
		value := r.renameNode(t.Value, toPop)
		if tb, ok := tempOf(t.Place); ok {
			np := r.newSSA(tb.Name)
			r.stack[tb] = append(r.stack[tb], np)
			*toPop = append(*toPop, tb)
		}
		return ir.Set{ID: t.ID, Place: r.renamePlace(t.Place, toPop), Value: value}
	case *ir.TempBlock:
		// Only size-1 temps are SSA variables; larger temps (arrays/records)
		// stay as memory blocks.
		if t.Size != 1 {
			return t
		}
		if v, ok := r.top(t); ok {
			return v
		}
		// Access to a definitely-undefined variable (may be unreachable).
		return ir.SSAPlace{Name: undefVarName, Num: 0}
	case ir.BlockPlace:
		return r.renamePlace(t, toPop)
	default:
		return n
	}
}

// renamePlace returns the SSA value for a size-1 temp place, or a rewritten
// BlockPlace for concrete memory (whose block/index may reference temps).
func (r *renamer) renamePlace(p ir.Place, toPop *[]*ir.TempBlock) ir.Place {
	bp, ok := p.(ir.BlockPlace)
	if !ok {
		return p // SSAPlace
	}
	if tb, ok := bp.Block.(*ir.TempBlock); ok && tb.Size == 1 {
		if v, ok := r.top(tb); ok {
			return v
		}
		return ir.SSAPlace{Name: undefVarName, Num: 0}
	}
	return ir.BlockPlace{
		Block:  r.renameNode(bp.Block, toPop),
		Index:  r.renameNode(bp.Index, toPop),
		Offset: bp.Offset,
	}
}

// FromSSA destroys SSA form: it splits each phi-carrying block's incoming edges
// with a "between" block, materializes phis as copies on those edges, and maps
// each SSA value back to a temp block named "name.num". Port of sonolus.py
// ssa.FromSSA. allocateTempBlocks must run afterward before finalization.
type FromSSA struct{}

func (FromSSA) Name() string { return "FromSSA" }

func (FromSSA) Run(gen *ir.IDGen, entry *ir.BasicBlock) *ir.BasicBlock {
	f := &fromSSAState{temps: map[string]*ir.TempBlock{}, gen: gen}
	for _, b := range ir.Preorder(entry) {
		f.processBlock(b)
	}
	return entry
}

// fromSSAState carries the temp-name cache so that values written and read
// across edges resolve to the same (pointer-identified) temp.
type fromSSAState struct {
	gen   *ir.IDGen
	temps map[string]*ir.TempBlock
}

func (f *fromSSAState) tempFor(name string) *ir.TempBlock {
	if t, ok := f.temps[name]; ok {
		return t
	}
	t := ir.NewTemp(name)
	f.temps[name] = t
	return t
}

func (f *fromSSAState) placeFromSSA(s ir.SSAPlace, suffix string) ir.BlockPlace {
	name := s.Name + "." + strconv.Itoa(s.Num) + suffix
	return ir.BlockPlace{Block: f.tempFor(name), Index: ir.Const(0), Offset: 0}
}

func (f *fromSSAState) processBlock(block *ir.BasicBlock) {
	origIncoming := append([]*ir.FlowEdge(nil), block.Incoming...)
	block.Incoming = nil

	for _, edge := range origIncoming {
		between := ir.NewBlock()
		edge.Dst = between
		between.Incoming = append(between.Incoming, edge)
		next := &ir.FlowEdge{Src: between, Dst: block, Cond: nil}
		block.Incoming = append(block.Incoming, next)
		between.Outgoing = append(between.Outgoing, next)
		for _, phi := range block.Phis {
			if arg, ok := phi.Args[edge.Src]; ok {
				phi.Args[between] = arg
			}
		}
	}
	for _, edge := range origIncoming {
		for _, phi := range block.Phis {
			delete(phi.Args, edge.Src)
		}
	}

	incomingBlocks := map[*ir.BasicBlock]bool{}
	for _, e := range block.Incoming {
		incomingBlocks[e.Src] = true
	}
	// For each predecessor, the set of SSA values it supplies as phi args.
	argsBySrc := map[*ir.BasicBlock]map[ir.SSAPlace]bool{}
	for _, phi := range block.Phis {
		for src, arg := range phi.Args {
			if argsBySrc[src] == nil {
				argsBySrc[src] = map[ir.SSAPlace]bool{}
			}
			argsBySrc[src][arg.(ir.SSAPlace)] = true
		}
	}

	// Break swap cycles: copy values that another assignment may overwrite.
	for _, phi := range block.Phis {
		if phi.Target == nil {
			continue
		}
		target := phi.Target.(ir.SSAPlace)
		for src, arg := range phi.Args {
			if !incomingBlocks[src] {
				continue
			}
			if argsBySrc[src][target] {
				src.Statements = append(src.Statements,
					f.gen.SetPlace(f.placeFromSSA(target, "*"), ir.GetPlace(f.placeFromSSA(arg.(ir.SSAPlace), ""))))
			}
		}
	}
	for _, phi := range block.Phis {
		if phi.Target == nil {
			continue
		}
		target := phi.Target.(ir.SSAPlace)
		for src, arg := range phi.Args {
			if !incomingBlocks[src] {
				continue
			}
			if argsBySrc[src][target] {
				src.Statements = append(src.Statements,
					f.gen.SetPlace(f.placeFromSSA(target, ""), ir.GetPlace(f.placeFromSSA(target, "*"))))
			} else {
				src.Statements = append(src.Statements,
					f.gen.SetPlace(f.placeFromSSA(target, ""), ir.GetPlace(f.placeFromSSA(arg.(ir.SSAPlace), ""))))
			}
		}
	}

	block.Phis = nil
	for i, s := range block.Statements {
		block.Statements[i] = f.processStmt(s)
	}
	block.Test = f.processStmt(block.Test)
}

func (f *fromSSAState) processStmt(n ir.Node) ir.Node {
	switch t := n.(type) {
	case nil, ir.Const, *ir.TempBlock:
		return n
	case ir.SSAPlace:
		return f.placeFromSSA(t, "")
	case ir.Instr:
		args := make([]ir.Node, len(t.Args))
		for i, a := range t.Args {
			args[i] = f.processStmt(a)
		}
		return ir.Instr{ID: t.ID, Op: t.Op, Args: args, Pure: t.Pure}
	case ir.Get:
		return ir.Get{Place: f.processPlace(t.Place)}
	case ir.Set:
		return ir.Set{ID: t.ID, Place: f.processPlace(t.Place), Value: f.processStmt(t.Value)}
	case ir.BlockPlace:
		return f.processPlace(t)
	default:
		return n
	}
}

func (f *fromSSAState) processPlace(p ir.Place) ir.Place {
	switch t := p.(type) {
	case ir.SSAPlace:
		return f.placeFromSSA(t, "")
	case ir.BlockPlace:
		return ir.BlockPlace{Block: f.processStmt(t.Block), Index: f.processStmt(t.Index), Offset: t.Offset}
	default:
		return p
	}
}

// Requires implements ManagedPass.
func (ToSSA) Requires() []Analysis { return nil }

// Preserves implements ManagedPass — ToSSA produces SSA form.
func (ToSSA) Preserves() []Analysis { return []Analysis{AnalysisSSA} }

// Destroys implements ManagedPass.
func (ToSSA) Destroys() []Analysis { return nil }

// Requires implements ManagedPass.
func (FromSSA) Requires() []Analysis { return []Analysis{AnalysisSSA} }

// Preserves implements ManagedPass.
func (FromSSA) Preserves() []Analysis { return nil }

// Destroys implements ManagedPass — FromSSA exits SSA form.
func (FromSSA) Destroys() []Analysis { return []Analysis{AnalysisSSA} }
