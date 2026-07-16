# Emits end-to-end pipeline golden fixtures from sonolus.py's STANDARD_PASSES.
# Run with PYTHONPATH pointed at a sonolus.py checkout:
#   PYTHONPATH=/path/to/sonolus.py python harness_pipeline.py > pipeline_golden.json
#
# Each case: build CFG → run STANDARD_PASSES → finalize → count nodes + canonize.
# The output is compared by Go's TestPipelineGolden in optimize_test.go.

import json
from pathlib import Path

from sonolus.backend.ir import IRConst, IRGet, IRSet, IRInstr, IRPureInstr
from sonolus.backend.place import BlockPlace, TempBlock
from sonolus.backend.finalize import cfg_to_engine_node
from sonolus.backend.optimize.flow import BasicBlock, traverse_cfg_reverse_postorder
from sonolus.backend.optimize.optimize import STANDARD_PASSES
from sonolus.backend.optimize.passes import OptimizerConfig, run_passes
from sonolus.backend.ops import Op
from sonolus.backend.mode import Mode
from sonolus.backend.blocks import PlayBlock


CFG = OptimizerConfig(mode=Mode.PLAY, callback="updateParallel")


def run_pipeline(entry, cfg=CFG):
    """Run STANDARD_PASSES end-to-end and return the final BasicBlock."""
    return run_passes(entry, STANDARD_PASSES, cfg)


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


def place_canon(place):
    if hasattr(place, "name") and hasattr(place, "num"):
        return "S(" + place.name + "." + str(place.num) + ")"
    if isinstance(place, BlockPlace):
        if isinstance(place.block, TempBlock):
            return "T(" + place.block.name + "," + str(place.index) + ")"
        block = place.block.value if hasattr(place.block, "value") else place.block
        return "M(" + str(block) + "," + str(place.index) + "," + str(place.offset) + ")"
    return str(place)


def value_canon(value):
    if isinstance(value, (IRPureInstr, IRInstr)):
        return value.op.value + "[" + ",".join(value_canon(arg) for arg in value.args) + "]"
    if isinstance(value, IRGet):
        return "G(" + place_canon(value.place) + ")"
    if isinstance(value, IRConst):
        number = float(value.value)
        return "#" + (str(int(number)) if number.is_integer() else repr(number))
    if isinstance(value, BlockPlace) or hasattr(value, "num"):
        return place_canon(value)
    if isinstance(value, (int, float)):
        number = float(value)
        return "#" + (str(int(number)) if number.is_integer() else repr(number))
    raise TypeError(type(value))


def statement_canon(statement):
    if isinstance(statement, IRSet):
        return place_canon(statement.place) + "=" + value_canon(statement.value)
    return value_canon(statement)


def place_snapshot(place):
    if hasattr(place, "name") and hasattr(place, "num"):
        return {"kind": "ssa", "name": place.name, "version": place.num}
    if isinstance(place, BlockPlace):
        if isinstance(place.block, TempBlock):
            return {"kind": "local", "name": place.block.name, "index": place.index, "offset": place.offset}
        block = place.block.value if hasattr(place.block, "value") else place.block
        return {"kind": "memory", "storage": str(block), "index": place.index, "offset": place.offset}
    return {"kind": type(place).__name__, "value": str(place)}


def value_snapshot(value):
    if isinstance(value, (IRPureInstr, IRInstr)):
        return {"kind": "call", "function": value.op.value, "arguments": [value_snapshot(arg) for arg in value.args]}
    if isinstance(value, IRGet):
        return {"kind": "load", "place": place_snapshot(value.place)}
    if isinstance(value, IRConst):
        return {"kind": "const", "value": float(value.value)}
    if isinstance(value, BlockPlace) or hasattr(value, "num"):
        return {"kind": "place", "place": place_snapshot(value)}
    if isinstance(value, (int, float)):
        return {"kind": "const", "value": float(value)}
    raise TypeError(type(value))


def cfg_snapshot(entry):
    blocks = list(traverse_cfg_reverse_postorder(entry))
    indexes = {block: index for index, block in enumerate(blocks)}
    result = []
    for index, block in enumerate(blocks):
        phis = []
        for target, arguments in block.phis.items():
            ordered = sorted(arguments.items(), key=lambda item: indexes[item[0]])
            phis.append({
                "target": place_snapshot(target),
                "arguments": [{"predecessor": indexes[pred], "value": place_snapshot(value)} for pred, value in ordered],
            })
        instructions = []
        for statement in block.statements:
            if isinstance(statement, IRSet):
                instructions.append({"kind": "store", "place": place_snapshot(statement.place), "value": value_snapshot(statement.value)})
            else:
                instructions.append({"kind": "eval", "value": value_snapshot(statement)})
        terminator = {"kind": "return"}
        if block.outgoing:
            terminator = {
                "kind": "switch",
                "value": value_snapshot(block.test),
                "edges": [
                    {"value": None if edge.cond is None else value_snapshot(edge.cond), "target": indexes[edge.dst]}
                    for edge in sorted(block.outgoing, key=lambda item: (item.cond is None, item.cond))
                ],
            }
        result.append({"id": index, "phis": phis, "instructions": instructions, "terminator": terminator})
    return {"entry": 0, "blocks": result}


