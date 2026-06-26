# Emits golden fixtures from sonolus.py's real optimizer passes. Run with
# PYTHONPATH pointed at a sonolus.py checkout:
#   PYTHONPATH=/path/to/sonolus.py python harness.py > optimize_golden.json
import json

from sonolus.backend.ir import IRConst, IRGet, IRSet
from sonolus.backend.node import FunctionNode
from sonolus.backend.place import BlockPlace
from sonolus.backend.finalize import cfg_to_engine_node
from sonolus.backend.optimize.flow import BasicBlock
from sonolus.backend.optimize.passes import OptimizerConfig
from sonolus.backend.optimize.simplify import CoalesceFlow
from sonolus.backend.optimize.dead_code import UnreachableCodeElimination, DeadCodeElimination
from sonolus.backend.optimize.dominance import DominanceFrontiers
from sonolus.backend.optimize.ssa import ToSSA, FromSSA
from sonolus.backend.optimize.inlining import InlineVars
from sonolus.backend.optimize.constant_evaluation import SparseConditionalConstantPropagation
from sonolus.backend.blocks import PlayBlock
from sonolus.backend.mode import Mode
from sonolus.backend.optimize.flow import traverse_cfg_reverse_postorder
from sonolus.backend.ir import IRPureInstr, IRInstr
from sonolus.backend.ops import Op
from sonolus.backend.place import TempBlock, SSAPlace

CFG = OptimizerConfig()


def canon(node):
    if isinstance(node, FunctionNode):
        return node.func.value + "(" + ",".join(canon(a) for a in node.args) + ")"
    v = float(node)
    if v.is_integer():
        return "#" + str(int(v))
    return "#" + repr(v)


def cell(b, i):
    return BlockPlace(b, i, 0)


def sset(b, i, v):
    return IRSet(cell(b, i), IRConst(v))


def sget(b, i):
    return IRGet(cell(b, i))


# --- case builders (mirrored exactly in Go) ---

def linear3():
    e, b1, b2 = BasicBlock(), BasicBlock(), BasicBlock()
    e.statements = [sset(1, 0, 1)]
    b1.statements = [sset(1, 1, 2)]
    e.connect_to(b1)
    b1.connect_to(b2)
    return e


def empty_skip():
    e, mid, tgt = BasicBlock(), BasicBlock(), BasicBlock()
    e.statements = [sset(0, 0, 7)]
    tgt.statements = [sset(1, 0, 9)]
    e.connect_to(mid)
    mid.connect_to(tgt)
    return e


def const_test():
    e, a, b, exit_ = BasicBlock(), BasicBlock(), BasicBlock(), BasicBlock()
    e.test = IRConst(0)
    a.statements = [sset(1, 0, 1)]
    b.statements = [sset(1, 0, 2)]
    e.connect_to(b, 0)     # cond 0 (taken, since test == 0)
    e.connect_to(a, None)  # default (pruned)
    a.connect_to(exit_)
    b.connect_to(exit_)
    return e


def diamond():
    e, then_b, else_b, merge = BasicBlock(), BasicBlock(), BasicBlock(), BasicBlock()
    e.test = sget(0, 0)
    then_b.statements = [sset(1, 0, 1)]
    else_b.statements = [sset(1, 0, 2)]
    e.connect_to(else_b, 0)
    e.connect_to(then_b, None)
    then_b.connect_to(merge)
    else_b.connect_to(merge)
    return e


CASES = {
    "linear3": linear3,
    "empty_skip": empty_skip,
    "const_test": const_test,
    "diamond": diamond,
}

out = {}
for name, build in CASES.items():
    out[name] = {
        "before": canon(cfg_to_engine_node(build())),
        "afterUCE": canon(cfg_to_engine_node(UnreachableCodeElimination().run(build(), CFG))),
        "afterCoalesce": canon(cfg_to_engine_node(CoalesceFlow().run(build(), CFG))),
    }


# --- DCE: compared at the IR level (post-DCE blocks still hold temps) ---

def pcanon(p):
    if isinstance(p, BlockPlace):
        if isinstance(p.block, TempBlock):
            return "T(" + p.block.name + ")"
        return "M(" + vcanon(p.block) + "," + vcanon(p.index) + ")"
    return str(p)


def vcanon(n):
    if isinstance(n, (IRPureInstr, IRInstr)):
        return n.op.value + "[" + ",".join(vcanon(a) for a in n.args) + "]"
    if isinstance(n, IRGet):
        return "G(" + pcanon(n.place) + ")"
    if isinstance(n, IRConst):
        return str(int(n.value))
    if isinstance(n, int):
        return str(int(n))
    if isinstance(n, BlockPlace):
        return pcanon(n)
    raise TypeError(type(n))


