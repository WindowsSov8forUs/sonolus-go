// TypeScript equivalent of arithmetic/engine.go.
import { Archetype, entityMemory, defineImport } from '@sonolus/sonolus.js-compiler/play'

const archetype = new Archetype('N')
const x = entityMemory(Number)
const y = entityMemory(Number)
const z = entityMemory(Number)
defineImport(x)
defineImport(y)
defineImport(z)

export function initialize() {
	x.set(0)
	y.set(0)
	z.set(0)
}

export function updateSequential() {
	x.set((3 + 5) * (2 + 1))
	y.set(x.get() * 2 - 4)
	z.set(x.get() + y.get())
}
