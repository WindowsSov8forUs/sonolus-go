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

等级不保证性能单调：Fast 的 pipeline 不是 Minimal 的超集，Standard 也不保证每个 callback 的动态执行成本都低于前两档。节点池大小、运行时执行步数和编译成本应分别测量。

## Minimal

主要步骤：

1. 常量 Branch/Switch 折叠。
2. 删除不可达 block。
3. 合并简单控制流。
4. 删除明确 no-op。
5. 顺序分配 local。
6. 稳定重编号。

Minimal 不建立 SSA，不运行全局数据流 pipeline；控制流清理可以利用相邻分支边的已知条件。顺序分配超过 4096 Temporary Memory slots 时直接失败。

## Fast

主要步骤：

```text
CoalesceFlow
RemoveUnreachable
TryAllocateBasic
CoalesceFlow
RenumberBlocks
```

Fast 首先尝试顺序分配；超过限制时使用活性复用。快速布局只追加到已放置冲突区间之后；若仍超限，则复用同一干涉图重试 first-fit，避免因忽略空位而错误拒绝可承载的布局。它不进入 SSA，适合开发期追求更短编译时间且需要比 Minimal 更好的内存分配时使用。

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

Go 在 CopyCoalesce 前额外执行一次 AdvancedDeadCodeElimination，删除会制造无效干涉的死写入；合并后的原有清理继续删除新产生的死复制。这个顺序差异来自 Go 保留完整 allocation 干涉图的实现，不能只照搬 Py 基于存活集合的 copy 图规则。SSA 消除与控制流合并之后，再执行一次受成本约束的 InlineVars 与 AdvancedDeadCodeElimination，清理重复条件使用消失后留下的单次快照。

`InlineVars` 的普通和 aggressive 形式均排除存在动态索引访问的整个 local，包括通过赋值地址表达式间接读取的数组。动态写入可能覆盖任意固定槽，不能只统计 `LocalPlace` 赋值就认定该槽只有一个定义。其余普通 local 只有在唯一赋值支配所有读取、且同 block 内赋值严格早于读取时才能替换；赋值右侧与地址表达式都属于写入前的读取。SSA 与合法的普通 local 内联继续保留。

## 副作用与数值

优化只处理 local 和 catalog 明确标记为 pure 的 RuntimeCall。semantic memory、动态索引和非纯调用采用保守规则，不跨副作用读取、删除或重排。

SCCP 不折叠随机调用或未确认与 Sonolus Runtime 数值行为一致的运算。NaN、Inf 和 Runtime 浮点细节不能按普通 Go 常量语义擅自推断。

## Allocation

Temporary Memory 硬限制为 4096 slots：

- Minimal：声明顺序连续分配。
- Fast：优先 basic，必要时保守活性复用。
- Standard：按 local size 与稳定 ID 排序，对干涉图确定性 first-fit。

活跃性分析识别同一 block 内不读取旧值的完整 aggregate 槽位覆盖，将旧生命周期截断在该覆盖组的首次写入处。单次部分或动态写入也可开始生命周期，但必须证明所有读取都在同一 block 内、严格晚于写入、地址相同，且索引依赖未被修改或副作用破坏。独立 DebugLog 只观察参数，不作为内存写入屏障；参数中的副作用仍先使索引快照失效。其余部分写入、多次动态写入及跨 block 初始化保持保守。赋值右侧和动态地址中的旧值读取同样参与分析。

AdvancedDeadCodeElimination 会使旧活跃性缓存失效。即使旧图仍能装入 4096 槽，也必须重新分析已经缩短的生命周期，避免遗留的干涉关系阻止复用。

无法装入时错误会报告 callback、所需 slots 和 4096 上限，不自动降低优化等级。

## 三等级验证记录（2026-10-03）

以下按阶段保留原始问题和修复证据；前期记录中的未解决项不代表当前状态。2026-10-04 的最终矩阵见下方“大集合排序的运行成本对照”“同输入的项目级编译成本对照”和“实际 callback 的受控执行对照”；承载表已同步到单次动态写入分析后的结果。

基线为 `ba71e5c`，Windows/amd64、Go 1.25.13、i7-13650HX、默认 `GOMAXPROCS=20`。端到端测量每次新建 Compiler，不跨实例共享 callback cache；Go 工具链与文件系统缓存未清空。下表为三个单次样本的中位数，仅用于本机比较，不作为跨机器性能保证。`B/op` 是累计分配量，不是峰值常驻内存。

