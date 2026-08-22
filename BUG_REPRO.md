# 聚合地址前缀诊断

## Bug 是什么

当部署地址已经携带路径前缀且末尾带有分隔符时，聚合下游地址会出现重复分隔符。下游目标与实际路由不再一致，导致聚合调用无法正确到达目标。

## 如何触发

为聚合下游配置接口标识，并把部署地址设置为包含路径前缀且以分隔符结尾的地址。执行聚合调用时，系统会在该地址后继续拼接内部目标路径。

## 根因

文件：internal/aggregateservice/aggregate_config.go；符号：resolveURL；失效机制：跨层数据流在配置读取、地址解析和下游调用之间没有统一的路径边界语义。地址末尾的分隔符未经归一化直接参与拼接，内部目标再次带入分隔符后形成重复路径边界，最终将错误地址传给下游请求。

## 运行指令

```bash
go test ./internal/aggregateservice -v -run '^TestAggregatePrefixTargetKeepsSinglePathSeparator$' -count=1
```

## 错误信息

聚合目标地址包含重复分隔符，与期望的部署前缀路径不一致。

## 错误堆栈

```text
--- FAIL: TestAggregatePrefixTargetKeepsSinglePathSeparator (0.00s)
    prefix_target_test.go:9: aggregate prefix target="https://mock.example/gateway/mock//internal/api/orders", want "https://mock.example/gateway/mock/internal/api/orders"
FAIL
```
