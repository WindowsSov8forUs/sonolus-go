// TypeScript equivalent of control_flow/engine.go.
import { Archetype, entityMemory, defineImport } from '@sonolus/sonolus.js-compiler/play'

const archetype = new Archetype('N')
const count = entityMemory(Number)
const flag = entityMemory(Number)
defineImport(count)
defineImport(flag)

export function initialize() {
	count.set(0)
	flag.set(0)
}

export function updateSequential() {
	for (let i = 0; i < 5; i++) {
		if (i > 2) {
			count.set(count.get() + 1)
		}
	}
	if (count.get() > 3) {
		flag.set(1)
	}
}