| 项目 | Minimal 时间 / 节点数 | Fast 时间 / 节点数 | Standard 时间 / 节点数 |
|---|---|---|---|
| reference，四模式 | 0.568 s / 64 | 0.578 s / 64 | 0.543 s / 38 |
| callvalues，Play/Watch | 0.620 s / 7,426 | 0.572 s / 7,426 | 0.667 s / 6,344 |
| Godori，四模式 | 1.955 s / 73,384 | 2.020 s / 73,781 | 5.245 s / 36,970 |

Godori 累计分配约为 1.642 / 1.639 / 3.003 GB。小项目时间容易受加载与采样噪声影响，不能把 reference 中 Standard 略快解释为其优化阶段更便宜。

使用相同已加载、已 lowering 的 Project 独立测量优化阶段（不经过 callback cache）：

| Godori 指标 | Minimal | Fast | Standard |
|---|---:|---:|---:|
| 优化阶段中位耗时 | 65.65 ms | 60.91 ms | 3,584.85 ms |
| 优化阶段累计分配 | 151.8 MB | 148.4 MB | 1,750.9 MB |
| 最大 callback 分配槽数 | 3,385 | 3,385 | 1,589 |

这里的最大槽数是最终 IR 中每个 callback 的物理 Temporary Memory layout 大小的最大值，不是运行中的最大实际索引。Standard 在该项目明显缩小了节点池和 temporary layout；Fast 的优化阶段略便宜，但端到端没有稳定胜过 Minimal，也没有在顺序分配本已足够时主动降低槽数。

### 承载边界与动态执行

`TestOptimizationTierCapacityAndRuntime` 通过 Builder 构造受验证 IR，再走完整 optimizer/backend，执行最终 EngineData。合成 IR 用于隔离 allocation 能力，不声称任意 Go 源码都生成相同 local 布局。

| 输入布局 | Minimal | Fast | Standard |
|---|---|---|---|
| 两个同时活跃的 2,048 槽数组 | 4,096 槽，执行通过 | 同左 | 同左 |
| 4,097 个依次写入、输出的 scalar | 稳定拒绝 | 1 槽，32,782 步 | 1 槽，8,200 步 |
| 三个生命周期不重叠的 2,048 槽数组，各写入并读取一个动态索引 | 拒绝 6,144 槽 | 2,048 槽，执行通过 | 2,048 槽，执行通过 |
| 三个 2,048 槽数组，活跃关系为链 | 拒绝 6,144 槽 | 4,096 槽，执行通过 | 4,096 槽，执行通过 |
| 两个同时活跃的 2,049 槽数组 | 稳定拒绝 | 稳定拒绝 | 稳定拒绝 |

成功分配的数组用例只读回先前写入的相同动态索引，并检查索引 0、1 及末端。原先三个顺序数组因多槽 local 没有定义边界而同时 live-in；单次写入证明已消除这项缺口。集中测试另外验证索引 local 改写、索引 semantic memory 改写、不同索引、跨块读取及先读后写，防止复用暴露其他数组的旧内容。三数组干涉链进一步通过区分日志与内存写入缩短生命周期；Fast 再通过超限时的 first-fit 重试找到空位。固定 Py 在同一用例仍拒绝，当前 Go Fast/Standard 均以 4,096 槽执行通过。日志参数内嵌 Set 的负例确保该放宽不会掩盖实际索引修改。

已有 DSL fixture 的最终节点执行显示，Standard 不总是降低运行成本：

| 场景 | Minimal | Fast | Standard | Standard 相对 Minimal |
|---|---:|---:|---:|---:|
| fuzzsemantics，动态循环 127 次 | 4,845 步 | 4,845 步 | 5,091 步 | +5.1% |
| 257 项逆序记录稳定排序，Play/local、checks none | 875,664 步 | 875,664 步 | 949,438 步 | +8.4% |

循环输出用独立求和公式验证；排序输出用 Go 稳定排序验证。三档结果一致，排序均调用比较器 1,825 次。节点步数来自内部模拟器，不能换算成官方客户端耗时或帧率；不同 RuntimeFunction 的真实成本不相同。

### Pass 成本与消融

仅通过临时 Go overlay 移除指定 pass，未修改正式 pipeline：

