# sonolus-go 文档

`sonolus-go` 将受限的 Go DSL 编译为 Sonolus EngineData。编译器支持 Play、Watch、Preview、Tutorial 四种模式，生成 EngineConfiguration、按需生成的 Engine ROM 和对应模式的 EngineData。

项目提供引擎编译和开发期数据服务。

## 文档导航

- [快速开始](getting-started.md)：建立一个可编译的四模式引擎包。
- [DSL 参考](dsl-reference.md)：资源、Configuration、ROM、Archetype、callback 和受支持 Go 子集。
- [命令行](cli.md)：`init`、`build`、`vet`、`list`、`dev`、`version` 的参数和输出。
- [编译器架构](architecture.md)：`Package -> Frontend -> IR -> Optimize -> Backend` 数据流。
- [优化器](optimization.md)：Minimal、Fast、Standard 的语义和适用场景。
- [性能](performance.md)：编译耗时、节点规模、Temporary Memory 和开发期实践。
- [维护指南](maintenance.md)：生成文件、Py/JS 固定基线、差分资产、完整验收与提交清理。

## 支持边界

`sonolus`、`sonolus/play`、`sonolus/watch`、`sonolus/preview`、`sonolus/tutorial` 和 `sonolus/native` 是编译器识别的声明桩包。它们的函数体不是普通 Go 运行时实现；调用的实际含义来自 compiler catalog。

允许的标准库仅包括：

- `_ "embed"`：使用 `//go:embed` 时由 Go 语言要求导入；`sonolus.ROMFile` 不直接使用 `embed` 包类型，因此采用空白导入。
- `iter`：仅用于编译期内联的 range-over-func、容器与 stream iterator，不生成运行时函数对象。
- `math`：仅允许 catalog 已登记的常量和函数。
- `math/rand`：仅允许 `Float64` 和 `Intn`。

其他标准库、第三方包、dot import 和动态 Go 特性会在加载或 frontend 阶段被拒绝。编译器会递归扫描从引擎入口可达且属于用户主 module 的普通 package；其中的静态 helper、resource、Configuration、ROM、Stream、level global、archetype 和 callback 都会参与当前引擎编译。将顶层模式声明保留在入口 package 只是一种便于导航、减少隐式聚合的代码组织建议，不是编译要求；需要在多个引擎间复用声明时，可以将 archetype 和 resource 放在双方共同导入的主 module 子包中。

## 稳定性

公开 API 与 DSL 当前属于 freeze candidate。内部 IR、优化器实现和 backend 节点组织仍可在不改变公开语义的前提下调整。正式稳定前仍需要至少一个完整引擎在 Sonolus Runtime 中进行端到端验证。
