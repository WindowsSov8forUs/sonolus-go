# 优化器

sonolus-go 实现了一个多级优化流水线，对标 sonolus.py 的优化器。

## 流水线级别

### Standard（完整优化）

```
ToSSA → SCCP → ToSSA → SCCP → InlineVars → InlineVars
→ LICM → CSE → FromSSA → CopyCoalesce → DCE
→ NormalizeSwitch → AllocateLive → RemoveRedundantArguments
→ NormalizeBlocks → RenumberVars
```

- 两次 SSA 往返，每次后跟 SCCP
- 两轮内联（积极 → 保守）
- 循环不变量外提（LICM）
- 全局公共子表达式消除（CSE）
- 线性扫描寄存器分配

### Fast

```
ToSSA → SCCP → FromSSA → CopyCoalesce → DCE → AllocateLive
```

- 单轮 SSA + SCCP
- 适合开发阶段，编译速度优先

### Minimal

```
CoalesceFlow → UCE → DCE → AllocateTempBlocks
```

- 仅基本块合并 + 死代码消除
- 无 SSA 构造，最快编译

## 优化 Pass 列表

| Pass | 文件 | 说明 |
|------|------|------|
| **ToSSA** | `ssa.go` | SSA 形式构造（phi 插入 + 变量重命名） |
| **FromSSA** | `ssa.go` | SSA 销毁（phi 具体化到边复制） |
| **SCCP** | `sccp.go` | 稀疏条件常量传播（三值晶格：Undef/Const/NAC） |
| **InlineVars** | `inlining.go` | SSA 变量内联（积极/保守双模式） |
| **LICM** | `licm.go` | 循环不变量外提（自然循环 + 回边检测） |
| **CSE** | `cse.go` | 全局公共子表达式消除（支配树遍历） |
| **CopyCoalesce** | `copycoalesce.go` | 副本合并（union-find） |
| **DCE** | `dce.go` | 标记-清除死代码消除 |
| **UCE** | `uce.go` | 不可达代码消除 |
| **CoalesceFlow** | `coalesce.go` | 块折叠（无 phi 单后继块合并） |
| **AllocateLive** | `advanced.go` | 线性扫描寄存器分配（基于活性分析） |
| **NormalizeSwitch** | `rewrite.go` | Switch 算术序列检测与归一化 |
| **RemoveRedundantArguments** | `simplify.go` | 代数简化（0 加数移除、1 乘数移除等） |
| **NormalizeBlocks** | `normalize.go` | 整数块引用归一化 |
| **RenumberVars** | `rewrite.go` | 临时变量重编号 |

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