- 去掉 CSE 后，127 次循环降至 4,555 步，257 项排序降至 873,609 步；这些样本的输出仍正确。去掉小条件块合并后，循环为 4,567 步。说明提取表达式和控制流变换的收益需要结合最终 backend 成本评估，不能仅根据 IR 表达式大小判断。
- 循环单独去掉 LICM、aggressive inline 或 NormalizeBlocks 均仍为 5,091 步；去掉普通 InlineVars、ADCE、CopyCoalesce 分别增至 6,651 / 5,853 / 7,963 步。这是局部消融，不能将各项收益相加或推广为所有 callback 的结论。
- Godori Standard CPU profile 中，CopyCoalesce 累计采样为 21.41 s，整个 Optimizer 调用栈为 45.82 s；这些是并行 worker 的 CPU 采样累计值，不是墙钟耗时。主要热点位于合并后干涉图逐边重映射、map 查询与 bitset 写入。
- 去掉 CopyCoalesce 的 Godori 优化阶段中位数为 0.728 s，但最大槽数增至 1,897，节点池增至 58,334；端到端约 2.313 s。构建成功不等于 Godori 游戏行为等价。应优化该 pass 的图重映射实现，而不是仅凭编译变快就删除它。

结论：Minimal 的保守编译路径成立，Fast 的 scalar 超限复用价值成立，Standard 在实际项目上的产物压缩价值成立；尚不能声称三级构成运行性能单调增强的阶梯。优先评估 CSE/条件块合并的收益模型、CopyCoalesce 的图算法成本、多槽 aggregate 的活跃性精度。调试体验和官方客户端性能未在本轮证明。

### 与固定 Py 基线的原因对照

继续对照本地 `sonolus.py@1040bc0dcc116efdbca05f144edec302e839bcd3`，未更新参考版本或重生成 golden。以下为正式重构前通过临时 overlay 或临时 harness 完成的原因定位；正式修改结果单列在后文。

**CSE 退化不能单独归咎于成本阈值。** Py 的 `optimize/cse.py` 同样以表达式成本 ≥ 4 为提取门槛，依赖后续 InlineVars 清理单次使用的提取。两边都有这一启发式的局限，但并非完全相同实现：Py 保留原始/重写表达式的对应关系，InlineVars 处理别名链和免费内联；Go 的 CSE、InlineVars 及 CFG 清理采用不同实现。

将 `fuzzsemantics` 的同一份未优化 Go IR 转为 Py CFG，使用 Py pipeline/finalizer，再将节点池交给相同 Go simulator 执行，127 次循环得到：

| pipeline | Go 优化器与 backend | Py 优化器与 finalizer |
|---|---:|---:|
| Standard | 5,091 步 | 4,566 步 |
| Standard 去掉 CSE | 4,555 步 | 5,074 步 |
| Standard 去掉小条件块合并 | 4,567 步 | 4,050 步 |

所有版本在 -1、0、1、8、32、127 输入上满足独立输出公式。此比较固定输入 CFG 和执行器，但 finalizer 不同，不能把全部差值归因于某一个 pass，也不是两种语言前端的同源码基准。小条件块合并在两边都可能增加这个样本的成本，属于共同启发式的收益边界；CSE 的收益方向不同则需要检查 Go pipeline 的配合。

进一步查看最终 Go IR，循环回边上保留了无指令的跳转块。Go `CoalesceFlow` 排除所有处于 SCC 环中的候选，Py 只使用局部自环、entry 和 Phi 等约束，因此能消去某些循环内转发块。临时仅允许无 Phi 的空块转发到有指令的目标，Go 循环从 5,091 降到 4,833 步，排序从 949,438 降到 925,484 步，现有样本输出不变。这确认了过度保守的 CFG 清理是成本来源之一；排序仍比 Minimal 多步，收益模型问题并未全部消失。

因此应先区分真正的纯空跳转环与合法循环内转发，确保终止性、Phi 前驱和循环语义，再评估 CSE/内联的净收益。简单增加原始表达式 key 或允许免费 SSA copy 内联的临时试验，没有改变此循环的 5,091 步结果，不足以构成修复。

