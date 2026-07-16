# 优化器

## 目标与契约

`internal/compiler/optimize` 接受强类型 CFG IR，返回深拷贝后的 final-form IR。它不认识 AST、`go/types`、frontend、backend 或 Sonolus 物理 memory block。

每个 pass 后执行 `ir.Validate`，pipeline 结束执行 `ir.ValidateFinal`。错误包含 mode、callback、function 和 pass 名称。

默认等级是 Standard。CLI 映射：

| `-O` | 等级 | 用途 |
|---|---|---|
| `0` | Minimal | 最少且保守的结构清理，便于调试 |
| `1` | Fast | 不进入 SSA 的快速编译 |
| `2` | Standard | 完整优化，默认 |

所有等级必须保持 callback 语义一致。

## Minimal

主要步骤：

1. 常量 Branch/Switch 折叠。
2. 删除不可达 block。
3. 合并简单控制流。
4. 删除明确 no-op。
5. 顺序分配 local。
6. 稳定重编号。

Minimal 不建立 SSA，不进行跨 block 数据流优化。顺序分配超过 4096 Temporary Memory slots 时直接失败。

## Fast

主要步骤：

```text
CoalesceFlow
RemoveUnreachable
TryAllocateBasic
CoalesceFlow
RenumberBlocks
```

Fast 首先尝试顺序分配；超过限制时使用保守活性复用。它不进入 SSA，适合开发期追求更短编译时间且需要比 Minimal 更好的内存分配时使用。

## Standard

Standard 以 `sonolus.py@1040bc0` 的 pass 语义和顺序为基线，包含：

- CFG 清理和小条件块合并。
- ToSSA、两轮 SCCP、FromSSA。
- 普通与高级 DCE。
- variable inline、aggressive inline。
- associative flatten/unflatten 和 redundant argument removal。
- if-chain 到 switch 重写、switch/exit 规范化。
- LICM、CSE、copy coalescing。
- 基于活性干涉图的确定性 first-fit allocation。

只提升可静态寻址的 scalar local slot。动态索引 aggregate 保持 memory 形式，避免错误 alias 推断。

## 副作用与数值

优化只处理 local 和 catalog 明确标记为 pure 的 RuntimeCall。semantic memory、动态索引和非纯调用采用保守规则，不跨副作用读取、删除或重排。

SCCP 不折叠随机调用或未确认与 Sonolus Runtime 数值行为一致的运算。NaN、Inf 和 Runtime 浮点细节不能按普通 Go 常量语义擅自推断。

## Allocation

Temporary Memory 硬限制为 4096 slots：

- Minimal：声明顺序连续分配。
- Fast：优先 basic，必要时保守活性复用。
- Standard：按 local size 与稳定 ID 排序，对干涉图确定性 first-fit。

无法装入时错误会报告 callback、所需 slots 和 4096 上限，不自动降低优化等级。

## Backend SNode peephole

IR optimizer 之后，backend 始终执行以 `sonolus.js-compiler@37b0eee` 为基线的 SNode peephole，包括算术单位元、常量组合、Get/Set shifted 互换、SetAdd 等融合和控制流尾值清理。

被代数消去但可能有副作用的动态参数会用 `Execute` 保留求值，确保严格的左到右副作用顺序。

## 差分与回归

仓库保存固定 Py pass snapshot 和 JS SNode golden。普通 Go CI 只读取 checked-in golden，不依赖相邻 Python/Node checkout。更新固定参考版本时，必须显式运行对应 testdata regeneration script并审核差异。
