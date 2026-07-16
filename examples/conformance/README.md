# DSL 一致性示例

这个四模式示例用于持续验证公开 Go DSL，而不是演示具体玩法。除静态资源、
archetype 和全局 callback 外，它还覆盖泛型 variadic helper、闭包捕获、定长数组与
整数 `range`、`switch`、semantic memory 和副作用调用。

```bash
go run ./cmd/sonolus-go build -name conformance -m all ./examples/conformance
```

该示例会在 Minimal、Fast 和 Standard 三个优化等级下由测试完整编译。