**数组超限是 Go 的分析精度缺口，不是 4,096 槽硬边界本身。** Py `optimize/liveness.py` 的 `preprocess_arrays`、`array_defs` 和 `is_array_init` 可以把数组生命周期截断在首次定义处；Go `localInterference` 只对单槽 local 的定义做 kill，多槽写入仍计为使用。同形状的三个顺序使用数组，Py AllocateFast/Allocate 均给出 offset 0，总占用 2,048 槽；最终节点在索引 0、1、2,047 上正确输出 10、20、30。Go 仍报告 6,144 槽。

可重构方向是明确的 aggregate 生命周期/完整初始化信息，或安全的槽区间活跃性。不能直接把任意首次 partial write 当作整个 Go 值被重新定义；必须覆盖部分写入、跨分支初始化、循环、pointer backing alias 和未覆盖槽的后续读取。Py 的这一策略是参考，不是足以替代 Go 安全证明的契约。

Py 并非总能找到理论可行的布局：同样的三数组干涉链，Py AllocateFast 和 Allocate 都拒绝；两个同时活跃的 2,049 槽数组也都拒绝。前者属于分配启发式能力边界，后者是实际硬上限。另一个独立边界差异是 Py AllocateBasic 使用 `>= 4096`，恰好 4,096 槽也拒绝，而其 AllocateFast/Allocate 和当前 Go 均允许恰好 4,096 槽。不能为了字面 parity 引入这一拒绝边界。

**CopyCoalesce 属于实现效率问题，可先保持算法行为重构。** Py 只为存活的单槽 TempBlock 建立干涉关系，筛选标量 copy 后以集合更新等价组；它不会像 Go 一样保留整份图给后续 Allocate。Go 为保留分析缓存而克隆、合并并逐边重映射整图，最后的 map 查询和对称边重复处理是额外成本。不能把图直接裁成标量子图后继续当完整 allocation 图复用。

临时试验预计算旧 local 到新 local 的稠密映射，并且只重映射无向图的一半边，仍由 `addInterference` 建立对称边。优化器全包测试和集中语义测试通过；Godori 最大槽数仍为 1,589，完整 artifacts 的 SHA-256 与原版均为 `e7791bdf6e810201e05440d6b687e34fa76d1c25af3bc34f2bb5a0a5e717916e`。三样本优化阶段中位数约 2.23 秒，原测量为 3.58 秒，随后原版三样本复测为 4.47 秒。存在本机测量波动，不承诺固定加速百分比。这证明有不删除 pass、不牺牲产物的优化空间，但临时版本未完成正式修改所需的全部验收。

建议按问题分别推进：先做保持输出的 CopyCoalesce 图重映射优化；再补安全的循环空块收缩及相关收益回归；随后设计 aggregate 生命周期分析；最后在消除这些配合差异后调整 CSE/小条件块的收益模型。以上均是可实施方向，不应通过关闭 Standard 阶段或默默降级绕过问题。

### 专项重构的阶段结果

首批实现保留三个等级及必需校验，调整以下算法细节：

- CopyCoalesce 使用稠密 ID 映射，只重映射一次无向边，并延迟到实际合并时才复制干涉图。
- CoalesceFlow 区分纯空跳转环与循环内部的空转发链，批量压缩后者；纯空环、entry 和 Phi 前驱边界继续保留。
- Standard 在复制合并前先清理死写入，再保留合并后的原有死代码清理。仅交换两阶段不足以消除新产生的死复制。

同一动态循环在 127 次迭代时，最终 EngineData 执行步数由 5,091 降为 4,047，固定 Py 为 4,566；集中测试保留独立输出公式，并以该 Py 步数作为此样本的回归预算。它不是所有 callback 的通用成本上界。

Godori 四模式优化阶段三样本中位数约 1.43 秒，分配量约 1.65 GB；一次端到端测量约 3.09 秒、29,016 个去重节点。最大 Temporary Memory 从 1,589 升到 1,897 槽：复制合并改变了干涉组及 first-fit 顺序，运行成本、节点池大小和槽数并非同步下降。上述测量来自同机候选实现，不是官方客户端性能。

固定 Py corpus 仍有未消除差距：动态分支/开关为 Go 24 对 Py 17 步，普通循环为 362 对 345 步，只读 CSE 循环为 132 对 120 步。三个顺序使用的动态数组仍受 Go 多槽活跃性精度限制。因而当前阶段不能宣称全面达到 Py 同等性能；后续还需聚合值生命周期、表达式/复制清理、backend 控制流成本，以及更大同输入 corpus 的编译和运行比较。

