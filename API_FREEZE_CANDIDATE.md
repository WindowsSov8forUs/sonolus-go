# Sonolus Go API Freeze Candidate

此文件临时记录 `sonolus` 引擎框架 API 的冻结边界。它将在 callback lowering
完成并形成正式 API 文档后删除或替换。

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
- `math`、`math/rand` 有限 intrinsic，以及禁止其他标准库和第三方依赖的策略；
- catalog 作为公开 API、native RuntimeFunction 和固定布局的唯一可审计索引；
- frontend 以 `go/types.Object` 身份区分 Sonolus API、标准库 intrinsic 和用户对象。

以上冻结仅约束源码声明兼容性，不承诺桩函数具有普通 Go 运行时行为。

## 尚未冻结

以下内容仍可在后续阶段调整，因此当前不属于稳定 API 承诺：

- callback body 到 IR/nodes 的 lowering 及优化语义；
- 每个高层或 native API 的最终 RuntimeFunction 展开方式；
- lowering 验证后需要修正的 façade 参数、返回值、模式或 callback 阶段；
- NaN、Inf、溢出、随机数及其他 Runtime 与普通 Go 的数值差异；
- CLI 对 `newcompiler` 的接入方式；
- 最终稳定版的兼容性与版本策略。

## 转为正式冻结的条件

- catalog 中每个公开符号均有 lowering 或明确的 compile-time-only 分类；
- 四模式逐 API lowering golden 全部通过；
- native 的模式和 callback 阶段矩阵完成审计；
- Py、JS、Wiki 与 RuntimeFunction 对照不存在未分类差异；
- 至少一个完整引擎通过 `newcompiler` 生成并验证可运行 EngineData。