def scanon(s):
    if isinstance(s, IRSet):
        return pcanon(s.place) + "=" + vcanon(s.value)
    return vcanon(s)


def block_canon(b):
    return ";".join(scanon(s) for s in b.statements)


def dce_dead_store():
    a, b = TempBlock("a"), TempBlock("b")
    e = BasicBlock()
    e.statements = [
        IRSet(a[0], IRConst(5)),
        IRSet(b[0], sget(0, 0)),
        IRSet(cell(0, 1), IRGet(b[0])),
    ]
    return e


def dce_self_copy():
    a = TempBlock("a")
    e = BasicBlock()
    e.statements = [
        IRSet(a[0], sget(0, 0)),
        IRSet(a[0], IRGet(a[0])),
        IRSet(cell(0, 1), IRGet(a[0])),
    ]
    return e


def dce_side_effect():
    a = TempBlock("a")
    e = BasicBlock()
    e.statements = [IRSet(a[0], IRInstr(Op.Draw, [IRConst(1), IRConst(2)]))]
    return e


def dce_transitive():
    a, b = TempBlock("a"), TempBlock("b")
    e = BasicBlock()
    e.statements = [
        IRSet(a[0], sget(0, 0)),
        IRSet(b[0], IRPureInstr(Op.Add, [IRGet(a[0]), IRConst(1)])),
        IRSet(cell(0, 1), IRGet(b[0])),
    ]
    return e


DCE_CASES = {
    "dead_store": dce_dead_store,
    "self_copy": dce_self_copy,
    "side_effect": dce_side_effect,
    "transitive": dce_transitive,
}

dce = {}
for name, build in DCE_CASES.items():
    dce[name] = block_canon(DeadCodeElimination().run(build(), CFG))


# --- SSA: render the SSA form (phis + versioned places) ---

def pcanon2(p):
    if isinstance(p, SSAPlace):
        return p.name + "." + str(p.num)
    return pcanon(p)


def vcanon2(n):
    if isinstance(n, (IRPureInstr, IRInstr)):
        return n.op.value + "[" + ",".join(vcanon2(a) for a in n.args) + "]"
    if isinstance(n, IRGet):
        return "G(" + pcanon2(n.place) + ")"
    if isinstance(n, IRConst):
        return "#" + str(int(n.value))
    if isinstance(n, SSAPlace):
        return pcanon2(n)
    if isinstance(n, int):
        return "#" + str(int(n))
    if isinstance(n, BlockPlace):
        return pcanon2(n)
    raise TypeError(type(n))


def scanon2(s):
    if isinstance(s, IRSet):
        return pcanon2(s.place) + "=" + vcanon2(s.value)
    return vcanon2(s)


def ssa_canon(entry):
    blocks = list(traverse_cfg_reverse_postorder(entry))
    idx = {b: i for i, b in enumerate(blocks)}
    out_blocks = []
    for b in blocks:
        parts = []
        for tgt, args in b.phis.items():
            arg_strs = ["P%d:%s" % (idx[src], pcanon2(args[src])) for src in sorted(args, key=lambda x: idx[x])]
            parts.append(pcanon2(tgt) + "=phi(" + ",".join(arg_strs) + ")")
        for s in b.statements:
            parts.append(scanon2(s))
        if not (isinstance(b.test, IRConst) and b.test.value == 0):
            parts.append("?" + vcanon2(b.test))
        out_blocks.append("B%d{%s}" % (idx[b], ";".join(parts)))
    return "".join(out_blocks)


def ssa_diamond():
    x = TempBlock("x")
    e, then_b, else_b, merge = BasicBlock(), BasicBlock(), BasicBlock(), BasicBlock()
    e.test = sget(0, 0)
    then_b.statements = [IRSet(x[0], IRConst(1))]
    else_b.statements = [IRSet(x[0], IRConst(2))]
    merge.statements = [IRSet(cell(0, 1), IRGet(x[0]))]
    e.connect_to(else_b, 0)
    e.connect_to(then_b, None)
    then_b.connect_to(merge)
    else_b.connect_to(merge)
    return e


def ssa_loop():
    i = TempBlock("i")
    e, header, body, exit_ = BasicBlock(), BasicBlock(), BasicBlock(), BasicBlock()
    e.statements = [IRSet(i[0], IRConst(0))]
    header.test = sget(0, 0)
    body.statements = [IRSet(i[0], IRPureInstr(Op.Add, [IRGet(i[0]), IRConst(1)]))]
    exit_.statements = [IRSet(cell(0, 1), IRGet(i[0]))]
    e.connect_to(header)
    header.connect_to(exit_, 0)
    header.connect_to(body, None)
    body.connect_to(header)
    return e


SSA_CASES = {"diamond": ssa_diamond, "loop": ssa_loop}