首批正式实现已通过全量串行测试、全量 race、build、vet 与 Godori 四模式 CLI vet。固定 Py golden 未变，只更新并核对了 Go 等价结构差异白名单。

第二批实现使 AdvancedDeadCodeElimination 不再保留旧活跃性缓存，并加入完整 aggregate 初始化识别。Godori 最大槽数降至 115，优化阶段三样本中位数约 1.45 秒。三档集中语义测试和固定 Py 语义 corpus 通过；完整初始化的三个顺序数组在 Fast/Standard 下均由 6,144 槽降到 2,048 槽。部分初始化、重复覆盖、读取旧值与动态写入另有最终 EngineData 语义回归。只写单一动态索引的合成数组仍保持保守，不将这部分缺口算作已解决。

随后修正自然循环识别的单块自环分支：`header == latch` 时循环体只有 header，不应沿 header 的外部前驱继续扩展。原实现把 preheader 算进循环，导致 LICM 放弃提升；固定 Py 的 `compute_loop_body` 已明确处理这一情况。修正后只读循环从 132 降为 116 步（Py 120），已通过 LICM 回归、固定 Py 最终语义和优化器全包测试。上面的 Godori 测量与下表的同输入计时均先于此项修正。

第三批进一步调整内联与出口生成：普通内联保留 LICM 已提升的循环外计算，按临时 Set/Get 成本允许便宜表达式的重复使用；先压缩不可变 SSA 别名链并更新 Phi 输入，再统计使用次数。可变 local 和 semantic memory 不参与别名归并，仅允许唯一使用紧邻无副作用 terminator 时移动读取。无返回值 callback 使用 JumpLoop 出口索引，避免额外 Break。集中测试覆盖循环 Phi、可变值快照、求值顺序和循环外计算。

固定 Py 小 corpus 的所有输入已满足 Go 最终节点执行步数不高于 Py。但更大的排序矩阵仍保留差距，不能用小 corpus 的结果推断普遍优越性。

全量语义验收另外暴露了 SSA 销毁的并行复制缺口：Phi 别名归并后，`VariantNote` 的 container helper 从预期 62 变成 63。逐 checkpoint 定位到首次 InlineVars 暴露的 Phi 输入依赖：循环边上的 `x = next; y = old x` 在 FromSSA 中被顺序写入，令 y 读取新 x。现在先保存所有同时作为复制源的目标旧值，再替换这些源完成赋值；循环交换、复制链以及原 container helper 的最终 EngineData 回归通过。这是正确性修复，不能当作允许的优化收益边界。

固定 Py 的 SSA 销毁虽显式处理交叉复制，直接构造合法 SSA 复制链并经其 FromSSA、AllocateBasic 和 finalizer 后，最终节点仍输出 `[7, 2, 1, 1]`，而并行赋值预期为 `[7, 2, 1, 7]`。这是该低层 pass 的反例，不证明同样结构能由所有 Py 前端程序触发。Go 修复后通过预期，不能为结构 parity 照搬这一 Py 行为。

Go 自身的 reference 输出快照仅更新四处无返回值出口（Break 改为 JumpLoop 出口索引）。固定 Py snapshot 与 JS peephole golden 均保持原基线。

上述第三批修改及 Phi 修复已通过全量串行测试、全量 race、build、vet 和 Godori 四模式 CLI vet；测试继续集中在既有文件中。通过这些验收不等于官方客户端性能验收，也不消除下面记录的运行成本反例。

### 大集合排序的运行成本对照

同一 Go frontend IR 分别经 Go 和 Py 优化、finalization，再由相同 simulator 执行最终节点。每档覆盖 Play/Watch、10 个长度（0 至 257）、5 种输入分布、两个方向及两种 storage，共 400 个输入；独立稳定排序预期、逐元素结果与比较次数上界均验证。

固定 Py 在 Standard 编译此 IR 时触发下节记录的 `smath.remainder` 缺失。为区分该基线错误和算法效果，另一次诊断进程仅将 `remainder` 指向同版本已有的 `_remainder`，没有修改 Py checkout 或参考 golden。下表的 Standard 因而属于补齐名称别名后的诊断对照，不是固定基线直接通过：

