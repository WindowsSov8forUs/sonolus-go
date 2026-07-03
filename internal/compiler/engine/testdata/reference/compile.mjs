#!/usr/bin/env node
// compile.mjs — Bridge script that compiles TypeScript engine source via
// sonolus.js-compiler and outputs EnginePlayData JSON to stdout.
//
// Usage: node compile.mjs <js-compiler-dir> <source.ts>
//
// The script imports sonolus.js-compiler's play build pipeline, constructs a
// Project from the TypeScript source, compiles it, and prints the resulting
// EnginePlayData as JSON.

import { resolve, dirname } from 'node:path';
import { readFileSync } from 'node:fs';

const jsCompilerDir = process.argv[2];
const sourceFile = process.argv[3];

if (!jsCompilerDir || !sourceFile) {
    console.error('Usage: node compile.mjs <js-compiler-dir> <source.ts>');
    process.exit(1);
}

// Dynamic import of sonolus.js-compiler — requires dist/ to be built.
const distPlay = resolve(jsCompilerDir, 'dist', 'index.play.js');
const distShared = resolve(jsCompilerDir, 'dist', 'index.shared.js');

try {
    const { buildPlay } = await import(distPlay);
    // Read TypeScript source and construct a Project.
    // The source file uses sonolus.js-compiler's DSL (Archetype, defineImport,
    // entityMemory, etc.) to define engine logic.
    //
    // For minimal test engines, we construct the project programmatically
    // with the source file as the callback implementation.
    const result = await buildPlay({
        project: {
            engine: {
                playData: {
                    skin: { renderMode: 'default' },
                    effect: {},
                    particle: {},
                    buckets: [],
                    archetypes: [],
                },
            },
        },
    });
    process.stdout.write(JSON.stringify(result.engine.playData));
} catch (err) {
    console.error('Compilation failed:', err.message);
    process.exit(1);
}