def cfg_canon(entry):
    blocks = list(traverse_cfg_reverse_postorder(entry))
    indexes = {block: index for index, block in enumerate(blocks)}
    result = []
    for index, block in enumerate(blocks):
        parts = []
        for target, arguments in block.phis.items():
            ordered = sorted(arguments.items(), key=lambda item: indexes[item[0]])
            args = ["P%d:%s" % (indexes[pred], place_canon(value)) for pred, value in ordered]
            parts.append(place_canon(target) + "=phi(" + ",".join(args) + ")")
        parts.extend(statement_canon(statement) for statement in block.statements)
        if not (isinstance(block.test, IRConst) and block.test.value == 0):
            parts.append("?" + value_canon(block.test))
        edges = []
        for edge in sorted(block.outgoing, key=lambda item: (item.cond is None, item.cond)):
            label = "default" if edge.cond is None else value_canon(edge.cond)
            edges.append(label + ":B" + str(indexes[edge.dst]))
        result.append("B%d{%s}[%s]" % (index, ";".join(parts), ",".join(edges)))
    return "".join(result)


CHECKPOINTS = {
    "toSSA": 5,
    "firstSCCPCleanup": 9,
    "secondSCCP": 15,
    "fromSSA": 31,
    "allocate": 38,
}


def checkpoint(build, pass_count):
    return cfg_canon(run_passes(build(), STANDARD_PASSES[:pass_count], CFG))


def structured_checkpoint(build, pass_count):
    return cfg_snapshot(run_passes(build(), STANDARD_PASSES[:pass_count], CFG))


def cell(b, i):
    if isinstance(b, int):
        b = PlayBlock.LevelMemory
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

def build_fixture_case(spec):
    locals_ = {
        (item if isinstance(item, str) else item["name"]): TempBlock(
            item if isinstance(item, str) else item["name"],
            1 if isinstance(item, str) else item["size"],
        )
        for item in spec["locals"]
    }
    blocks = [BasicBlock() for _ in spec["blocks"]]

    def expression(raw):
        operation = raw[0]
        if operation == "const":
            return IRConst(raw[1])
        if operation == "local":
            return IRGet(locals_[raw[1]][0 if len(raw) == 2 else raw[2]])
        if operation == "memory":
            return sget(0, raw[1])
        if operation == "memoryAt":
            return IRGet(BlockPlace(getattr(PlayBlock, raw[1]), raw[2], 0))
        if operation == "call":
            return IRPureInstr(getattr(Op, raw[1]), [expression(argument) for argument in raw[2:]])
        if operation == "effectCall":
            return IRInstr(getattr(Op, raw[1]), [expression(argument) for argument in raw[2:]])
        raise ValueError(f"unknown fixture expression {operation}")

    for block, block_spec in zip(blocks, spec["blocks"]):
        for statement in block_spec["statements"]:
            if statement[0] == "setLocal":
                offset = 0 if len(statement) == 3 else statement[2]
                value = statement[2] if len(statement) == 3 else statement[3]
                block.statements.append(IRSet(locals_[statement[1]][offset], expression(value)))
            elif statement[0] == "setMemory":
                block.statements.append(IRSet(cell(0, statement[1]), expression(statement[2])))
            elif statement[0] == "eval":
                block.statements.append(expression(statement[1]))
            else:
                raise ValueError(f"unknown fixture statement {statement[0]}")
        terminator = block_spec["terminator"]
        if terminator[0] == "jump":
            block.connect_to(blocks[terminator[1]])
        elif terminator[0] == "branch":
            block.test = expression(terminator[1])
            block.connect_to(blocks[terminator[3]], 0)
            block.connect_to(blocks[terminator[2]], None)
        elif terminator[0] != "return":
            raise ValueError(f"unknown fixture terminator {terminator[0]}")
    return blocks[0]


with Path(__file__).with_name("pipeline_fixture.json").open(encoding="utf-8") as fixture_file:
    fixture = json.load(fixture_file)
if fixture["schemaVersion"] != 2:
    raise ValueError(f"unsupported pipeline fixture schema {fixture['schemaVersion']}")
CASES = {
    case["name"]: (lambda case=case: build_fixture_case(case))
    for case in fixture["cases"]
}

out = {}
for name, build in CASES.items():
    case_spec = next(case for case in fixture["cases"] if case["name"] == name)
    if case_spec.get("expectAllocateError"):
        try:
            run_passes(build(), STANDARD_PASSES[:CHECKPOINTS["allocate"]], CFG)
        except ValueError as error:
            if str(error) != "Temporary memory limit exceeded":
                raise
            allocate_error = "temporary-memory-overflow"
        else:
            raise AssertionError(f"{name} unexpectedly allocated within 4096 slots")
        checkpoint_counts = {key: value for key, value in CHECKPOINTS.items() if key != "allocate"}
        out[name] = {
            "checkpoints": {key: checkpoint(build, value) for key, value in checkpoint_counts.items()},
            "structuredCheckpoints": {key: structured_checkpoint(build, value) for key, value in checkpoint_counts.items()},
            "nodeCount": 0,
            "nodes": "",
            "allocateError": allocate_error,
        }
        continue
    entry = build()
    entry = run_pipeline(entry)
    n = cfg_to_engine_node(entry)
    out[name] = {
        "checkpoints": {
            checkpoint_name: checkpoint(build, pass_count)
            for checkpoint_name, pass_count in CHECKPOINTS.items()
        },
        "structuredCheckpoints": {
            checkpoint_name: structured_checkpoint(build, pass_count)
            for checkpoint_name, pass_count in CHECKPOINTS.items()
        },
        "nodeCount": node_count(build()),
        "nodes": canon_node(n),
        "allocateError": "",
    }

print(json.dumps(out, indent=2))