| 等级 | Go 总步数 | Py 总步数 | Go 更慢的输入数 |
|---|---:|---:|---:|
| Minimal | 46,588,936 | 52,194,952 | 0 / 400 |
| Fast | 46,588,936 | 52,194,952 | 0 / 400 |
| Standard | 35,041,264 | 49,678,044 | 0 / 400 |

SSA 别名归并前，Standard 总步数为 48,296,336，142 项慢于 Py；归并后为 46,418,904，仍有 62 项慢于 Py。继续对新插入表达式进行传递内联，并按展开后的成本重新检查复制预算后，降为 40,628,992 步，只剩持久 storage 的 20 个空输入各多 4 步。将常量 Phi 输入直接写到所选前驱边后，这些差距也消失。随后完成下文的死值需求传播、死 Phi 清理、循环块合并与分支事实简化，刷新得到表中结果。257 项逆序样本当前为 Go 683,003 对 Py 966,673 步，比较次数均为 1,825。

传递展开在单次 callback 内缓存，替换到每个使用处时仍独立复制表达式树。集中回归同时检查单次使用确实完全展开，以及便宜包装展开为昂贵表达式后不会被重复内联；循环外计算和可变读取限制继续生效。

总步数是上述固定矩阵的等权求和，不代表真实输入分布或客户端时间。该矩阵已逐项达到 Py 诊断对照成本，但不据此推断任意实际引擎或输入均有相同表现。

### 同输入的项目级编译成本对照

将 Godori 四模式 frontend 输出的 186 个 callback IR 经临时适配器转换为 Py CFG，保留 semantic memory 的模式 block 映射、动态索引、控制流、纯度和 ExportValue。两边逐 callback 串行优化，均不使用 callback cache；不计 loader、frontend、适配或 backend。Go 的计时包含 Optimizer 自身的输入深拷贝与每阶段校验。以下为同机三样本中位数：

| 等级 | 共同成功 callback 数 | Go | 固定 Py |
|---|---:|---:|---:|
| Minimal | 186 | 0.251 s | 2.291 s |
| Fast | 186 | 0.225 s | 2.528 s |
| Standard | 185 | 1.354 s | 51.109 s |

Go 计时已在全部 callback 覆盖、分支事实简化和动态数组干涉链改进后刷新，共同成功的 185 个 Standard callback 最大 Temporary Memory 为 38 槽；导出的共同输入 IR SHA-256 仍为 `931e9138575bf77338ab1cca3cf9f8129e46f882b9713a415c46842d69457178`。近期 Standard 测量为 1.251、1.499、1.354 秒，包含新增分析成本与采样波动，不据此归因单项改动的耗时收益。Py 复用此前同一输入的三次计时，不是同一时刻的交错测量。

Standard 单列了 PreviewStage.Render：固定 Py 的 `constant_evaluation.py` 调用不存在的 `smath.remainder`，而同版本 `math_impls.py` 只定义 `_remainder`。未修补 Py 或更新基线；Go 的 Standard 对照计时排除同一个 callback，Py 计时也只累加成功 callback。这是共同成功集合的编译成本比较，不是 186 个 callback 均通过的 Py 验收。

该测量支持当前 Go 在此规模同输入优化任务上的编译效率，但不证明最终产物游戏行为等价，也不替代运行节点成本与承载边界的独立验证。reference 的小样本低于本机 Go 计时分辨率，不据此宣称零成本或计算加速倍数。

复现常规测量：

```bash
go test ./internal/compiler -run '^TestOptimizationTierCapacityAndRuntime$' -v -count=1
go test ./internal/compiler -run '^TestLargeContainerSortPreservesStableOrder$' -v -count=1
go test ./internal/compiler -run '^$' -bench '^BenchmarkCompileAll$' -benchtime=1x -count=3 -benchmem
go test ./internal/compiler -run '^$' -bench '^BenchmarkCompilerStages/(reference|godori)/optimize' -benchtime=1x -count=3 -benchmem
```

### 实际 callback 的受控执行对照

进一步以 Godori 实际四模式 callback 的同一 IR 对照最终节点，使用 Development Level 的 42 个实体、默认选项、实际 ROM 和受控 BPM/时间输入。先用 Minimal preprocess 初始化，再独立执行各 callback。初次矩阵只覆盖 161 个 callback；补入 AccentTapNote，以及按源码 Spawn 参数构造的 HoldManager、ScheduledLaneEffect 后，全部 186 个 callback 均进入矩阵，没有跳过项。新增实体检查提前帧、目标时刻、结束后，及触摸、replay stream、已激活特效和 skip 状态，每档共 1,108 次执行。Py 仍使用前述仅在诊断进程中的 remainder 名称别名。

