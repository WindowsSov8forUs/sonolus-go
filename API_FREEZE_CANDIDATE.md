# Sonolus Go API Freeze Candidate

此文件记录 `sonolus` 引擎框架 API、callback Go DSL 与
`Package -> IR -> EngineData` 的冻结边界。
它将在形成正式 API 和编译器架构文档后删除或替换。

## 当前冻结范围

以下契约视为 freeze candidate。除修复与 Sonolus Runtime、`sonolus.py` 或
`sonolus.js-compiler` 不一致的行为外，不再随意重命名或重组：

- `sonolus` 根包以及 `play`、`watch`、`preview`、`tutorial`、`native` 子包的
  包结构；
- 名称型资源 handle 与构建函数：
  - `Sprite` / `SkinSprite(name)`；
  - `Clip` / `EffectClip(name)`；
  - `Effect` / `ParticleEffect(name)`；
  - `Text` / `InstructionText(name)`；
  - `Icon` / `InstructionIcon(name)`；
- `//sonolus:resource` 资源声明方式、字段及定长数组的声明顺序即资源 ID 顺序；
- skin、effect、particle、instruction、instruction icon 与 bucket 的独立静态模型；
- configuration、ROM、archetype field、callback order 和 global callback 的声明形式；
- Configuration 字段统一使用 `configuration` struct tag；其中 `ui` 与
  `replayFallback` 从 configuration singleton 的静态初始化值构建；
- `math`、`math/rand` 有限 intrinsic，以及禁止其他标准库和第三方依赖的策略；
- catalog 作为公开 API、native RuntimeFunction 和固定布局的唯一可审计索引；
- frontend 以 `go/types.Object` 身份区分 Sonolus API、标准库 intrinsic 和用户对象；
- `frontend.Parser.Parse(mode, pkg)` 以模式为原子单位完成静态声明与 callback
  lowering，`GetProject` 负责共享 Configuration/ROM 验证与多模式聚合；
- callback 支持的声明和表达式包括常量、局部变量、短变量声明、普通/复合/多重赋值、
  自增自减、标量及已登记 struct/定长数组复合值、字段/定长数组索引、算术、比较、
  逻辑短路和显式数值转换；
- callback 支持的控制流包括 `if/else`、三段式及条件 `for`、`int` 整数、定长数组、
  variadic helper 参数和 catalog 容器 `range`、tagless/constant-tag `switch`、
  `break`、`continue` 和 `return`；
- helper 必须能从源码静态确定目标；支持跨用户包函数、具体方法、泛型实例、立即调用
  闭包、命名/多返回值、裸 `return` 和静态 variadic 参数，并以内联语义严格保持 Go
  的左到右求值顺序；variadic 参数仅支持 `len/cap`、索引、`range` 和静态转发；
- 局部函数变量可唯一初始化为具名 user helper 或函数 literal，并可作为静态 helper
  参数传递；它们不占运行时 slot、不得重赋值、逃逸或在运行时控制流中选择目标；
- catalog 容器仅支持已登记的 `VarArray`、`ArrayMap`、`ArraySet` constructor、方法和
  固定 capacity/backing layout；资源 handle、EntityRef、archetype storage、native、
  `math` 与 `math/rand` 仅按 catalog recipe 使用；
- callback 输出为 `internal/compiler/ir` 的强类型 CFG，block、local、place、
  terminator、RuntimeFunction 调用和源码位置均不依赖 AST 或 `go/types`；
- catalog recipe 是高层 API lowering 的唯一语义入口；公开 callback recipe 和
  public native 均具有 `Package -> IR` 覆盖，native 显式适用于四模式及全部 callback
  阶段；
- `internal/compiler/backend` 将优化后的强类型 CFG 确定性转换为四模式
  EngineData、RuntimeFunction node pool 和带非有限常量前缀的 float32 ROM；
- `compiler.Compiler` 调度 mode build tags、frontend、三级 optimizer 与 backend，
  CLI 的 `build/serve/pack` 直接接收 package patterns 并只调用该链路；
- 运行时选择的函数值、函数返回值、package 级函数变量、接口派发、直接或间接递归、
  variadic 参数的修改或逃逸、运行时容器 backing 选择、无法静态确定的调用目标、
  未登记 builtin、map/slice/string 运行时操作、类型
  断言、反射、goroutine、channel、`defer` 及其他未登记 Go 特性均以稳定错误拒绝。
  这些拒绝属于 DSL 契约，不是待补功能。

以上冻结约束源码声明兼容性、`Package -> IR` lowering 语义、三级 optimizer/backend 产物语义
以及 package-pattern CLI 输入契约，不承诺桩函数具有普通 Go 运行时行为。

## 尚未冻结

以下内容仍可在后续阶段调整，因此当前不属于稳定 API 承诺：

- 不改变已冻结公开契约的内部 IR 重构和优化实现细节；
- NaN、Inf、溢出、随机数及其他 Runtime 与普通 Go 的数值差异；
- 最终稳定版的兼容性与版本策略。

## 转为正式稳定版的条件

- Py、JS、Wiki 与 RuntimeFunction 对照不存在未分类差异；
- 至少一个真实完整引擎通过 `compiler` 生成并在 Sonolus Runtime 验证运行行为。
