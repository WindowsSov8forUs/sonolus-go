# 编译器架构

## 总览

```text
package patterns
    -> source.LoadMode
    -> packages.Package
    -> frontend.Parser
    -> typed CFG IR
    -> optimize.Optimizer
    -> backend
    -> compiler.Artifacts
    -> build

development LevelFile
    -> level loader (play/watch/preview)
    -> archetype/import validation
    -> Sonolus dev server
```

根调度器是 `internal/compiler.Compiler`。CLI 只依赖该根包门面，不直接调用 frontend、IR 或 backend。

CLI 先通过 `compiler.DiscoverTargets` 将 package patterns 展开为稳定排序的 engine main package。未指定 `-o` 时，每个目标使用 module path 最后一段作为名称并由独立 Compiler 编译；产物固定写入 `dist/<name>`。

开发 LevelData 是 `dev` 的独立输入，不进入 Compiler IR 或 `compiler.Artifacts`。`internal/level` 解析共享 embed 声明并对三种普通关卡模式校验；`internal/devserver` 将成功的引擎与关卡快照装配到 `sonolus-server-go`，并使用内置 free-pack 资源提供完整开发路由。

`list` 使用独立的 declaration-only frontend 路径读取 Play、Watch、Preview archetype 字段，不 lower callback。字段来源 Contract 同时供 Py 兼容 schema 投影和 Development Level 的逐模式 imports 校验使用。

## Compiler 快照

`Compiler` 在创建时固定 package patterns、优化等级和 fallback ROM。它是单一源码快照：

- `Compile(modes...)` 编译请求模式，重复模式去重。
- `CompileAll()` 按 Play、Watch、Preview、Tutorial 顺序请求全部模式。
- 已成功模式不会重新加载。
- 新增模式只有在 load、frontend、共享验证、优化和 backend 全部成功后才原子提交。
- 返回值是深拷贝，调用方修改不会污染缓存。
- `Stats()` 返回加载、frontend、优化、backend 与总耗时。
- `SourceFiles()` 返回成功快照涉及的 Go 和 embed 文件。

同一 Compiler 的并发调用通过互斥锁串行提交；不同模式的首次加载在内部并行执行。

## Source

`source.LoadMode` 为每个模式单独调用 `packages.Load`，设置 `-tags=<mode>`。这样模式文件可以定义相同 Go 名称而互不冲突。

加载规则：

- 只递归扫描用户 main module。
- `sonolus` 桩包是 catalog leaf，不扫描其 AST。
- 允许的标准库是 intrinsic leaf，不扫描其内部依赖。
- 禁止 dot import、非白名单标准库和第三方包。

`source.ASTTracer` 负责通用静态值追踪。静态可确定的直接函数调用表示为 symbolic call，但 tracer 不执行函数，也不判断该调用是否为合法 Sonolus intrinsic。资源、Configuration 和 ROM consumer 决定是否接受。

## Frontend

每种模式拥有独立 session，包括 `packages.Package`、FileSet、AST、`go/types` 对象图和 tracer。这些对象绝不跨模式比较。

Frontend 分为：

1. 扫描 directive、marker 和 singleton。
2. 收集并规范化静态资源。
3. 构建 archetype storage layout 和资源 ID lookup。
4. 校验 Configuration 与 ROM。
5. 识别 callback，并将 callback body lower 为强类型 CFG IR。

`frontend.Parser.GetProject()` 比较所有已解析模式的 Configuration 和 ROM。Configuration 按规范化语义比较，ROM 按最终用户 float32 字节比较；声明变量名和文件路径不影响相等性。

## Catalog

Catalog 是公开 Sonolus API 的唯一语义来源，记录：

- 包路径、对象名称、签名和类型 layout。
- 可用模式与 callback 阶段。
- effect、读写权限和 RuntimeFunction。
- receiver/参数展开及返回值重组。
- semantic memory storage 和资源 handle 规则。
- 静态声明专用、callback 可用或明确禁止分类。

Frontend 按准确 `go/types.Object` 查 catalog，不依赖 import alias 或源码文本猜测。

## IR

`internal/compiler/ir` 是与 Go frontend 解耦的强类型 CFG：

- Function 由显式 Block 组成。
- Terminator 为 Jump、Branch、Switch、Return 或 Unreachable。
- Value 带 DSL Type 和有序 scalar slots。
- Place 表示 virtual local、indexed local 或 semantic memory。
- RuntimeCall 保存 RuntimeFunction、参数顺序、返回 layout、effect 和轻量 SourcePos。

IR 不导入 AST、`go/token`、`go/types`、packages、frontend 或 catalog。

`ir.Builder` 负责 block/local 编号、load/store、RuntimeCall 和构建期检查。`ir.Validate` 接受优化中间形式；`ir.ValidateFinal` 要求 backend 可消费的已分配形式。

## Optimize

Optimizer 深拷贝每个 callback IR，并执行所选 pipeline。分析缓存只存在于单次 Optimize 调用，不跨 callback 或并发共享。详情见 [优化器](optimization.md)。

Temporary Memory 上限为 4096 slots。allocation 将 virtual local 重写为物理 Temporary Memory layout；backend 拒绝残留 SSA、Phi 或未分配 local。

## Backend

Backend 只依赖规范化 `frontend.Project` 和 final-form IR，不依赖 AST、`go/types` 或 packages。

每个模式：

1. 验证 callback IR final form。
2. 将 semantic memory place 映射到模式 memory block。
3. 将 CFG finalization 为 Sonolus RuntimeFunction 控制流树。
4. 自底向上执行 SNode peephole。
5. 子节点优先写入、确定性去重并生成 EngineData node pool。
6. 组装 archetype callback index/order 和模式静态资源。

ROM 最终布局固定为 NaN、`+Inf`、`-Inf` 三个 float32 前缀，之后连接用户 ROM。

## Artifacts 与输出

`compiler.Artifacts` 包含：

```go
type Artifacts struct {
	Configuration *resource.EngineConfiguration
	ROM           []byte
	Play          *resource.EnginePlayData
	Watch         *resource.EngineWatchData
	Preview       *resource.EnginePreviewData
	Tutorial      *resource.EngineTutorialData
}
```

`internal/build` 将存在的模式产物 gzip 后原子写入目录。