这轮验证首先发现了验证工具的两处规范偏差：模拟器曾只接受二元 Subtract，而 Runtime/JS 支持按顺序处理多参数；整数 switch 的非整数判别值应当走默认分支或返回零，不应作为非法索引报错。已修正 simulator、生成式参数元数据并补入既有集中测试。省略 Play shouldSpawn 的默认真值，以及空 stream 的内部表示差异，则在临时对照工具中修正，没有作为优化器语义错误报告。

实际产物也暴露了 ADCE 的算法缺口：普通活跃性分析把死赋值右侧的读取当作必要使用，使跨块死值链和死值环残留。改为工作队列传播实际需求，仅让必须保留的指令产生读取需求后，一次 pass 即可完成清理。集中测试同时覆盖不可观察的 32 块值环和通过 DebugLog 观察的同一循环，执行最终节点验证语义。Godori Standard 最大临时槽数由 55 降至 39。

| 等级 | Go 总步数 | Py 总步数 | Go 更慢的输入数 | 可观察差异 / 执行错误 |
|---|---:|---:|---:|---:|
| Minimal | 608,620 | 623,924 | 0 / 1,108 | 0 / 0 |
| Fast | 608,620 | 623,780 | 0 / 1,108 | 0 / 0 |
| Standard | 163,793 | 397,287 | 0 / 1,108 | 0 / 0 |

上述矩阵验证返回值、语义内存、顺序副作用及 stream 内容。仅修正 ADCE 时，Standard 仍有 26 项慢于 Py，Play FlickNote 提前帧从 275 步降至 253 步，而 Py 为 206 步。继续按 Py 的可观察根依赖遍历重构普通 DCE，消除互相维持活性的死 Phi 环后，该样本降至 193 步，Standard 的逐项差距消失。普通 DCE 不再依靠反复全图计数删除定义。

Minimal/Fast 的主要差距来自 CoalesceFlow 对整个循环 SCC 禁止合并的过度限制。唯一前驱线性后继在循环中也可合并；保留入口、自边与多前驱边界，并修复 Phi 前驱即可。Fast 的不可达代码清理还需像 Py 一样先折叠常量控制流。两项调整后，TapNote/AccentTapNote.UpdateSequential 曾有 11 个输入为 Go 176、Py 175 步：Go 将连续 switch 化为带起始值减法的整数索引跳转，Py 的顺序比较恰好在第一项命中。后端现对起点较小的正向连续 case 补入默认分支，直接使用原始判别值索引，省掉 Subtract 及其常量读取。该 Tap 样本降为 174 步，表中三档均已没有较慢输入。

新增覆盖实际发现了进一步的算法缺口。ScheduledLaneEffect 重复测试短路条件；CoalesceFlow 现在沿空块复用已求值条件，并在 SSA 消除后再进行受成本约束的内联与死代码清理。Watch HoldManager 的两个非 replay 提前帧原为 Go 79、Py 77 步；唯一分支边上的逻辑事实现在可简化后继第一条纯赋值中的 Not，消除了该矩阵的剩余 Standard 差距。测试保留同目标双边、多前驱、Phi、写入和副作用边界，不能把一般非零数或带符号零当作规范化布尔数替换。

Py 将 StreamGetPreviousKey 等查询标记为 pure，Go 保留可写 stream 的读取快照。上述改进没有把查询直接改为 pure；跨 StreamSet 的读取移动仍需要单独的读写依赖证明。此保守规则不构成该矩阵的剩余成本反例，也不表示任意 stream 程序都已最优。

控制流折叠另有独立的语义修复：同目标 branch 与空 case switch 原来会丢弃判别式中的副作用；现在用 void Execute 保留求值。集中测试在三个等级均执行最终节点并验证日志只出现一次。

此处是受控环境的离线 callback 比较，没有真实触摸、完整生命周期调度或客户端渲染；它不证明完整 Godori 行为或官方客户端帧率。最终实现已通过全量串行测试、全量 race、build、vet、Godori 四模式 CLI vet，以及刷新后的三档排序矩阵；没有新增测试文件或更新固定 Py/JS 参考基线。

