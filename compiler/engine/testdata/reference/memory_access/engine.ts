// TypeScript equivalent of memory_access/engine.go.
import { Archetype, entityMemory, defineImport } from '@sonolus/sonolus.js-compiler/play'

const archetype = new Archetype('N')
const beat = defineImport('Beat')
const value = entityMemory(Number)
const saved = entityMemory(Number)
defineImport(value)
defineImport(saved)

export function initialize() {
	value.set(0)
	saved.set(0)
}

export function updateSequential() {
	value.set(beat * 2)
	if (value.get() > 100) {
		saved.set(value.get())
		value.set(0)
	}
}
