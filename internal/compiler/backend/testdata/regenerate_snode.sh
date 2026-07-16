#!/usr/bin/env bash
# Regenerates snode_golden.json from the real sonolus.js-compiler sources.
#
# Requires Node >= 22.6 (native TypeScript type stripping) and a local checkout
# of sonolus.js-compiler. The optimizer/assembler files import @sonolus/core
# only as types, so we copy that subtree, strip the type-only imports, rewrite
# the .js import specifiers to .ts, and run a harness that dumps fixtures.
set -euo pipefail

SRC="${1:-../../../../sonolus.js-compiler/src}"   # path to sonolus.js-compiler/src
HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
REF="$REPO/.goldtmp"

rm -rf "$REF"; mkdir -p "$REF/snode"
cp -r "$SRC/snode/nodes" "$REF/snode/nodes"
cp -r "$SRC/snode/optimize" "$REF/snode/optimize"
cp "$SRC/build/shared/assemble.ts" "$REF/assemble.ts"

find "$REF" -name '*.ts' -print0 | while IFS= read -r -d '' f; do
  sed -i "/from '@sonolus\/core'/d" "$f"
  sed -i "s/\.js'/.ts'/g" "$f"
  sed -i \
    -e "s/^import { Func } from/import type { Func } from/" \
    -e "s/^import { Value } from/import type { Value } from/" \
    -e "s/^import { SNode } from/import type { SNode } from/" \
    -e "s/^import { OptimizeFunc } from/import type { OptimizeFunc } from/" \
    -e "s/^import { OptimizeFunc, optimizeSNode } from/import { type OptimizeFunc, optimizeSNode } from/" \
    "$f"
done
sed -i "s#'\.\./\.\./snode/#'./snode/#g" "$REF/assemble.ts"

cp "$HERE/harness.ts" "$REF/harness.ts"
node "$REF/harness.ts" > "$HERE/snode_golden.json"
echo "wrote $HERE/snode_golden.json"
