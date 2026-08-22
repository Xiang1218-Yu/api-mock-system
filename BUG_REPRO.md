# Bug 复现说明

## Bug 是什么

请求已经结束后，异步调用记录继承了已取消的上下文，存储层拒绝保存，响应虽然已完成，统计却找不到该次调用。

## 如何触发

先取消请求上下文，再触发聚合调用记录的异步保存。记录保存应拥有独立的观测生命周期，而不是立即因原请求已经结束而失败。

## 运行指令

```bash
go test ./internal/aggregatehandler -v -run '^TestCancelledRequestStillPersistsAggregateCallLog$' -count=1
```

## 错误信息

含缺陷版本中，异步保存会收到已取消上下文。

## 错误堆栈

```text
--- FAIL: TestCancelledRequestStillPersistsAggregateCallLog
    v5_contract_test.go:43: cancelled request did not retain call log: context canceled
FAIL
FAIL    api-mock-system/internal/aggregatehandler
```

## 根因

文件：internal/aggregatehandler/aggregatehandler.go；文件：internal/calllogrepo/calllogrepo.go；文件：internal/dashboardservice/dashboardservice.go；文件：internal/mockhandler/mockhandler.go。`Handler.recordCall` 以原请求上下文派生异步保存的超时上下文。请求取消后，这个派生上下文立即结束，仓储保存入口会把它视为取消写入并返回。仪表盘再从持久化记录聚合统计，因此已经返回给客户端的调用从后续统计中消失。

修正边界应为观测记录建立独立生命周期，同时保留仓储对真正取消请求的保护语义，并让统计链路只读取成功持久化的调用。这里需要保持请求取消、仓储错误传播和统计读取之间的一致性。该诊断未修改生产代码。
