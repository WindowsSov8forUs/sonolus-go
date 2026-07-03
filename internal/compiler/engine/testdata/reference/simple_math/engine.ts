// TypeScript equivalent of simple_math/engine.go.
// Compiled via sonolus.js-compiler for reference comparison.
// See compile.mjs for the bridge invocation.

import { Archetype, entityMemory, defineImport } from '@sonolus/sonolus.js-compiler/play'

const archetype = new Archetype('N')
const a = entityMemory(Number)
const b = entityMemory(Number)
defineImport(a)
defineImport(b)

export function lerp(a: number, b: number, t: number): number {
	return a + (b - a) * t
}

export function initialize() {
	a.set(1 + 2 * 3)
	b.set(lerp(a.get(), 100, 0.05))
}
