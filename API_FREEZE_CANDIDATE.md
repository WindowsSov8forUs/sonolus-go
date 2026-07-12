# Sonolus Go API Freeze Candidate

此文件临时记录 `sonolus` 引擎框架 API 与 `Package -> IR` frontend 的冻结边界。
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
- callback 支持已登记的 Go DSL 冻结子集，包括局部值、复合值、控制流、定长数组、
  catalog 容器、立即闭包以及静态可确定的跨包、方法和泛型 helper；
- callback 输出为 `internal/newcompiler/ir` 的强类型 CFG，block、local、place、
  terminator、RuntimeFunction 调用和源码位置均不依赖 AST 或 `go/types`；
- catalog recipe 是高层 API lowering 的唯一语义入口；公开 callback recipe 和
  public native 均具有 `Package -> IR` 覆盖，native 显式适用于四模式及全部 callback
  阶段；
- 动态函数值、接口派发、递归、无法静态确定的调用目标、运行时容器 backing 选择及
  其他未登记 Go 特性以稳定错误拒绝，不延后到后续阶段猜测。

以上冻结约束源码声明兼容性以及 `Package -> IR` 的结构和 lowering 语义，不承诺桩函数
具有普通 Go 运行时行为。

## 尚未冻结

以下内容仍可在后续阶段调整，因此当前不属于稳定 API 承诺：

- SSA 转换、IR optimization 与 finalization；
- IR 到 `EngineDataNode` 以及各模式最终 EngineData 的生成；
- 不改变已冻结公开契约的内部 IR 重构和优化实现细节；
- NaN、Inf、溢出、随机数及其他 Runtime 与普通 Go 的数值差异；
- CLI 对 `newcompiler` 的接入方式；
- 最终稳定版的兼容性与版本策略。

## 转为正式稳定版的条件

- Py、JS、Wiki 与 RuntimeFunction 对照不存在未分类差异；
- 至少一个完整引擎通过 `newcompiler` 生成并验证可运行 EngineData。
