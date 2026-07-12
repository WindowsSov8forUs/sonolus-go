# CLAUDE.md

## 构建

```bash
go build ./...           # 编译
go test ./...            # 全部测试
go vet ./...             # 静态分析
gofmt -w internal cmd sonolus    # 格式化
gofmt -l internal cmd sonolus    # 格式检查 (CI)
```

## 架构

```
Go源文件 → frontend → ir/optimize → ir/finalize → snode → mode/{play,watch,preview,tutorial} → build
```

详细架构见 `docs/architecture.md`。

## 代码约定

- **Pass 接口**: 优化 Pass 实现 `Pass` 接口，分析型 Pass 额外实现 `ManagedPass`
- **错误处理**: `fmt.Errorf` + `%w` 包装传播
- **测试**: Golden file + Fuzz 覆盖
- **并发**: `IDGen` 实例隔离 + `CompileCache` sync.RWMutex
- **命名**: camelCase / PascalCase，与 sonolus.py 对齐

## 参考

| 项目 | 路径 |
|------|------|
| sonolus.py | `../sonolus.py` |
| sonolus.js-compiler | `../sonolus.js-compiler` |
| sonolus-core-go | `../sonolus-core-go` |
| sonolus-pack-go | `../sonolus-pack-go` |
| sonolus-server-go | `../sonolus-server-go` |

## 测试常用

```bash
go test ./...                                          # 全量
go test ./internal/compiler/optimize/ -v -count=1   # 优化器
go test ./internal/compiler/ -run TestReferenceEngineDataGolden -update-reference  # 更新 golden
go test -fuzz=FuzzStaticValueEval -fuzztime=30s ./internal/compiler/source/  # Fuzz
```
