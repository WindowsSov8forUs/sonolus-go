# 维护指南

本文面向维护编译器、公开 DSL、差分资产与 Godori 验收工程的贡献者。用户侧用法见[快速开始](getting-started.md)与 [DSL 参考](dsl-reference.md)。

## 事实来源与固定基线

- 当前编译器入口为 `internal/compiler`，不得恢复旧 compiler 或 `internal/newcompiler` 链路。
- Py optimizer 基线固定为 `sonolus.py@1040bc0dcc116efdbca05f144edec302e839bcd3`。
- JS Runtime/SNode 基线固定为 `sonolus.js-compiler@37b0eee5aa16d1e01973d33d625d86f5ef72d268`。
- Catalog 是 callback API、layout、阶段权限、effect 与 lowering recipe 的唯一事实来源。
- Godori 是实际四模式黑盒验收工程；`internal/compiler/testdata` 负责隔离的语法、IR、差分与稳定拒绝测试。

普通测试只读取 checked-in golden，不要求相邻的 Python 或 JavaScript checkout。只有明确更新固定基线或已审核实现语义变化时才运行 regeneration。

## 修改影响面

修改公开 callback API 时同步检查：

1. `sonolus` 声明桩及对应模式 facade。
2. Catalog symbol、recipe、阶段权限和 simulation metadata。
3. Frontend 成功 lowering 与稳定拒绝 fixture。
4. IR/backend 或最终 EngineData 语义覆盖。
5. `docs/dsl-reference.md`。

修改 Archetype、Configuration、资源或 typed global 声明时，还需检查 declaration-only schema、Development Level 验证和四模式共享规则。修改 RuntimeFunction 或 SNode 语义时，还需检查 simulator inventory、JS golden 与被消去表达式的副作用保留。

## 生成文件

Catalog 或公开声明源发生变化后运行：

```bash
go generate ./internal/compiler/catalog
```

该命令更新 catalog、native facade、标准名称和 native coverage fixture。所有生成差异都应与源声明修改处于同一提交。

Godori 关卡源码位于 `godori/internal/leveldata`。修改后运行：

```bash
go generate ./godori
```

`godori/godori_test.go` 会逐字节检查 `godori/dev-level.json` 是否与 builder 输出一致。

## Py optimizer 差分

`internal/compiler/testdata/optimize/pipeline_fixture.json` 是 schema v2 中立 CFG 输入；Go 与 Python 分别解析它。生成的 `py_pass_golden.json` 是 schema v4 snapshot，覆盖 ToSSA、第一轮 SCCP cleanup、第二轮 SCCP、FromSSA、Allocate 和完整 Standard 输出。

更新固定 Py 基线或经审核的 pipeline 语义时运行：

```powershell
& internal/compiler/testdata/optimize/regenerate.ps1 -PythonCheckout ..\sonolus.py
```

结构等价但规范化 CFG 不同的条目必须精确写入 `py_pass_allowlist.json`，包含 case、checkpoint、JSON pointer、Go/Py 值与原因。未知差异和失效 allowlist 都会使测试失败；不得通过放宽全局比较处理。

## Backend 与 JS golden

更新四模式 reference audit 与 JS RuntimeFunction 数值基线：

```powershell
& internal/compiler/testdata/backend/regenerate.ps1 `
    -PythonRepo ..\sonolus.py `
    -JavaScriptRepo ..\sonolus.js-compiler
```

更新直接来自 JS compiler 的 SNode golden：

```bash
internal/compiler/testdata/backend/regenerate_snode.sh ../sonolus.js-compiler/src
```

两个脚本都会以固定 commit 为前提。`reference.golden.json` 是审核后的 Go 四模式语义资产；`runtime_native_golden.json` 与 `snode_golden.json` 是固定 JS 实现的直接输出，三者不能混为同一种证据。

## 验收层级

局部修改先运行最窄的相关测试。涉及公开链路、IR、优化器、backend、并发、CLI 或 Godori 时，提交前运行：

```bash
go test -race -count=1 ./...
go vet ./...
go build ./...
gofmt -l .
git diff --check

go run ./cmd/sonolus-go vet -O 0 ./godori
go run ./cmd/sonolus-go vet -O 1 ./godori
go run ./cmd/sonolus-go vet -O 2 ./godori
go run ./cmd/sonolus-go list ./godori
```

`gofmt -l .` 必须无输出。不要用节点数单独证明语义正确；优化修改还需通过 simulator 或 reference corpus 比较 callback 结果、semantic memory、streams 与副作用顺序。

## 提交与发布前清理

- 按公开 API、frontend/IR、optimizer、Godori、文档等功能边界组织提交，不混入本地 benchmark、生成暂存目录或编辑器文件。
- 检查 `git status --short`，确认 `.goldtmp`、`dist`、测试二进制和本地 Agent 配置仍由 `.gitignore` 排除。
- 检查 `rg 'newcompiler|internal/compiler/engine|internal/compiler/snode'`，避免重新引入已删除链路。
- 不因 regeneration 方便而更新固定 Py/JS commit；基线升级必须单独审核实现差异、golden 和 allowlist。
- 发布候选除自动化验收外，还应使用真实 Sonolus 客户端检查 Godori Play、Watch、Preview 与 Tutorial。单 callback simulator 不替代完整 Runtime 生命周期验证。
