# 最小引擎示例

这是一个可以完整编译四种模式的最小输入，覆盖共享 Configuration 与 ROM、
模式 build tag、名称型资源、archetype 和全局 callback。

在仓库根目录执行：

```bash
go run ./cmd/sonolus-go build -o minimal -m all ./examples/minimal
```

编译产物默认写入 `dist/minimal`。
