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

仓库根目录下的 `godori/` 是完整链路的长期验收工程：同一组源码覆盖 Play、Watch、Preview、Tutorial、Development Level 和 CLI dev server。它参考 `sonolus.py@1040bc0dcc116efdbca05f144edec302e839bcd3` 的 pydori 设计，包含 Tap、Flick、Directional Flick、以 abstract note 和 `prev`/`next` 统一链表示的任意 anchor Hold、同时押宽 hitbox 切分、99999 实体容量的回放 stream 与分段 Hold 音效、组合 judgment bucket、动态管理 archetype、BPM/Timescale、projective stage transform、统一 layer z 规则、音符边界淡入淡出、周期 Flick 箭头、screen-space affine particle 校正、Preview 与 Tutorial，并使用结构化 level globals 复用公共状态布局。

CLI 先通过 `compiler.DiscoverTargets` 将 package patterns 展开为稳定排序的 engine main package。未指定 `-o` 时，每个目标使用 module path 最后一段作为名称并由独立 Compiler 编译；产物固定写入 `dist/<name>`。

`init` 不进入 Compiler 链路。`internal/scaffold` 负责无覆盖地生成最小四模式 package、新 module metadata 和编辑器配置；CLI 只解析目标目录、module path 与依赖版本。生成过程不访问网络，依赖解析由用户随后执行的 `go mod tidy` 完成。

开发 LevelData 是 `dev` 的独立输入，不进入 Compiler IR 或 `compiler.Artifacts`。`internal/level` 解析共享 embed 声明并对三种普通关卡模式校验；`internal/devserver` 将成功的引擎与关卡快照装配到 `sonolus-server-go`，并使用内置 free-pack 资源提供完整开发路由。

`sonolus/level` 是宿主侧类型化 LevelData builder，与 `sonolus/sim` 一样不属于 callback catalog。它把普通 Go struct、定长数组和类型化实体引用展开为 `resource.LevelData`；Godori 的 `levelgen` 使用它生成 checked-in `dev-level.json`。生成与加载是两条独立路径：builder 负责构造，`internal/level` 仍以最终 JSON 和三模式声明契约为权威校验。

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

`LevelMemoryResource` 与 `LevelDataResource` 是 declaration-only marker。Frontend 按稳定声明顺序为每个模式的多个 singleton 分配共享 semantic memory layout，并为 nested record、定长数组和 container 建立路径化 layout tree；selector 与动态数组索引沿 descriptor 派生对应 `MemoryPlace`。容器字段只在 IR 中保留 size slot 和 backing descriptor，不把完整 backing 物化到 Temporary Memory。LevelData 的 preprocess-only 写权限同时供 IR validator 与 optimizer readonly oracle 使用。

Callable、pointer、catalog container 与 static interface 的 runtime 分支合并统一表示为有限变体：一槽 Temporary Memory tag 加按源码首次出现排序的静态 alternatives。各 alternative 保留独立 callable 捕获环境与具体泛型 type substitution、pointer place set、container backing descriptor 或 concrete interface payload；helper 参数/返回与 dispatch 只生成 CFG，不在最终 IR 中留下 Go 函数、pointer、container descriptor 或 interface 对象。Callable cell 和一维固定 callable 数组在赋值、参数、返回与 value range 时复制当前 tag，而不是共享后续重绑；package callable 数组从静态 initializer 重建为不可变函数表。普通 package scalar、runtime record 与定长数组通过 ASTTracer 验证 pure static initializer，再按 runtime layout 重建不可变值；它们不占用长期 semantic memory。Static struct value 额外保留 literal 字段的显式性，使 Configuration 能在不吞掉显式零值的前提下应用 Py 默认 UI。Iterator 创建时冻结 descriptor tag，后续变量重绑不会改变既有 iterator 的目标。Entity view 在本地 aggregate 中同样只展开为一槽 entity index。单次注册的 defer 使用函数级 active cell 汇入统一 return block 并逆序内联；可能重复注册的 defer 不建立运行时栈。

## IR

`internal/compiler/ir` 是与 Go frontend 解耦的强类型 CFG：

- Function 由显式 Block 组成。
- Terminator 为 Jump、Branch、Switch、Return 或 Unreachable。
- Value 带 DSL Type 和有序 scalar slots。
- Place 表示 virtual local、indexed local 或 semantic memory；嵌套动态 entity/array index 在 frontend 中规范化为可组合 affine address。
- RuntimeCall 保存 RuntimeFunction、参数顺序、返回 layout、effect 和轻量 SourcePos。

IR 不导入 AST、`go/token`、`go/types`、packages、frontend 或 catalog。

`ir.Builder` 负责 block/local 编号、load/store、RuntimeCall 和构建期检查。`ir.Validate` 接受优化中间形式；`ir.ValidateFinal` 要求 backend 可消费的已分配形式。

## Optimize

Optimizer 深拷贝每个 callback IR，并执行所选 pipeline。callback 使用不超过 `GOMAXPROCS` 的有界 worker pool 并行优化，结果与错误仍按稳定 job 顺序归并。分析缓存只存在于单次 Optimize 调用，不跨 callback 或并发共享；未声明契约的 pass 按保守策略清空缓存，CopyCoalesce clone 并重映射干涉图，后续只删除指令/CFG 的 pass 可保留该 over-approximation 供 Allocate 复用。若该保守图无法满足 4096 slots，Allocate 会基于当前最终 CFG 重算精确 liveness/interference 后重试；精确图仍超限才返回包含失败 local 的稳定错误。liveness 与 interference graph 使用固定宽度 bitset。LICM/CSE 只把 catalog 按 mode/callback 判定为 readonly 的 semantic memory 作为候选，RuntimeUI 等存在同阶段 setter 的 storage 保持屏障。详情见 [优化器](optimization.md)。

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

源码显式声明 ROM、提供 fallback，或优化后的 callback IR 实际读取 ROM 时，backend 才生成 ROM。其最终布局固定为 NaN、`+Inf`、`-Inf` 三个 float32 前缀，之后连接用户 ROM；否则 `Artifacts.ROM` 为 nil，build 不写出 `EngineRom`。

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
	Diagnostics   map[int]string
}
```

`internal/build` 将存在的模式产物 gzip 后原子写入目录。

`sonolus/sim` 消费相同最终 Artifacts，并通过不依赖 compiler 门面的 `internal/simexec` 直接解释各模式 EngineData node pool；reference tests、Py final-tree 差分和公开 simulator 共用这一个 executor。它不绕过 optimizer/backend，因此用于比较三个优化等级的 callback 返回值、semantic memory、streams 与副作用顺序。Temporary Memory 的物理编号与内容不属于跨优化等级的可观察语义。

Catalog generator 为完整 core RuntimeFunction inventory 生成唯一 simulation metadata，包括 class、arity/result slots、effect、strategy、特殊 shape 标记与 memory/stream 参数角色；同一 RuntimeFunction 的 facade recipe 不得产生写入语义冲突。Simulator 在访问参数节点前按该生成策略验证 arity/shape 和参数，保留 IEEE NaN/Inf，并对非法 node、argument/state、memory/stream index、缺失 handler 与 step limit 返回可由 `errors.As` 获取的 `sim.ExecutionError`，不得因畸形 EngineData panic。
