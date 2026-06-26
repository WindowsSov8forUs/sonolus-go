// Emits golden fixtures from the REAL sonolus.js-compiler optimizer + assembler,
// for the Go port to validate against. Run with Node 24 (strips types).
import { optimizeSNode } from './snode/optimize/index.ts'
import { createAppendSNode } from './assemble.ts'

type SNode = number | { func: string; args: SNode[] }
const F = (func: string, ...args: SNode[]): SNode => ({ func, args })
// Opaque "variable" nodes the optimizer leaves untouched (Get with a constant index).
const G = (i: number): SNode => F('Get', 1000, i)
const X = G(0), Y = G(1), Z = G(2), D = G(3), A = G(4), B = G(5), C = G(6), E = G(7)

const cases: { name: string; input: SNode }[] = [
    { name: 'value_root', input: 7 },
    { name: 'value_float', input: 0.1 },

    { name: 'add_empty', input: F('Add') },
    { name: 'add_consts', input: F('Add', 2, 3) },
    { name: 'add_const_dyn', input: F('Add', 0, X) },
    { name: 'add_const_front', input: F('Add', 2, X, 3) },
    { name: 'add_nested', input: F('Add', F('Add', X, 1), Y) },

    { name: 'mul_zero', input: F('Multiply', 0, X) },
    { name: 'mul_identity', input: F('Multiply', 1, X) },
    { name: 'mul_consts', input: F('Multiply', 2, 3) },
    { name: 'mul_const_dyn', input: F('Multiply', 2, X, 3) },

    { name: 'sub_basic', input: F('Subtract', X, 2, 3) },
    { name: 'sub_nested', input: F('Subtract', F('Subtract', X, 1), 2) },
    { name: 'sub_all_const', input: F('Subtract', X, 0) },

    { name: 'div_basic', input: F('Divide', X, 2, 4) },

    { name: 'mod_flatten', input: F('Mod', F('Mod', X, Y), Z) },
    { name: 'power_single', input: F('Power', X) },

    { name: 'if_and', input: F('If', X, Y, 0) },
    { name: 'if_keep', input: F('If', X, Y, Z) },

    { name: 'get_shifted', input: F('Get', 1000, F('Add', X, F('Multiply', Y, Z))) },
    { name: 'getshifted_fold_const', input: F('GetShifted', 1000, 2, 3, 4) },
    { name: 'getshifted_fold_zero', input: F('GetShifted', 1000, X, 0, 0) },

    { name: 'set_compound', input: F('Set', 1000, 5, F('Add', F('Get', 1000, 5), X)) },
    { name: 'set_compound_commutative', input: F('Set', 1000, 5, F('Add', X, F('Get', 1000, 5))) },
    { name: 'set_shifted_index', input: F('Set', 1000, F('Add', 2, F('Multiply', Y, 3)), X) },

    { name: 'switch_normalize_default0', input: F('SwitchWithDefault', D, 0, A, 1, B, 2, C, 0) },
    { name: 'switch_normalize_withdefault', input: F('SwitchWithDefault', D, 0, A, 1, B, 2, C, E) },
    { name: 'switch_no_normalize', input: F('SwitchWithDefault', D, 0, A, 3, B, 5, C, 0) },

    { name: 'while_execute2', input: F('While', X, F('Execute', Y, 0)) },
    { name: 'while_execute_n', input: F('While', X, F('Execute', Y, Z, 0)) },

    { name: 'appender_dedup', input: F('Add', F('Multiply', X, X), F('Multiply', X, X)) },
]

const results = cases.map(({ name, input }) => {
    const optimized = optimizeSNode(input as never)
    const nodes: unknown[] = []
    const root = createAppendSNode(nodes as never)(optimized)
    return { name, input, optimized, nodes, root }
})

// JS canonical number stringifications for FormatNumber() validation.
const numberFormats = [
    0, 1, 5, -5, 0.1, 0.5, 1.5, 100, 1000000, 0.000001, 1e-7, 5e-7, 1e21, 1e20,
    123456789, 2.5e-8, -3.14, 9007199254740991, 0.0001, 12345.678,
].map((value) => ({ value, str: String(value) }))

process.stdout.write(JSON.stringify({ numberFormats, cases: results }, null, 2))
