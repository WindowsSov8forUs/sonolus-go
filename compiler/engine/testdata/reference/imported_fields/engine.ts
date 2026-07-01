// TypeScript equivalent of imported_fields/engine.go.
import { Archetype, entityMemory, defineImport } from '@sonolus/sonolus.js-compiler/play'

const archetype = new Archetype('N')
const beat = defineImport('Beat')
const bpm = defineImport('Bpm')
const sum = entityMemory(Number)
defineImport(sum)

export function initialize() {
	sum.set(beat + bpm)
}

export function updateSequential() {
	sum.set(beat * 2 + bpm)
}
