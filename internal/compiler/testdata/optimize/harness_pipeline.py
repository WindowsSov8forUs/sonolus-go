# Emits end-to-end pipeline golden fixtures from sonolus.py's STANDARD_PASSES.
# Run with PYTHONPATH pointed at a sonolus.py checkout:
#   PYTHONPATH=/path/to/sonolus.py python harness_pipeline.py > pipeline_golden.json
#
# Each case: build CFG → run STANDARD_PASSES → finalize → count nodes + canonize.
# The output is compared by Go's TestPipelineGolden in optimize_test.go.

import json

from sonolus.backend.ir import IRConst, IRGet, IRSet, IRInstr, IRPureInstr
from sonolus.backend.place import BlockPlace, TempBlock
from sonolus.backend.finalize import cfg_to_engine_node
from sonolus.backend.optimize.flow import BasicBlock, traverse_cfg_reverse_postorder
from sonolus.backend.optimize.passes import STANDARD_PASSES, OptimizerConfig
from sonolus.backend.ops import Op
from sonolus.backend.mode import Mode
from sonolus.backend.blocks import PlayBlock


CFG = OptimizerConfig(mode=Mode.PLAY, callback="updateParallel")


def run_pipeline(entry, cfg=CFG):
    """Run STANDARD_PASSES end-to-end and return the final BasicBlock."""
    for p in STANDARD_PASSES:
        entry = p.run(entry, cfg)
    return entry


def canon_node(node):
    """Canonical string for an engine node (mirrors Go test canon)."""
    if hasattr(node, 'func'):
        args = ",".join(canon_node(a) for a in node.args)
        return node.func.value + "(" + args + ")"
    v = float(node)
    if v.is_integer():
        return "#" + str(int(v))
    return "#" + repr(v)


def node_count(entry):
    """Count finalized engine nodes after full pipeline."""
    n = cfg_to_engine_node(run_pipeline(entry))
    # Walk the tree counting nodes.
    def count(n):
        c = 1
        if hasattr(n, 'args'):
            for a in n.args:
                c += count(a)
        return c
    return count(n)


def cell(b, i):
    return BlockPlace(b, i, 0)


def sset(b, i, v):
    return IRSet(cell(b, i), IRConst(v))


def sget(b, i):
    return IRGet(cell(b, i))


# ── case builders ──

def linear_constant():
    """x := 1+2; set(0,0,x) → constant 3 should fold."""
    x = TempBlock("x")
    e = BasicBlock()
    e.statements = [
        IRSet(x[0], IRConst(3)),  # pre-folded for simplicity
        IRSet(cell(0, 0), IRGet(x[0])),
    ]
    return e


def diamond_constant():
    """x := 5; if x>3 { set(0,0,1) } else { set(0,0,2) } → dead branch pruned."""
    x = TempBlock("x")
    e, then_b, else_b, merge = BasicBlock(), BasicBlock(), BasicBlock(), BasicBlock()
    e.statements = [IRSet(x[0], IRConst(5))]
    e.test = IRPureInstr(Op.Greater, [IRGet(x[0]), IRConst(3)])
    then_b.statements = [sset(0, 0, 1)]
    else_b.statements = [sset(0, 0, 2)]
    e.connect_to(else_b, 0)
    e.connect_to(then_b, None)
    then_b.connect_to(merge)
    else_b.connect_to(merge)
    return e


def diamond_memory():
    """x := get(0,0); if x>5 { set(0,1,1) } else { set(0,1,2) }"""
    x = TempBlock("x")
    e, then_b, else_b, merge = BasicBlock(), BasicBlock(), BasicBlock(), BasicBlock()
    e.statements = [IRSet(x[0], sget(0, 0))]
    e.test = IRPureInstr(Op.Greater, [IRGet(x[0]), IRConst(5)])
    then_b.statements = [sset(0, 1, 1)]
    else_b.statements = [sset(0, 1, 2)]
    e.connect_to(else_b, 0)
    e.connect_to(then_b, None)
    then_b.connect_to(merge)
    else_b.connect_to(merge)
    return e


def loop_memory():
    """sum := 0; for i := range 10 { v := get(0,0); sum := sum+v }; set(0,1,sum)"""
    sum_t = TempBlock("sum")
    i_t = TempBlock("i")
    v_t = TempBlock("v")

    e = BasicBlock()
    header, body, exit_ = BasicBlock(), BasicBlock(), BasicBlock()
    e.statements = [
        IRSet(sum_t[0], IRConst(0)),
        IRSet(i_t[0], IRConst(0)),
    ]

    header.test = IRPureInstr(Op.Less, [IRGet(i_t[0]), IRConst(10)])
    header.connect_to(exit_, 0)
    header.connect_to(body, None)

    body.statements = [
        IRSet(v_t[0], sget(0, 0)),  # loop-invariant memory load
        IRSet(sum_t[0], IRPureInstr(Op.Add, [IRGet(sum_t[0]), IRGet(v_t[0])])),
        IRSet(i_t[0], IRPureInstr(Op.Add, [IRGet(i_t[0]), IRConst(1)])),
    ]
    body.connect_to(header)

    exit_.statements = [IRSet(cell(0, 1), IRGet(sum_t[0]))]

    e.connect_to(header)
    return e


def switch_chain():
    """x := get(0,0); if x==1 {set(0,1,10)} else if x==2 {set(0,1,20)} else {set(0,1,30)}"""
    x = TempBlock("x")
    e, c1, rest, c2, dflt, merge = (BasicBlock() for _ in range(6))

    e.statements = [IRSet(x[0], sget(0, 0))]
    e.test = IRPureInstr(Op.Equal, [IRGet(x[0]), IRConst(1)])
    c1.statements = [sset(0, 1, 10)]
    e.connect_to(rest, 0)  # false → test next
    e.connect_to(c1, None)  # true

    rest.test = IRPureInstr(Op.Equal, [IRGet(x[0]), IRConst(2)])
    c2.statements = [sset(0, 1, 20)]
    dflt.statements = [sset(0, 1, 30)]
    rest.connect_to(dflt, 0)
    rest.connect_to(c2, None)

    c1.connect_to(merge)
    c2.connect_to(merge)
    dflt.connect_to(merge)

    return e


# ── Generate ──

CASES = {
    "linear_constant": linear_constant,
    "diamond_constant": diamond_constant,
    "diamond_memory": diamond_memory,
    "loop_memory": loop_memory,
    "switch_chain": switch_chain,
}

out = {}
for name, build in CASES.items():
    entry = build()
    entry = run_pipeline(entry)
    n = cfg_to_engine_node(entry)
    out[name] = {
        "nodeCount": node_count(build()),
        "nodes": canon_node(n),
    }

print(json.dumps(out, indent=2))