## Backend SNode peephole

IR optimizer 之后，backend 始终执行以 `sonolus.js-compiler@37b0eee` 为基线的 SNode peephole，包括算术单位元、常量组合、Get/Set shifted 互换、SetAdd 等融合和控制流尾值清理。

被代数消去但可能有副作用的动态参数会用 `Execute` 保留求值，确保严格的左到右副作用顺序。

与固定 JS 基线的差异：步长为 1、起点为正且不超过 8 和 case 数量的 switch，可在前面补默认分支引用，省去索引归一化减法。补位不提前执行默认分支，也不重复求值判别式；大偏移或非单位步长继续使用原规则。集中测试执行最终节点，覆盖非整数、NaN/Inf、命中与未命中、默认分支与判别式的副作用，并约束表大小。固定 JS golden 保持不变。

## 差分与回归

### 连续分支改写的 Phi 边身份修复（2026-10-04）

`f9e0f44` 的 Standard 第 20 步 `RewriteToSwitch` 将连续相等判断合并为 switch 时，只改了跳转目标，没有保留被绕过比较块送入后继 Phi 的值。随后的不可达块删除会连同这些 Phi 输入一起删除。实际两层九宫格循环因此从 12 个非退化矩形变成 11 个，并出现坐标错位；前 19 个步骤输出正确，故不是数组容量或绘制 API 的限制。

修复记录每个 case/default 原来的前驱。只有被改接且目标含 Phi 的边才创建转接块，并复制该边原有的 Phi 输入；同一源/目标边复用转接块。这样保留不同 case 汇入同一目标时各自的值，不关闭整个 switch 优化。`gaugeprecision` fixture 在三个等级及每个 Standard checkpoint 执行最终 EngineData，固定检查 12 片及其顺序；使用方另外复用了完整 HUD 七场景对照。

补偿乘积是另一项精度边界，不能与上述 CFG 缺陷合并：现有常量求值器和内部 simulator 使用 binary64，而使用方的附加模型对每个节点强制 binary32 舍入。`4097 * 1000000` 在两种模型中分别为 4097000000 与 4096999936；继续执行 `c-(c-b)` 后，b 的高/低分量分别为 `1000000/0` 与 `999936/64`。现有 `Product` 回归验证 binary64 下三级优化和逐 checkpoint 输出一致；它不证明客户端逐节点 binary32 模型，也不证明这种模型下的误差已修复。未增加精度模式、未改算术折叠或数值版本契约。

官方[数学函数优化规则](https://wiki.sonolus.com/engine-specs/function-optimizations/mathematical-functions)明确支持常量折叠，但该页没有约定其浮点精度；[ROM规范](https://wiki.sonolus.com/engine-specs/resources/engine-rom)规定的是存储格式。不能由后者推导全部算术节点必须逐步舍入到binary32，也不能把禁用所有常量折叠视作已获得依据的修复。

该修复已通过隔离 Ubuntu worker 上的全量 build、串行 test、串行 race、vet 和 Godori 四模式 CLI vet；受测修改文件与本地源码哈希一致。未修改依赖或版本字段，未替换使用方已安装的编译器，也未做客户端验收。

仓库保存固定 Py pass snapshot 和 JS SNode golden。普通 Go CI 只读取 checked-in golden，不依赖相邻 Python/Node checkout。更新固定参考版本时，必须显式运行对应 testdata regeneration script并审核差异。

具体 fixture/snapshot schema、allowlist 规则和 regeneration 命令见[维护指南](maintenance.md)。

CG-02 的 `[24]Vec2` 值参数、值返回和20项组合回归在旧 Standard 的第11个 pass（首次 `InlineVars`）首次从324变为0：数组动态写入被漏算，复制时的固定槽读取被替换为初始化零值。23项组合复用了同类型 local，使源数组固定槽定义次数从1变为2，恰好避开错误替换；这不是24元素容量限制，也不是该组合正确性的保证。集中 `callvalues` fixture 保留隔离与组合调用顺序，回归执行每个 checkpoint 经 backend 装配的 EngineData，并验证三个优化等级下的明确 Go 预期与逐元素顺序，不把 Minimal 输出当作正确性标准。

上述回归针对当前源码。源码验收通过不代表既有 v2.3.14 Release 已更新，也不替代使用方采用新版本后的完整引擎与设备验收。
