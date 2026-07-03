# 优化器

sonolus-go 实现了一个多级优化流水线，对标 sonolus.py 的优化器。

sonolus-go 提供三级优化，通过 `pipeline.go` 中的 `Minimal()`/`Fast()`/`Standard()` 函数定义。
完整的分歧说明和 pass-by-pass 对比见 [PIPELINE.md](../compiler/ir/optimize/PIPELINE.md)。

### Standard（完整优化，44 passes）

完整的 SSA 往返 + 双轮 SCCP + 三轮内联 + 循环优化 + 寄存器分配：

```
Pre-SSA: CoalesceFlow → CoalesceSmallConditionalBlocks → UCE → DCE
SSA:     ToSSA → SCCP → UCE → DCE → CoalesceFlow →
         NormalizeBlocks → InlineVars×2(+DCE) → SCCP →
         FlattenAssociativeOps → RemoveRedundantArguments → DCE →
         CoalesceFlow → RewriteToSwitch → InlineVars(aggressive) →
         UnflattenAssociativeOps → LICM → CSE → NormalizeBlocks →
         FlattenAssociativeOps → InlineVars → DCE →
         FlattenAssociativeOps → RemoveRedundantArguments
Exit:    FromSSA → UnflattenAssociativeOps
Post:    CopyCoalesce → UCE → CoalesceFlow → CombineExitBlocks →
         NormalizeSwitch → AdvancedDCE → DCE →
         CoalesceFlow → UCE → DCE → RenumberVars
Alloc:   AllocateLive
```

- 完整 SSA 往返（ToSSA → FromSSA）
- 两轮 SCCP（SSA 内联前后各一次）
- 三轮内联（保守 → 保守 → 激进）+ RewriteToSwitch
- 循环不变量外提（LICM）+ 全局公共子表达式消除（CSE）
- 活性感知线性扫描寄存器分配（AllocateLive）

### Fast（均衡优化，26 passes）

单轮 SSA + 一轮内联 + 顺序分配（spill 时回退到活性分配）：

```
Pre:  CoalesceFlow → UCE → DCE → CoalesceFlow
SSA:  ToSSA → SCCP → UCE → DCE → CoalesceFlow →
      NormalizeBlocks → InlineVars → DCE → CoalesceFlow
Exit: FlattenAssociativeOps → FromSSA → UnflattenAssociativeOps
Post: CoalesceFlow → UCE → DCE → NormalizeSwitch →
      CombineExitBlocks → CoalesceFlow → UCE → DCE → RenumberVars
Alloc: TryAllocateBasic
```

- 单轮 SSA + SCCP
- 一轮内联
- TryAllocateBasic: 先尝试顺序分配，超过 256 槽时回退到 AllocateLive

### Minimal（最快编译，6 passes）

无 SSA 构造，仅基本清理 + 顺序分配：

```
CoalesceFlow → UCE → DCE → CoalesceFlow → RenumberVars
Alloc: AllocateBasic
```

- 仅基本块合并 + 死代码消除
- 无 SSA、SCCP、内联、LICM、CSE
- AllocateBasic 顺序分配（无活性分析）

## 优化 Pass 列表

| Pass | 文件 | 说明 |
|------|------|------|
| **ToSSA** | `ssa.go` | SSA 形式构造（phi 插入 + 变量重命名） |
| **FromSSA** | `ssa.go` | SSA 销毁（phi 具体化到边复制） |
| **SCCP** | `sccp.go` | 稀疏条件常量传播（三值晶格：Undef/Const/NAC） |
| **InlineVars** | `inlining.go` | SSA 变量内联（积极/保守双模式） |
| **LICM** | `licm.go` | 循环不变量外提（自然循环 + 回边检测） |
| **CSE** | `cse.go` | 全局公共子表达式消除（支配树遍历） |
| **CopyCoalesce** | `copycoalesce.go` | 副本合并（活性增强的 union-find） |
| **DCE** | `dce.go` | 标记-清除死代码消除 |
| **UCE** | `uce.go` | 不可达代码消除 |
| **AdvancedDCE** | `advanced.go` | 高级死代码消除（基于活性分析） |
| **CoalesceFlow** | `coalesce.go` | 控制流合并（空块跳过 + 线性链合并） |
| **CoalesceSmallConditionalBlocks** | `coalesce_small_cond.go` | 小条件块折叠 |
| **CombineExitBlocks** | `combine_exit.go` | 出口块合并 |
| **RewriteToSwitch** | `rewrite.go` | if-else 链 → switch 重写 |
| **NormalizeSwitch** | `normalize_switch.go` | Switch case 算术序列归一化 |
| **FlattenAssociativeOps** | `flatten.go` | 结合运算展平（Add/Mul/Mod/Rem） |
| **UnflattenAssociativeOps** | `unflatten.go` | 结合运算分解（n-ary → binary） |
| **RemoveRedundantArguments** | `redundant_args.go` | 代数简化（0/1 单位元移除 + Sub(0,a)→Negate） |
| **NormalizeBlocks** | `normalize_blocks.go` | BlockPlace.Index nil→Const(0) 归一化 |
| **RenumberVars** | `renumber_vars.go` | 临时变量确定性重编号 |
| **AllocateBasic** | `alloc_basic.go` | 顺序寄存器分配（无活性分析） |
| **TryAllocateBasic** | `alloc_basic.go` | 分层分配（顺序优先，spill 时回退） |
| **AllocateLive** | `advanced.go` | 线性扫描寄存器分配（基于活性区间打包） |

## SNode Peephole 优化

最终 SNode 树层面的运行时特定优化：

| 优化 | 说明 |
|------|------|
| Arithmetic flattening/folding | `Add`/`Subtract`/`Multiply`/`Divide` 合并常量 + 单位元消除 |
| Mod/Rem/Power flattening | 结合性展平 |
| Get/GetShifted pattern | `Get(index + y*stride)` → `GetShifted(...)` |
| Set/SetShifted pattern | `Set(x, Read(x) + 1)` → `SetAdd(x, 1)` |
| If(x, y, 0) → And(x, y) | 条件化简 |
| SwitchWithDefault normalize | 算术序列检测 + default 消除 |
| While body tail removal | 空循环体消除 |

## 参考

- sonolus.py 优化器: `../sonolus.py/sonolus/backend/optimize/`
- sonolus.js-compiler SNode 优化: `../sonolus.js-compiler/src/snode/optimize/`