ssa = {}
for name, build in SSA_CASES.items():
    entry = build()
    DominanceFrontiers().run(entry, CFG)
    ToSSA().run(entry, CFG)
    ssa[name] = ssa_canon(entry)


# --- FromSSA: full-CFG IR canon after a ToSSA -> FromSSA round trip ---

def cfg_canon(entry):
    blocks = list(traverse_cfg_reverse_postorder(entry))
    idx = {b: i for i, b in enumerate(blocks)}
    res = []
    for b in blocks:
        body = [scanon(s) for s in b.statements]
        if not (isinstance(b.test, IRConst) and b.test.value == 0):
            body.append("?" + vcanon(b.test))
        edges = []
        for e in sorted(b.outgoing, key=lambda e: (e.cond is None, e.cond)):
            lab = "->" if e.cond is None else (str(int(e.cond)) + ":")
            edges.append(lab + "B" + str(idx[e.dst]))
        res.append("B%d{%s}(%s)" % (idx[b], ";".join(body), ",".join(edges)))
    return "".join(res)


fromssa = {}
for name, build in SSA_CASES.items():
    entry = build()
    DominanceFrontiers().run(entry, CFG)
    ToSSA().run(entry, CFG)
    FromSSA().run(entry, CFG)
    fromssa[name] = cfg_canon(entry)


# --- InlineVars: SSA-form canon after ToSSA + InlineVars ---

def inl_const_chain():
    x, y = TempBlock("x"), TempBlock("y")
    e = BasicBlock()
    e.statements = [
        IRSet(x[0], IRConst(5)),
        IRSet(y[0], IRPureInstr(Op.Add, [IRGet(x[0]), IRConst(3)])),
        IRSet(cell(0, 0), IRGet(y[0])),
    ]
    return e


def inl_shared_const():
    x = TempBlock("x")
    e = BasicBlock()
    e.statements = [
        IRSet(x[0], IRConst(5)),
        IRSet(cell(0, 0), IRGet(x[0])),
        IRSet(cell(0, 1), IRGet(x[0])),
    ]
    return e


INLINE_CASES = {
    "const_chain": inl_const_chain,
    "shared_const": inl_shared_const,
    "diamond": ssa_diamond,
}

inline = {}
for name, build in INLINE_CASES.items():
    entry = build()
    DominanceFrontiers().run(entry, CFG)
    ToSSA().run(entry, CFG)
    InlineVars().run(entry, CFG)
    inline[name] = ssa_canon(entry)


# A memory read from a non-writable block (EngineRom) inlines once the block
# oracle knows the callback can't write it.
def inl_memory():
    t = TempBlock("t")
    e = BasicBlock()
    e.statements = [
        IRSet(t[0], IRGet(BlockPlace(PlayBlock.EngineRom, 0, 0))),
        IRSet(BlockPlace(PlayBlock.LevelMemory, 1, 0), IRGet(t[0])),
    ]
    return e


play_cfg = OptimizerConfig(mode=Mode.PLAY, callback="updateParallel")
_e = inl_memory()
DominanceFrontiers().run(_e, play_cfg)
ToSSA().run(_e, play_cfg)
InlineVars().run(_e, play_cfg)
inline["memory"] = ssa_canon(_e)

# --- SCCP: SSA-form canon after ToSSA + SCCP ---

def sccp_const_fold():
    x, y = TempBlock("x"), TempBlock("y")
    e = BasicBlock()
    e.statements = [
        IRSet(x[0], IRConst(5)),
        IRSet(y[0], IRPureInstr(Op.Add, [IRGet(x[0]), IRConst(3)])),
        IRSet(cell(0, 0), IRGet(y[0])),
    ]
    return e


def sccp_phi_const():
    x = TempBlock("x")
    e, then_b, else_b, merge = BasicBlock(), BasicBlock(), BasicBlock(), BasicBlock()
    e.test = sget(0, 0)
    then_b.statements = [IRSet(x[0], IRConst(7))]
    else_b.statements = [IRSet(x[0], IRConst(7))]
    merge.statements = [IRSet(cell(0, 1), IRGet(x[0]))]
    e.connect_to(else_b, 0)
    e.connect_to(then_b, None)
    then_b.connect_to(merge)
    else_b.connect_to(merge)
    return e


SCCP_CASES = {"const_fold": sccp_const_fold, "phi_const": sccp_phi_const}

sccp = {}
for name, build in SCCP_CASES.items():
    entry = build()
    DominanceFrontiers().run(entry, CFG)
    ToSSA().run(entry, CFG)
    SparseConditionalConstantPropagation().run(entry, CFG)
    sccp[name] = ssa_canon(entry)

print(json.dumps({
    "cases": out, "dceCases": dce, "ssaCases": ssa,
    "fromSSACases": fromssa, "inlineCases": inline, "sccpCases": sccp,
}, indent=2))
