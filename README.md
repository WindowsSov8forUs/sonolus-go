# sonolus-go

全 Go 语言实现的 Sonolus 引擎编译工具链。

将 Go 源代码子集编译为 Sonolus 可加载的 EngineData（EngineConfiguration +
EnginePlayData / EngineWatchData / EnginePreviewData / EngineTutorialData），
支持四种模式完整打包与本地开发服务器。

## 构建

### 前置条件

- Go 1.25+
- 三个兄弟仓库必须相邻 checkout（与 `sonolus-go` 在同一父目录）：

```
VSCode/
  sonolus-go/              # 本仓库
  sonolus-core-go/         # EngineData 数据结构定义
  sonolus-pack-go/         # sonolus-pack 打包工具
  sonolus-server-go/       # Sonolus HTTP 服务器接口
```

```bash
# 一键 clone 全部依赖
git clone https://github.com/WindowsSov8forUs/sonolus-go.git
git clone https://github.com/WindowsSov8forUs/sonolus-core-go.git
git clone https://github.com/WindowsSov8forUs/sonolus-pack-go.git
git clone https://github.com/WindowsSov8forUs/sonolus-server-go.git
```

模块通过 `go.mod` 中的 `replace` 指令指向本地路径（`../sonolus-core-go` 等），
对外部使用者需先发布 tagged 版本后删除 `replace` 块。

### 命令

```bash
make build       # 编译所有包
make test        # 运行全部测试
make vet         # 运行 go vet
make fmt         # 格式化所有源文件
make fmt-check   # 检查格式 (CI)
make clean       # 清除构建产物
```

## 架构

```
Go源文件 (.go)
  │
  ▼ compiler/frontend     ← go/parser + go/types 类型检查 + AST 追踪 → IR
  │
  ▼ compiler/ir/optimize  ← CFG/SSA 优化器 (~40 个 pass: SCCP/LICM/CSE/...)
  │
  ▼ compiler/ir/finalize  ← 寄存器分配 + 指令扁平化
  │
  ▼ compiler/snode        ← 运行时节点树 (SNode) + 去重 + 序列化
  │
  ▼ compiler/{play,watch,preview,tutorial} + compiler/modecompile
  │                       ← 四模式装配 + 回调 body 合成
  │
  ▼ compiler/build        ← EngineData 包装 + ROM
  │
  ▼ compiler/pack         ← sonolus-pack 源树输出
  │
  ▼ cmd/sonolus-go        ← CLI: build / serve / host / pack / level
```

## 命令行

```bash
# 编译单一模式
sonolus-go build -m play ./engine/

# 编译全部四种模式
sonolus-go build -m all ./engine/

# 本地开发服务器 (带热编译)
sonolus-go serve -m play ./engine/

# 生产模式打包 + 服务
sonolus-go pack serve ./engine/ -author "YourName"
```

## 参照项目

- [sonolus.js](https://github.com/NonSpicyBurrito/sonolus.js) — 引擎 DSL 标准库（TypeScript 运行时）
- [sonolus.js-compiler](https://github.com/NonSpicyBurrito/sonolus.js-compiler) — JS → 引擎节点编译器（结构参照）
- [sonolus.py](https://github.com/NonSpicyBurrito/sonolus.py) — Python 实现 / 性能目标（45k LOC，最完整）

## 许可

MIT
