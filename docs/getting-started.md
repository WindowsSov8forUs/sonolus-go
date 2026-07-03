# 快速入门

## 前置条件

- Go 1.25+
- 三个兄弟仓库必须相邻 checkout：

```
VSCode/
  sonolus-go/              # 本仓库
  sonolus-core-go/         # EngineData 数据结构定义
  sonolus-pack-go/         # sonolus-pack 打包工具
  sonolus-server-go/       # Sonolus HTTP 服务器接口
```

```bash
git clone https://github.com/WindowsSov8forUs/sonolus-go.git
git clone https://github.com/WindowsSov8forUs/sonolus-core-go.git
git clone https://github.com/WindowsSov8forUs/sonolus-pack-go.git
git clone https://github.com/WindowsSov8forUs/sonolus-server-go.git
```

## 构建

```bash
make build       # 编译所有包
make test        # 运行全部测试
make vet         # 运行 go vet
make fmt         # 格式化所有源文件
make fmt-check   # 检查格式 (CI)
make clean       # 清除构建产物
```

## 编写引擎

创建一个 `engine.go` 文件：

```go
package main

type Skin struct {
    Note float64 // skin sprite name
}

type Note struct {
    Beat float64 `sonolus:"imported"`
    T    float64 `sonolus:"memory"`
}

func (n *Note) Initialize() {
    n.T = n.Beat * 0.5
}

func (n *Note) UpdateParallel(dt float64) {
    v := vec2(sin(n.T), cos(n.T))
    draw(1, v.x, v.y, 1, 1, 0, 1, 0, 0)
}
```

## 编译

```bash
# 编译 Play 模式
sonolus-go build -m play ./engine.go

# 启动本地开发服务器
sonolus-go serve -m play ./engine.go
```

## DSL 语法

参见 [DSL 语言参考](FRONTEND_SYNTAX.md)。
