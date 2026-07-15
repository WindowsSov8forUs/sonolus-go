import { createHash } from 'node:crypto'
import { readFileSync, writeFileSync } from 'node:fs'

const [sourcePath, outputPath, commit] = process.argv.slice(2)
if (!sourcePath || !outputPath || !commit) throw new Error('usage: node runtime_native_harness.mjs <Native.ts> <output> <commit>')

const source = readFileSync(sourcePath, 'utf8')
const start = source.indexOf('const reducer =')
if (start < 0) throw new Error('Native.ts reducer definition was not found')

let executable = source.slice(start)
const replacements = [
    ['(fn: (a: number, b: number) => number)', '(fn)'],
    ['(...values: number[])', '(...values)'],
    ['(x: number, a: number, b: number)', '(x, a, b)'],
    ['(x: number, y: number, s: number)', '(x, y, s)'],
    ['(a: number, b: number, x: number)', '(a, b, x)'],
    [
        'const funcs: Partial<Record<RuntimeFunction, [number, (...values: number[]) => number]>> =',
        'const funcs =',
    ],
]
for (const [typescript, javascript] of replacements) {
    if (!executable.includes(typescript)) throw new Error(`Native.ts no longer contains expected TypeScript fragment: ${typescript}`)
    executable = executable.replace(typescript, javascript)
}
const funcs = Function(`${executable}\nreturn funcs`)()

const inputs = {
    Add: [[1, -2, 3.5], [], [7], [Number.POSITIVE_INFINITY, Number.NEGATIVE_INFINITY]],
    Multiply: [[2, -3, 0.5], [], [7], [-0, 4]],
    Divide: [[24, 3, 2], [], [7], [1, 0], [0, 0]],
    Rem: [[-5, 3], [], [7], [1, 0]],
    Mod: [[-5, 3], [], [7], [1, 0]],
    Power: [[2, 3], [], [7], [-1, 0.5]],
    Log: [[Math.E], [0], [-1]],
    Negate: [[-3.25], [0], [-0]],
    Equal: [[2, 2], [Number.NaN, Number.NaN], [0, -0]],
    NotEqual: [[2, 3], [Number.NaN, Number.NaN]],
    Greater: [[3, 2], [Number.NaN, 0]],
    GreaterOr: [[2, 2], [Number.NaN, 0]],
    Less: [[2, 3], [Number.NaN, 0]],
    LessOr: [[2, 2], [Number.NaN, 0]],
    Not: [[0], [-0], [Number.NaN], [2]],
    Abs: [[-3.25], [-0], [Number.NEGATIVE_INFINITY]],
    Sign: [[-3.25], [-0], [Number.NaN]],
    Min: [[2, -3], [Number.NaN, 1], [0, -0]],
    Max: [[2, -3], [Number.NaN, 1], [0, -0]],
    Ceil: [[-1.25], [-0], [Number.POSITIVE_INFINITY]],
    Floor: [[-1.25], [-0], [Number.NEGATIVE_INFINITY]],
    Round: [[1.5], [-1.5], [-0.5], [-0]],
    Frac: [[-1.25], [-0], [Number.POSITIVE_INFINITY]],
    Trunc: [[-1.75], [-0], [Number.NEGATIVE_INFINITY]],
    Degree: [[Math.PI], [Number.POSITIVE_INFINITY]],
    Radian: [[180], [Number.NEGATIVE_INFINITY]],
    Sin: [[0.75], [Number.POSITIVE_INFINITY]],
    Cos: [[0.75], [Number.POSITIVE_INFINITY]],
    Tan: [[0.75], [Number.POSITIVE_INFINITY]],
    Sinh: [[0.75], [Number.POSITIVE_INFINITY]],
    Cosh: [[0.75], [Number.NEGATIVE_INFINITY]],
    Tanh: [[0.75], [Number.NEGATIVE_INFINITY]],
    Arcsin: [[0.5], [2]],
    Arccos: [[0.5], [2]],
    Arctan: [[0.5], [Number.POSITIVE_INFINITY]],
    Arctan2: [[1, -1], [-0, -0]],
    Clamp: [[3, -1, 2], [Number.NaN, -1, 2]],
    Lerp: [[-2, 6, 0.25], [0, Number.POSITIVE_INFINITY, 0]],
    LerpClamped: [[-2, 6, 2], [-2, 6, Number.NaN]],
    Unlerp: [[-2, 6, 2], [1, 1, 1]],
    UnlerpClamped: [[-2, 6, 10], [1, 1, 1]],
    Remap: [[0, 10, -2, 2, 2.5], [1, 1, 0, 1, 1]],
    RemapClamped: [[0, 10, -2, 2, 20], [1, 1, 0, 1, 1]],
}

const encodeNumber = (value) => {
    if (Number.isNaN(value)) return 'NaN'
    if (value === Number.POSITIVE_INFINITY) return '+Inf'
    if (value === Number.NEGATIVE_INFINITY) return '-Inf'
    if (Object.is(value, -0)) return '-0'
    return value
}

const cases = []
for (const [functionName, functionInputs] of Object.entries(inputs)) {
    const entry = funcs[functionName]
    if (!entry) throw new Error(`Native.ts does not define ${functionName}`)
    const [arity, fn] = entry
    for (const arguments_ of functionInputs) {
        if (arity !== Number.POSITIVE_INFINITY && arguments_.length !== arity) {
            throw new Error(`${functionName} expects ${arity} arguments, got ${arguments_.length}`)
        }
        cases.push({
            function: functionName,
            arguments: arguments_.map(encodeNumber),
            result: encodeNumber(fn(...arguments_)),
        })
    }
}

writeFileSync(outputPath, `${JSON.stringify({
    schemaVersion: 2,
    javascriptCommit: commit,
    nativeSourceSha256: createHash('sha256').update(source).digest('hex'),
    cases,
}, null, 2)}\n`)
